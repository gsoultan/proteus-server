package jobs

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// MemoryStore keeps jobs in process memory. Jobs are lost on restart and are
// visible only on the instance that accepted them; use the Postgres store
// for more than one instance.
type MemoryStore struct {
	mu      sync.Mutex
	jobs    map[string]*memJob
	maxJobs int
}

type memJob struct {
	Record
	lease  time.Time
	result []byte
}

// NewMemoryStore returns a store holding at most maxJobs jobs (default 1000).
func NewMemoryStore(maxJobs int) *MemoryStore {
	if maxJobs <= 0 {
		maxJobs = 1000
	}
	return &MemoryStore{jobs: map[string]*memJob{}, maxJobs: maxJobs}
}

// Create implements Store.
func (s *MemoryStore) Create(_ context.Context, r *Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.jobs) >= s.maxJobs {
		s.sweepLocked(time.Now())
		if len(s.jobs) >= s.maxJobs {
			return ErrQueueFull
		}
	}
	r.ID, r.CreatedAt, r.Status = newUUID(), time.Now().UTC(), Queued
	j := &memJob{Record: *r}
	j.Payload = append([]byte(nil), r.Payload...)
	s.jobs[r.ID] = j
	return nil
}

// CountQueued implements Store.
func (s *MemoryStore) CountQueued(context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, j := range s.jobs {
		if j.Status == Queued {
			n++
		}
	}
	return n, nil
}

// Claim implements Store.
func (s *MemoryStore) Claim(_ context.Context, now time.Time, lease time.Duration, maxAttempts int) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var best *memJob
	for _, j := range s.jobs {
		lapsed := j.Status == Running && now.After(j.lease)
		if lapsed && j.Attempts >= maxAttempts {
			j.Status, j.Error, j.FinishedAt = Failed, "conversion was interrupted too many times", now
			continue
		}
		if j.Status != Queued && !lapsed {
			continue
		}
		if best == nil || j.CreatedAt.Before(best.CreatedAt) {
			best = j
		}
	}
	if best == nil {
		return nil, nil
	}
	best.Status, best.lease, best.StartedAt = Running, now.Add(lease), now
	best.Attempts++
	r := best.Record
	r.Payload = append([]byte(nil), best.Payload...)
	return &r, nil
}

// Finish implements Store.
func (s *MemoryStore) Finish(_ context.Context, id string, o Outcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return ErrNotFound
	}
	j.Status, j.Error, j.Warnings = o.Status, o.Error, append([]string(nil), o.Warnings...)
	j.FinishedAt, j.ExpiresAt = o.FinishedAt, o.ExpiresAt
	j.result, j.ResultBytes, j.Payload = o.Result, int64(len(o.Result)), nil
	return nil
}

// Release implements Store.
func (s *MemoryStore) Release(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j, ok := s.jobs[id]; ok && j.Status == Running {
		j.Status, j.StartedAt, j.lease = Queued, time.Time{}, time.Time{}
	}
	return nil
}

func (s *MemoryStore) owned(owner, id string) (*memJob, error) {
	j, ok := s.jobs[id]
	if !ok || j.Owner != owner {
		return nil, ErrNotFound
	}
	return j, nil
}

// Get implements Store.
func (s *MemoryStore) Get(_ context.Context, owner, id string) (*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.owned(owner, id)
	if err != nil {
		return nil, err
	}
	r := j.Record
	r.Payload = nil
	r.Warnings = append([]string(nil), j.Warnings...)
	return &r, nil
}

// Result implements Store.
func (s *MemoryStore) Result(_ context.Context, owner, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.owned(owner, id)
	if err != nil {
		return nil, err
	}
	return j.result, nil
}

// Delete implements Store.
func (s *MemoryStore) Delete(_ context.Context, owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, err := s.owned(owner, id)
	if err != nil || (j.Status != Succeeded && j.Status != Failed) {
		return ErrNotFound
	}
	delete(s.jobs, id)
	return nil
}

// Sweep implements Store.
func (s *MemoryStore) Sweep(_ context.Context, now time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sweepLocked(now), nil
}

func (s *MemoryStore) sweepLocked(now time.Time) int64 {
	var n int64
	for id, j := range s.jobs {
		if !j.ExpiresAt.IsZero() && now.After(j.ExpiresAt) {
			delete(s.jobs, id)
			n++
		}
	}
	return n
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand: %v", err))
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
