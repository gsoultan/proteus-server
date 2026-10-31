// Package jobs runs asynchronous conversions. Jobs live in memory on the
// instance that accepted them, with a bounded queue and a result TTL; a
// Postgres-backed queue can replace Store later without changing handlers.
package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// Status of a job.
type Status string

const (
	Queued    Status = "queued"
	Running   Status = "running"
	Succeeded Status = "succeeded"
	Failed    Status = "failed"
)

var (
	// ErrQueueFull is returned by Submit when the queue is at capacity.
	ErrQueueFull = errors.New("job queue is full, retry later")
	// ErrNotFound is returned for unknown, expired, or someone else's jobs.
	ErrNotFound = errors.New("job not found")
	// ErrClosed is returned by Submit after Shutdown started.
	ErrClosed = errors.New("server is shutting down")
)

// Job is a snapshot of an asynchronous conversion.
type Job struct {
	ID         string    `json:"id"`
	Status     Status    `json:"status"`
	Format     string    `json:"format"`
	CreatedAt  time.Time `json:"created_at"`
	StartedAt  time.Time `json:"started_at,omitzero"`
	FinishedAt time.Time `json:"finished_at,omitzero"`
	ExpiresAt  time.Time `json:"expires_at,omitzero"`
	Error      string    `json:"error,omitempty"`
	Bytes      int       `json:"bytes,omitempty"`
	Warnings   []string  `json:"warnings,omitempty"`

	owner   string
	req     *convert.Request
	pdf     []byte
	webhook string
}

// Settings configure a Manager.
type Settings struct {
	Workers       int           // concurrent async conversions; default 2
	QueueSize     int           // queued jobs waiting for a worker; default 100
	ResultTTL     time.Duration // how long results are kept; default 1h
	MaxJobs       int           // jobs retained in memory (any state); default 1000
	WebhookSecret []byte        // HMAC-SHA256 key for webhook signatures; empty disables webhooks
	Webhook       *safefetch.Client
	Logger        *slog.Logger
	Observer      Observer
}

// Observer receives queue telemetry.
type Observer interface {
	QueueDepth(n int)
	WebhookDone(outcome string)
}

type nopObserver struct{}

func (nopObserver) QueueDepth(int)     {}
func (nopObserver) WebhookDone(string) {}

// Manager owns the queue, the workers and the job table.
type Manager struct {
	s   Settings
	svc *convert.Service

	mu     sync.Mutex
	jobs   map[string]*Job
	queue  chan *Job
	closed bool

	workers sync.WaitGroup // job workers
	wg      sync.WaitGroup // background loops (sweeper)
	cancel  context.CancelFunc
	ctx     context.Context
}

// NewManager starts the workers and the expiry sweeper.
func NewManager(svc *convert.Service, s Settings) *Manager {
	if s.Workers <= 0 {
		s.Workers = 2
	}
	if s.QueueSize <= 0 {
		s.QueueSize = 100
	}
	if s.ResultTTL <= 0 {
		s.ResultTTL = time.Hour
	}
	if s.MaxJobs <= 0 {
		s.MaxJobs = 1000
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Observer == nil {
		s.Observer = nopObserver{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		s: s, svc: svc, jobs: map[string]*Job{}, queue: make(chan *Job, s.QueueSize),
		ctx: ctx, cancel: cancel,
	}
	for range s.Workers {
		m.workers.Go(m.worker)
	}
	m.wg.Go(m.sweeper)
	return m
}

// WebhooksEnabled reports whether jobs may ask for a callback.
func (m *Manager) WebhooksEnabled() bool { return len(m.s.WebhookSecret) > 0 && m.s.Webhook != nil }

// Submit queues req for owner (the API key id) and returns the new job.
func (m *Manager) Submit(owner string, req *convert.Request) (Job, error) {
	if req.WebhookURL != "" {
		if !m.WebhooksEnabled() {
			return Job{}, &convert.BadRequestError{Msg: "webhooks are not enabled on this server"}
		}
		if err := checkWebhookURL(m.s.Webhook, req.WebhookURL); err != nil {
			return Job{}, &convert.BadRequestError{Msg: "webhook_url: " + err.Error()}
		}
	}
	j := &Job{
		ID: newID(), Status: Queued, Format: string(req.Format), CreatedAt: time.Now().UTC(),
		owner: owner, req: req, webhook: req.WebhookURL,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Job{}, ErrClosed
	}
	if len(m.jobs) >= m.s.MaxJobs {
		m.evictLocked()
		if len(m.jobs) >= m.s.MaxJobs {
			return Job{}, ErrQueueFull
		}
	}
	select {
	case m.queue <- j:
	default:
		return Job{}, ErrQueueFull
	}
	m.jobs[j.ID] = j
	m.s.Observer.QueueDepth(len(m.queue))
	return j.snapshot(), nil
}

// Get returns the job if it exists and belongs to owner.
func (m *Manager) Get(owner, id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.owner != owner {
		return Job{}, ErrNotFound
	}
	return j.snapshot(), nil
}

// Result returns the PDF of a succeeded job.
func (m *Manager) Result(owner, id string) ([]byte, Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.owner != owner {
		return nil, Job{}, ErrNotFound
	}
	return j.pdf, j.snapshot(), nil
}

// Delete removes a finished job and its result.
func (m *Manager) Delete(owner, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok || j.owner != owner {
		return ErrNotFound
	}
	if j.Status == Queued || j.Status == Running {
		return &convert.BadRequestError{Msg: "job is still " + string(j.Status)}
	}
	delete(m.jobs, id)
	return nil
}

// Shutdown stops accepting jobs and lets queued and running jobs finish
// until ctx is done. After that, running conversions are canceled and the
// remaining queued jobs fail with ErrClosed.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	if !m.closed {
		m.closed = true
		close(m.queue)
	}
	m.mu.Unlock()
	done := make(chan struct{})
	go func() { m.workers.Wait(); close(done) }()
	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
		m.cancel()
		<-done
	}
	m.cancel()
	m.wg.Wait()
	return err
}

