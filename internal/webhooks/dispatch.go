// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package webhooks

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// Start runs the dispatcher workers and the log sweeper.
func (s *Service) Start() {
	s.once.Do(func() {
		for range s.s.Workers {
			s.wg.Go(s.worker)
		}
		s.wg.Go(s.sweeper)
	})
}

// Shutdown stops claiming deliveries and waits for attempts in flight
// until ctx is done. A delivery cut off by shutdown is tried again once its
// claim lapses.
func (s *Service) Shutdown(ctx context.Context) error {
	s.stop()
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) worker() {
	t := time.NewTicker(s.s.PollInterval)
	defer t.Stop()
	for {
		if s.stopCtx.Err() != nil {
			return
		}
		worked, err := s.runOne()
		if err != nil && !errors.Is(err, context.Canceled) {
			s.s.Logger.Error("webhook claim failed", "error", err)
		}
		if worked {
			continue
		}
		select {
		case <-s.stopCtx.Done():
			return
		case <-s.wake:
		case <-t.C:
		}
	}
}

// lease is how long a claimed delivery is held: one attempt and the
// bookkeeping around it.
func (s *Service) lease() time.Duration { return s.s.Timeout + time.Minute }

// runOne claims and attempts a single delivery. It reports whether there was one.
func (s *Service) runOne() (bool, error) {
	now := time.Now().UTC()
	d, err := s.store.ClaimDelivery(s.stopCtx, now, s.lease())
	if err != nil || d == nil {
		return false, err
	}
	// Bookkeeping must finish even when shutdown starts mid-attempt.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.stopCtx), 30*time.Second)
	defer cancel()

	e, err := s.store.GetEndpoint(ctx, d.Owner, d.EndpointID)
	switch {
	case errors.Is(err, ErrNotFound):
		return true, s.giveUp(ctx, d, "endpoint was deleted")
	case err != nil:
		return true, err
	case !e.Enabled:
		return true, s.giveUp(ctx, d, "endpoint is disabled")
	case d.Attempts > d.MaxAttempts:
		return true, s.finish(ctx, e, d, attempt{at: now, err: "delivery was interrupted too many times"}, false)
	}
	a := s.attempt(s.stopCtx, e, d)
	if a.canceled {
		// Shutdown cut the request off: leave the claim to lapse, minus
		// the attempt it never really made.
		return true, nil
	}
	retry := !a.ok() && a.retryable && d.Attempts < d.MaxAttempts
	return true, s.finish(ctx, e, d, a, retry)
}

