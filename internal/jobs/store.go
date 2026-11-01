package jobs

import (
	"context"
	"time"
)

// Record is a job as a Store holds it.
type Record struct {
	ID           string
	Owner        string // API key id that submitted the job
	Status       Status
	InputFormat  string // convert.InputFormat
	OutputFormat string // convert.OutputFormat
	FileName     string // suggested download name of the result
	Payload      []byte // serialized SubmitJobRequest; only set by Claim
	WebhookURL   string
	Attempts     int
	CreatedAt    time.Time
	StartedAt    time.Time
	FinishedAt   time.Time
	ExpiresAt    time.Time
	Error        string
	Warnings     []string
	ResultBytes  int64
}

// Outcome is how a job ended.
type Outcome struct {
	Status     Status // Succeeded or Failed
	Error      string
	Warnings   []string
	Result     []byte
	FinishedAt time.Time
	ExpiresAt  time.Time
}

// Store persists jobs. The memory store serves one instance; the Postgres
// store (internal/storage/pgstore) lets every instance share one queue.
// Implementations must be safe for concurrent use.
type Store interface {
	// Create inserts a queued job and fills in ID and CreatedAt.
	Create(ctx context.Context, r *Record) error
	// CountQueued returns how many jobs wait for a worker.
	CountQueued(ctx context.Context) (int64, error)
	// Claim takes the oldest queued job, or a running one whose lease ran
	// out, marks it running until now+lease and returns it with its
	// payload. A lapsed job that already had maxAttempts attempts is
	// failed instead. It returns nil when there is nothing to do.
	Claim(ctx context.Context, now time.Time, lease time.Duration, maxAttempts int) (*Record, error)
	// Finish records the outcome of a claimed job.
	Finish(ctx context.Context, id string, o Outcome) error
	// Release puts a claimed job back in the queue (used on shutdown).
	Release(ctx context.Context, id string) error
	// Get returns a job of owner without payload or result; ErrNotFound otherwise.
	Get(ctx context.Context, owner, id string) (*Record, error)
	// Result returns the output of owner's succeeded job.
	Result(ctx context.Context, owner, id string) ([]byte, error)
	// Delete removes owner's finished job; ErrNotFound when there is none.
	Delete(ctx context.Context, owner, id string) error
	// Sweep deletes jobs whose ExpiresAt is before now.
	Sweep(ctx context.Context, now time.Time) (int64, error)
}
