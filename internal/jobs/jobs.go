// Package jobs runs asynchronous conversions on top of a Store: a memory
// store for a single instance, or the Postgres store so that every instance
// shares one queue. Workers claim jobs with a lease, so a job whose worker
// died is picked up again once the lease runs out.
package jobs

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
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
	// ErrNotFinished is returned when a job's result or deletion is asked for too early.
	ErrNotFinished = errors.New("job has not finished")
)

// Settings configure a Manager.
type Settings struct {
	Workers       int           // concurrent async conversions; default 2
	QueueSize     int64         // queued jobs before Submit refuses; default 100
	ResultTTL     time.Duration // how long finished jobs are kept; default 1h
	Lease         time.Duration // how long a claim lasts; default conversion timeout + 1m
	MaxAttempts   int           // claims per job before it fails; default 3
	PollInterval  time.Duration // how often idle workers look for work; default 1s
	MaxAssets     int
	WebhookSecret []byte // HMAC-SHA256 key for webhook signatures; empty disables webhooks
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

// Manager owns the workers on top of a Store.
type Manager struct {
	s     Settings
	svc   *convert.Service
	store Store

	wake    chan struct{}
	mu      sync.Mutex
	closed  bool
	stop    context.CancelFunc // stops claiming
	stopCtx context.Context
	kill    context.CancelFunc // cancels running conversions
	killCtx context.Context
	workers sync.WaitGroup
	loops   sync.WaitGroup
}

// NewManager starts the workers and the expiry sweeper.
func NewManager(svc *convert.Service, store Store, s Settings) *Manager {
	if s.Workers <= 0 {
		s.Workers = 2
	}
	if s.QueueSize <= 0 {
		s.QueueSize = 100
	}
	if s.ResultTTL <= 0 {
		s.ResultTTL = time.Hour
	}
	if s.Lease <= 0 {
		s.Lease = svc.Timeout() + time.Minute
	}
	if s.MaxAttempts <= 0 {
		s.MaxAttempts = 3
	}
	if s.PollInterval <= 0 {
		s.PollInterval = time.Second
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Observer == nil {
		s.Observer = nopObserver{}
	}
	m := &Manager{s: s, svc: svc, store: store, wake: make(chan struct{}, s.Workers)}
	m.stopCtx, m.stop = context.WithCancel(context.Background())
	m.killCtx, m.kill = context.WithCancel(context.Background())
	for range s.Workers {
		m.workers.Go(m.worker)
	}
	m.loops.Go(m.sweeper)
	return m
}

// WebhooksEnabled reports whether jobs may ask for a callback.
func (m *Manager) WebhooksEnabled() bool { return len(m.s.WebhookSecret) > 0 && m.s.Webhook != nil }

// Submit validates and queues req for owner (the API key id).
func (m *Manager) Submit(ctx context.Context, owner string, req *proteusv1.SubmitJobRequest) (*Record, error) {
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return nil, ErrClosed
	}
	parsed, err := convert.FromProto(req.GetDocument(), req.GetOutputFormat(), req.GetOptions(), convert.Limits{MaxAssets: m.s.MaxAssets})
	if err != nil {
		return nil, err
	}
	if err := m.svc.Check(parsed); err != nil {
		return nil, err
	}
	if hook := req.GetWebhookUrl(); hook != "" {
		if !m.WebhooksEnabled() {
			return nil, &convert.BadRequestError{Msg: "webhooks are not enabled on this server"}
		}
		if err := checkWebhookURL(m.s.Webhook, hook); err != nil {
			return nil, &convert.BadRequestError{Msg: "webhook_url: " + err.Error()}
		}
	}
	n, err := m.store.CountQueued(ctx)
	if err != nil {
		return nil, err
	}
	m.s.Observer.QueueDepth(int(n))
	if n >= m.s.QueueSize {
		return nil, ErrQueueFull
	}
	payload, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	r := &Record{
		Owner: owner, InputFormat: string(parsed.Format), OutputFormat: string(parsed.Output),
		FileName: convert.FileName(req.GetOptions().GetMetadata().GetTitle(), parsed.Output),
		Payload:  payload, WebhookURL: req.GetWebhookUrl(),
	}
	if err := m.store.Create(ctx, r); err != nil {
		return nil, err
	}
	r.Payload = nil
	select {
	case m.wake <- struct{}{}:
	default:
	}
	return r, nil
}

// Get returns owner's job.
func (m *Manager) Get(ctx context.Context, owner, id string) (*Record, error) {
	return m.store.Get(ctx, owner, id)
}

// Result returns the output of owner's succeeded job.
func (m *Manager) Result(ctx context.Context, owner, id string) (*Record, []byte, error) {
	r, err := m.store.Get(ctx, owner, id)
	if err != nil {
		return nil, nil, err
	}
	if r.Status != Succeeded {
		return r, nil, ErrNotFinished
	}
	out, err := m.store.Result(ctx, owner, id)
	return r, out, err
}

// Delete removes owner's finished job.
func (m *Manager) Delete(ctx context.Context, owner, id string) error {
	r, err := m.store.Get(ctx, owner, id)
	if err != nil {
		return err
	}
	if r.Status == Queued || r.Status == Running {
		return ErrNotFinished
	}
	return m.store.Delete(ctx, owner, id)
}

// Shutdown stops claiming jobs and lets running ones finish until ctx is
// done. Conversions still running then are canceled and their jobs go back
// to the queue for another instance (or the next start) to pick up.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.stop()
	done := make(chan struct{})
	go func() { m.workers.Wait(); close(done) }()
	var err error
	select {
	case <-done:
	case <-ctx.Done():
		err = ctx.Err()
		m.kill()
		<-done
	}
	m.kill()
	m.loops.Wait()
	return err
}

