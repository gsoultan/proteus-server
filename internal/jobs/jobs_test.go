package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	proteus "github.com/gsoultan/proteus-converter"

	"github.com/gsoultan/proteus-server/internal/convert"
)

func req() *convert.Request {
	return &convert.Request{Format: proteus.FormatMarkdown, Source: []byte("# x")}
}

func TestQueueFullAndShutdown(t *testing.T) {
	svc := convert.NewService(convert.Settings{Workers: 1, SystemFonts: false})
	// Hold the only slot so jobs stay queued.
	release, _ := svc.TryAcquire()
	m := NewManager(svc, Settings{Workers: 1, QueueSize: 2})
	var ids []string
	for {
		j, err := m.Submit("k", req())
		if errors.Is(err, ErrQueueFull) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, j.ID)
		if len(ids) > 10 {
			t.Fatal("queue never filled")
		}
	}
	// Two sit in the queue, plus possibly one the worker already took.
	if len(ids) < 2 || len(ids) > 3 {
		t.Fatalf("accepted %d jobs, want 2 or 3", len(ids))
	}
	release()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		j, err := m.Get("k", id)
		if err != nil || j.Status != Succeeded {
			t.Fatalf("job %s: %+v %v", id, j, err)
		}
	}
	if _, err := m.Submit("k", req()); !errors.Is(err, ErrClosed) {
		t.Fatalf("submit after shutdown = %v", err)
	}
}

func TestShutdownDeadlineFailsQueuedJobs(t *testing.T) {
	svc := convert.NewService(convert.Settings{Workers: 1, SystemFonts: false})
	release, _ := svc.TryAcquire() // never released before the deadline
	defer release()
	m := NewManager(svc, Settings{Workers: 1, QueueSize: 4})
	j, err := m.Submit("k", req())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := m.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown = %v", err)
	}
	got, _ := m.Get("k", j.ID)
	if got.Status != Failed {
		t.Fatalf("status %s, want failed", got.Status)
	}
}

func TestOwnership(t *testing.T) {
	svc := convert.NewService(convert.Settings{Workers: 1, SystemFonts: false})
	m := NewManager(svc, Settings{})
	defer m.Shutdown(context.Background())
	j, err := m.Submit("alice", req())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get("bob", j.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob sees alice's job: %v", err)
	}
	if err := m.Delete("bob", j.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob deletes alice's job: %v", err)
	}
}

func TestWebhookNeedsSecret(t *testing.T) {
	m := NewManager(convert.NewService(convert.Settings{}), Settings{})
	defer m.Shutdown(context.Background())
	r := req()
	r.WebhookURL = "https://example.com/hook"
	var bre *convert.BadRequestError
	if _, err := m.Submit("k", r); !errors.As(err, &bre) {
		t.Fatalf("webhook without secret: %v", err)
	}
}

func TestSign(t *testing.T) {
	if got := Sign([]byte("k"), "1", []byte("{}")); len(got) != 64 {
		t.Fatalf("signature %q", got)
	}
	if Sign([]byte("k"), "1", []byte("{}")) == Sign([]byte("k"), "2", []byte("{}")) {
		t.Fatal("timestamp not covered by the signature")
	}
}
