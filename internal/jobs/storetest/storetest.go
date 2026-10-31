// Package storetest is a conformance suite every jobs.Store must pass.
package storetest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gsoultan/proteus-server/internal/jobs"
)

// Run exercises s. The store must be empty.
func Run(t *testing.T, s jobs.Store) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("ClaimEmpty", func(t *testing.T) {
		r, err := s.Claim(ctx, now, time.Minute, 3)
		if err != nil || r != nil {
			t.Fatalf("Claim on empty store = %+v, %v", r, err)
		}
	})

	a := &jobs.Record{Owner: "alice", Format: "html", Payload: []byte("payload-a"), WebhookURL: "https://example.com/h"}
	t.Run("Create", func(t *testing.T) {
		if err := s.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
		if a.ID == "" || a.CreatedAt.IsZero() || a.Status != jobs.Queued {
			t.Fatalf("created %+v", a)
		}
		if n, err := s.CountQueued(ctx); err != nil || n != 1 {
			t.Fatalf("CountQueued = %d, %v", n, err)
		}
	})

	t.Run("OwnerScoping", func(t *testing.T) {
		if _, err := s.Get(ctx, "bob", a.ID); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("bob Get = %v", err)
		}
		if _, err := s.Get(ctx, "alice", "not-a-uuid"); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("bad id Get = %v", err)
		}
		got, err := s.Get(ctx, "alice", a.ID)
		if err != nil || got.Payload != nil || got.WebhookURL != a.WebhookURL {
			t.Fatalf("Get = %+v, %v", got, err)
		}
	})

	t.Run("ClaimAndFinish", func(t *testing.T) {
		r, err := s.Claim(ctx, now, time.Minute, 3)
		if err != nil || r == nil || r.ID != a.ID || string(r.Payload) != "payload-a" || r.Attempts != 1 || r.Status != jobs.Running {
			t.Fatalf("Claim = %+v, %v", r, err)
		}
		if again, err := s.Claim(ctx, now, time.Minute, 3); err != nil || again != nil {
			t.Fatalf("second Claim = %+v, %v", again, err)
		}
		if err := s.Delete(ctx, "alice", a.ID); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("Delete of running job = %v", err)
		}
		o := jobs.Outcome{Status: jobs.Succeeded, PDF: []byte("%PDF-1.7 x"), Warnings: []string{"w1"},
			FinishedAt: now, ExpiresAt: now.Add(time.Hour)}
		if err := s.Finish(ctx, a.ID, o); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, "alice", a.ID)
		if err != nil || got.Status != jobs.Succeeded || got.ResultBytes != int64(len(o.PDF)) ||
			len(got.Warnings) != 1 || got.ExpiresAt.IsZero() {
			t.Fatalf("after Finish = %+v, %v", got, err)
		}
		pdf, err := s.Result(ctx, "alice", a.ID)
		if err != nil || string(pdf) != string(o.PDF) {
			t.Fatalf("Result = %q, %v", pdf, err)
		}
		if _, err := s.Result(ctx, "bob", a.ID); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("bob Result = %v", err)
		}
	})

	t.Run("LapsedLeaseIsReclaimed", func(t *testing.T) {
		b := &jobs.Record{Owner: "alice", Format: "markdown", Payload: []byte("b")}
		if err := s.Create(ctx, b); err != nil {
			t.Fatal(err)
		}
		if r, err := s.Claim(ctx, now, time.Second, 2); err != nil || r == nil || r.ID != b.ID {
			t.Fatalf("claim b = %+v, %v", r, err)
		}
		later := now.Add(time.Minute)
		r, err := s.Claim(ctx, later, time.Second, 2)
		if err != nil || r == nil || r.ID != b.ID || r.Attempts != 2 {
			t.Fatalf("reclaim b = %+v, %v", r, err)
		}
		// Past maxAttempts the job fails instead of running again.
		r, err = s.Claim(ctx, later.Add(time.Minute), time.Second, 2)
		if err != nil || r != nil {
			t.Fatalf("third claim = %+v, %v", r, err)
		}
		got, _ := s.Get(ctx, "alice", b.ID)
		if got.Status != jobs.Failed || got.Error == "" {
			t.Fatalf("exhausted job = %+v", got)
		}
	})

	t.Run("Release", func(t *testing.T) {
		c := &jobs.Record{Owner: "carol", Format: "html", Payload: []byte("c")}
		if err := s.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
		if r, _ := s.Claim(ctx, now, time.Minute, 3); r == nil || r.ID != c.ID {
			t.Fatalf("claim c = %+v", r)
		}
		if err := s.Release(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, "carol", c.ID)
		if got.Status != jobs.Queued {
			t.Fatalf("released status %s", got.Status)
		}
		r, _ := s.Claim(ctx, now, time.Minute, 3)
		if r == nil || r.ID != c.ID {
			t.Fatalf("claim after release = %+v", r)
		}
		s.Finish(ctx, c.ID, jobs.Outcome{Status: jobs.Failed, Error: "x", FinishedAt: now, ExpiresAt: now.Add(-time.Second)})
	})

	t.Run("ConcurrentClaimsAreDisjoint", func(t *testing.T) {
		const n = 20
		for range n {
			if err := s.Create(ctx, &jobs.Record{Owner: "dave", Format: "html", Payload: []byte("d")}); err != nil {
				t.Fatal(err)
			}
		}
		var mu sync.Mutex
		seen := map[string]int{}
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for {
					r, err := s.Claim(ctx, now, time.Hour, 3)
					if err != nil {
						t.Error(err)
						return
					}
					if r == nil {
						return
					}
					mu.Lock()
					seen[r.ID]++
					mu.Unlock()
				}
			})
		}
		wg.Wait()
		if len(seen) != n {
			t.Fatalf("claimed %d distinct jobs, want %d", len(seen), n)
		}
		for id, c := range seen {
			if c != 1 {
				t.Fatalf("job %s claimed %d times", id, c)
			}
		}
	})

	t.Run("DeleteAndSweep", func(t *testing.T) {
		if err := s.Delete(ctx, "alice", a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, "alice", a.ID); !errors.Is(err, jobs.ErrNotFound) {
			t.Fatalf("after Delete = %v", err)
		}
		n, err := s.Sweep(ctx, now)
		if err != nil || n < 1 {
			t.Fatalf("Sweep = %d, %v (the released job expired)", n, err)
		}
	})
}