func (m *Manager) worker() {
	t := time.NewTicker(m.s.PollInterval)
	defer t.Stop()
	for {
		if m.stopCtx.Err() != nil {
			return
		}
		worked, err := m.runOne()
		if err != nil {
			m.s.Logger.Error("job claim failed", "error", err)
		}
		if worked {
			continue
		}
		select {
		case <-m.stopCtx.Done():
			return
		case <-m.wake:
		case <-t.C:
		}
	}
}

// runOne claims and runs a single job. It reports whether there was one.
func (m *Manager) runOne() (bool, error) {
	// Hold a conversion slot before claiming so a claimed job never waits
	// on the pool while its lease runs.
	release, err := m.svc.Acquire(m.stopCtx)
	if err != nil {
		return false, nil
	}
	defer release()
	rec, err := m.store.Claim(m.stopCtx, time.Now().UTC(), m.s.Lease, m.s.MaxAttempts)
	if err != nil || rec == nil {
		if errors.Is(err, context.Canceled) {
			err = nil
		}
		return false, err
	}
	res, runErr := m.convert(rec)
	if errors.Is(runErr, context.Canceled) && m.killCtx.Err() != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.Release(ctx, rec.ID); err != nil {
			m.s.Logger.Warn("job release failed", "job_id", rec.ID, "error", err)
		}
		return true, nil
	}
	now := time.Now().UTC()
	o := Outcome{FinishedAt: now, ExpiresAt: now.Add(m.s.ResultTTL)}
	if runErr != nil {
		o.Status, o.Error = Failed, publicError(runErr)
		m.s.Logger.Warn("job failed", "job_id", rec.ID, "error", runErr)
	} else {
		o.Status, o.Result, o.Warnings = Succeeded, res.Content, res.Warnings
		m.s.Logger.Info("job succeeded", "job_id", rec.ID, "output", rec.OutputFormat, "bytes", len(res.Content))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := m.store.Finish(ctx, rec.ID, o); err != nil {
		return true, err
	}
	if rec.WebhookURL != "" {
		rec.Status, rec.Error, rec.Warnings = o.Status, o.Error, o.Warnings
		rec.FinishedAt, rec.ExpiresAt, rec.ResultBytes = o.FinishedAt, o.ExpiresAt, int64(len(o.Result))
		m.deliver(rec.WebhookURL, rec)
	}
	return true, nil
}

func (m *Manager) convert(rec *Record) (*convert.Result, error) {
	var req proteusv1.SubmitJobRequest
	if err := proto.Unmarshal(rec.Payload, &req); err != nil {
		return nil, &convert.BadRequestError{Msg: "stored job is unreadable"}
	}
	parsed, err := convert.FromProto(req.GetDocument(), req.GetOutputFormat(), req.GetOptions(), convert.Limits{MaxAssets: m.s.MaxAssets})
	if err != nil {
		return nil, err
	}
	return m.svc.Run(m.killCtx, parsed)
}

func publicError(err error) string {
	var bre *convert.BadRequestError
	switch {
	case errors.As(err, &bre):
		return bre.Msg
	case errors.Is(err, convert.ErrTimeout):
		return err.Error()
	}
	return "conversion failed"
}

// ToProto converts a record to its API form.
func ToProto(r *Record) *proteusv1.Job {
	j := &proteusv1.Job{
		Id: r.ID, Error: r.Error, Warnings: r.Warnings, ResultBytes: r.ResultBytes,
		CreateTime: ts(r.CreatedAt), StartTime: ts(r.StartedAt), FinishTime: ts(r.FinishedAt), ExpireTime: ts(r.ExpiresAt),
	}
	switch r.Status {
	case Queued:
		j.Status = proteusv1.JobStatus_JOB_STATUS_QUEUED
	case Running:
		j.Status = proteusv1.JobStatus_JOB_STATUS_RUNNING
	case Succeeded:
		j.Status = proteusv1.JobStatus_JOB_STATUS_SUCCEEDED
	case Failed:
		j.Status = proteusv1.JobStatus_JOB_STATUS_FAILED
	}
	j.InputFormat = convert.InputToProto(convert.InputFormat(r.InputFormat))
	out := convert.OutputFormat(r.OutputFormat)
	j.OutputFormat = convert.OutputToProto(out)
	if t, ok := convert.TargetOf(out); ok {
		j.MimeType = t.MIMEType
	}
	j.FileName = r.FileName
	return j
}

func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// deliver POSTs the job (protobuf JSON) to the webhook, signed with
// HMAC-SHA256 over "<timestamp>.<body>", retrying a few times with backoff.
func (m *Manager) deliver(target string, rec *Record) {
	body, err := protojson.Marshal(ToProto(rec))
	if err != nil {
		m.s.Observer.WebhookDone("error")
		return
	}
	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		ctx, cancel := context.WithTimeout(m.killCtx, 10*time.Second)
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
		case <-m.killCtx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 4
	}
	m.s.Observer.WebhookDone("error")
	m.s.Logger.Warn("webhook delivery failed", "job_id", rec.ID)
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
		case <-m.killCtx.Done():
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(m.killCtx, 30*time.Second)
			if n, err := m.store.Sweep(ctx, time.Now().UTC()); err != nil {
				m.s.Logger.Warn("job sweep failed", "error", err)
			} else if n > 0 {
				m.s.Logger.Info("expired jobs removed", "count", n)
			}
			if n, err := m.store.CountQueued(ctx); err == nil {
				m.s.Observer.QueueDepth(int(n))
			}
			cancel()
		}
	}
}