// giveUp fails a delivery without sending it; it does not count against
// the endpoint's health.
func (s *Service) giveUp(ctx context.Context, d *Delivery, why string) error {
	d.Status, d.Error, d.NextAttemptAt = Failed, why, time.Time{}
	s.s.Observer.WebhookDone("skipped")
	if err := s.store.FinishAttempt(ctx, d); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

func (s *Service) finish(ctx context.Context, e *Endpoint, d *Delivery, a attempt, retry bool) error {
	d.StatusCode, d.LatencyMS, d.Error, d.LastAttemptAt = a.statusCode, a.latency.Milliseconds(), a.err, a.at
	switch {
	case a.ok():
		d.Status, d.NextAttemptAt = Succeeded, time.Time{}
	case retry:
		d.Status, d.NextAttemptAt = Pending, a.at.Add(s.backoff(d.Attempts))
	default:
		d.Status, d.NextAttemptAt = Failed, time.Time{}
	}
	if err := s.store.FinishAttempt(ctx, d); err != nil {
		return err
	}
	if d.Status == Pending {
		return nil
	}
	s.s.Observer.WebhookDone(outcome(d.Status))
	paused, err := s.store.RecordResult(ctx, e.Owner, e.ID, Result{OK: a.ok(), StatusCode: a.statusCode, At: a.at}, s.s.PauseAfter)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if d.Status == Failed {
		s.s.Logger.Warn("webhook delivery failed", "endpoint_id", e.ID, "delivery_id", d.ID, "event", d.EventType,
			"status_code", a.statusCode, "error", a.err)
	}
	if paused {
		s.s.Logger.Warn("webhook endpoint paused after consecutive failures", "endpoint_id", e.ID, "owner", e.Owner,
			"failures", s.s.PauseAfter)
	}
	return nil
}

// backoff is the wait after the given attempt: RetryBackoff, then x6.
func (s *Service) backoff(attempts int) time.Duration {
	d := s.s.RetryBackoff
	for i := 1; i < attempts; i++ {
		d *= 6
	}
	return d
}

// outcome is the metric label of a finished delivery, as for per-job webhooks.
func outcome(st DeliveryStatus) string {
	if st == Succeeded {
		return "ok"
	}
	return "error"
}

type attempt struct {
	at         time.Time
	statusCode int
	latency    time.Duration
	err        string
	retryable  bool
	canceled   bool
}

func (a attempt) ok() bool { return a.err == "" && a.statusCode >= 200 && a.statusCode < 300 }

// signatures returns the signature header value: the current secret's
// signature, then the previous secret's while it is in its grace period.
func (s *Service) signatures(e *Endpoint, ts string, body []byte, now time.Time) (string, error) {
	cur, err := s.s.Sealer.Open(e.Secret)
	if err != nil {
		return "", err
	}
	sigs := []string{"sha256=" + Sign([]byte(cur), ts, body)}
	if len(e.PreviousSecret) > 0 && e.PreviousExpiresAt.After(now) {
		if prev, err := s.s.Sealer.Open(e.PreviousSecret); err == nil {
			sigs = append(sigs, "sha256="+Sign([]byte(prev), ts, body))
		}
	}
	return strings.Join(sigs, ","), nil
}

// attempt POSTs d's payload to e once.
func (s *Service) attempt(ctx context.Context, e *Endpoint, d *Delivery) attempt {
	start := time.Now()
	a := attempt{at: start.UTC()}
	ts := strconv.FormatInt(start.Unix(), 10)
	sig, err := s.signatures(e, ts, d.Payload, start)
	if err != nil {
		a.err = err.Error()
		return a
	}
	rctx, cancel := context.WithTimeout(ctx, s.s.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodPost, e.URL, bytes.NewReader(d.Payload))
	if err != nil {
		a.err = "invalid endpoint URL"
		return a
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Proteus-Event", d.EventType)
	req.Header.Set("X-Proteus-Delivery", d.ID)
	req.Header.Set("X-Proteus-Timestamp", ts)
	req.Header.Set("X-Proteus-Signature", sig)
	resp, err := s.s.Client.Do(req)
	a.latency = time.Since(start)
	if err != nil {
		switch {
		case ctx.Err() != nil:
			a.canceled, a.err = true, "canceled"
		case errors.Is(err, safefetch.ErrBlocked):
			a.err = "destination not allowed"
		case rctx.Err() != nil:
			a.err, a.retryable = "timed out after "+s.s.Timeout.String(), true
		default:
			a.err, a.retryable = "request failed: "+connError(err), true
		}
		return a
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	a.statusCode = resp.StatusCode
	if !a.ok() {
		a.err = "endpoint answered " + strconv.Itoa(resp.StatusCode)
		a.retryable = resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusRequestTimeout
	}
	return a
}

// connError keeps the useful tail of a transport error without the URL,
// which the log already shows.
func connError(err error) string {
	msg := err.Error()
	if i := strings.LastIndex(msg, ": "); i >= 0 && i+2 < len(msg) {
		msg = msg[i+2:]
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return msg
}

func (s *Service) sweeper() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for {
		select {
		case <-s.stopCtx.Done():
			return
		case <-t.C:
			s.Sweep()
		}
	}
}

// Sweep deletes deliveries older than the retention period.
func (s *Service) Sweep() {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(s.stopCtx), time.Minute)
	defer cancel()
	n, err := s.store.SweepDeliveries(ctx, time.Now().UTC().Add(-s.s.Retention))
	if err != nil {
		s.s.Logger.Warn("webhook log sweep failed", "error", err)
	} else if n > 0 {
		s.s.Logger.Info("old webhook deliveries removed", "count", n)
	}
}
