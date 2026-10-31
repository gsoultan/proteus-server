package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/convert"
)

func submitReq() *proteusv1.SubmitJobRequest {
	return &proteusv1.SubmitJobRequest{Document: &proteusv1.Document{
		Format: proteusv1.Format_FORMAT_MARKDOWN,
		Source: &proteusv1.Document_Content{Content: []byte("# x")},
	}}
}

func newSvc() *convert.Service {
	return convert.NewService(convert.Settings{Workers: 1, SystemFonts: false})
}

func waitStatus(t *testing.T, m *Manager, owner, id string, want Status) *Record {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r, err := m.Get(context.Background(), owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status == want {
			return r
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s", id, want)
	return nil
}

func TestLifecycle(t *testing.T) {
	m := NewManager(newSvc(), NewMemoryStore(0), Settings{Workers: 1, PollInterval: 10 * time.Millisecond})
	defer m.Shutdown(context.Background())
	ctx := context.Background()
	r, err := m.Submit(ctx, "alice", submitReq())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != Queued || r.ID == "" {
		t.Fatalf("submitted %+v", r)
	}
	done := waitStatus(t, m, "alice", r.ID, Succeeded)
	if done.ResultBytes == 0 || done.ExpiresAt.IsZero() {
		t.Fatalf("finished %+v", done)
	}
	_, pdf, err := m.Result(ctx, "alice", r.ID)
	if err != nil || len(pdf) != int(done.ResultBytes) {
		t.Fatalf("result %d bytes, %v", len(pdf), err)
	}
	if _, err := m.Get(ctx, "bob", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob sees alice's job: %v", err)
	}
	if err := m.Delete(ctx, "bob", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bob deletes alice's job: %v", err)
	}
	if err := m.Delete(ctx, "alice", r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(ctx, "alice", r.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestQueueFull(t *testing.T) {
	svc := newSvc()
	release, _ := svc.TryAcquire() // hold the only slot so jobs stay queued
	defer release()
	m := NewManager(svc, NewMemoryStore(0), Settings{Workers: 1, QueueSize: 2})
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		m.Shutdown(ctx)
	}()
	ctx := context.Background()
	for i := range 2 {
		if _, err := m.Submit(ctx, "k", submitReq()); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	if _, err := m.Submit(ctx, "k", submitReq()); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third submit = %v", err)
	}
}

func TestShutdownRequeuesRunningJob(t *testing.T) {
	store := NewMemoryStore(0)
	svc := newSvc()
	m := NewManager(svc, store, Settings{Workers: 1, PollInterval: 10 * time.Millisecond})
	ctx := context.Background()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Submit(ctx, "k", submitReq()); !errors.Is(err, ErrClosed) {
		t.Fatalf("submit after shutdown = %v", err)
	}
	// A claimed job whose conversion is canceled goes back to the queue.
	r := &Record{Owner: "k", Format: "markdown"}
	if err := store.Create(ctx, r); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Claim(ctx, time.Now(), time.Minute, 3); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get(ctx, "k", r.ID)
	if got.Status != Queued {
		t.Fatalf("status %s, want queued", got.Status)
	}
}

func TestValidationAndWebhook(t *testing.T) {
	m := NewManager(newSvc(), NewMemoryStore(0), Settings{})
	defer m.Shutdown(context.Background())
	ctx := context.Background()
	var bre *convert.BadRequestError
	if _, err := m.Submit(ctx, "k", &proteusv1.SubmitJobRequest{}); !errors.As(err, &bre) {
		t.Fatalf("empty request: %v", err)
	}
	r := submitReq()
	r.WebhookUrl = "https://example.com/hook"
	if _, err := m.Submit(ctx, "k", r); !errors.As(err, &bre) {
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