func (m *Manager) worker() {
	for j := range m.queue {
		m.s.Observer.QueueDepth(len(m.queue))
		if m.ctx.Err() != nil {
			m.finish(j, nil, ErrClosed)
			continue
		}
		release, err := m.svc.Acquire(m.ctx)
		if err != nil {
			m.finish(j, nil, ErrClosed)
			continue
		}
		m.mu.Lock()
		j.Status, j.StartedAt = Running, time.Now().UTC()
		req := j.req
		m.mu.Unlock()
		res, err := m.svc.Run(m.ctx, req)
		release()
		m.finish(j, res, err)
	}
}

func (m *Manager) finish(j *Job, res *convert.Result, err error) {
	m.mu.Lock()
	now := time.Now().UTC()
	j.FinishedAt, j.ExpiresAt, j.req = now, now.Add(m.s.ResultTTL), nil
	if err != nil {
		j.Status, j.Error = Failed, publicError(err)
	} else {
		j.Status, j.pdf, j.Bytes, j.Warnings = Succeeded, res.PDF, len(res.PDF), res.Warnings
	}
	snap, hook := j.snapshot(), j.webhook
	m.mu.Unlock()
	if err != nil {
		m.s.Logger.Warn("job failed", "job_id", j.ID, "error", err)
	} else {
		m.s.Logger.Info("job succeeded", "job_id", j.ID, "bytes", snap.Bytes)
	}
	if hook != "" && m.ctx.Err() == nil {
		m.deliver(hook, snap)
	}
}

func publicError(err error) string {
	var bre *convert.BadRequestError
	switch {
	case errors.As(err, &bre):
		return bre.Msg
	case errors.Is(err, convert.ErrTimeout), errors.Is(err, ErrClosed):
		return err.Error()
	}
	return "conversion failed"
}

// deliver POSTs the job snapshot to the webhook, signed with HMAC-SHA256 over
// "<timestamp>.<body>", retrying a few times with backoff.
func (m *Manager) deliver(target string, snap Job) {
	body, _ := json.Marshal(snap)
	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			cancel()
			m.s.Observer.WebhookDone("error")
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Proteus-Timestamp", ts)
		req.Header.Set("X-Proteus-Signature", "sha256="+Sign(m.s.WebhookSecret, ts, body))
		resp, err := m.s.Webhook.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		cancel()
		if err == nil && resp.StatusCode < 300 {
			m.s.Observer.WebhookDone("ok")
			return
		}
		if errors.Is(err, safefetch.ErrBlocked) || (err == nil && resp.StatusCode < 500 && resp.StatusCode != 429) {
			break // not worth retrying
		}
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 4
	}
	m.s.Observer.WebhookDone("error")
	m.s.Logger.Warn("webhook delivery failed", "job_id", snap.ID)
}

// Sign returns the hex HMAC-SHA256 of "<ts>.<body>" under secret.
func Sign(secret []byte, ts string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func checkWebhookURL(c *safefetch.Client, raw string) error {
	req, err := http.NewRequest(http.MethodPost, raw, nil)
	if err != nil {
		return errors.New("invalid URL")
	}
	return c.CheckURL(req.URL)
}

func (m *Manager) sweeper() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			m.mu.Lock()
			m.evictLocked()
			m.mu.Unlock()
		}
	}
}

// evictLocked drops expired finished jobs.
func (m *Manager) evictLocked() {
	now := time.Now()
	for id, j := range m.jobs {
		if !j.ExpiresAt.IsZero() && now.After(j.ExpiresAt) {
			delete(m.jobs, id)
		}
	}
}

func (j *Job) snapshot() Job {
	c := *j
	c.req, c.pdf, c.owner, c.webhook = nil, nil, "", ""
	c.Warnings = append([]string(nil), j.Warnings...)
	return c
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	return hex.EncodeToString(b[:])
}
