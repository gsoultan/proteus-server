// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package webhooks

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore keeps endpoints and deliveries in process memory: lost on
// restart and visible to one instance only. It holds at most maxDeliveries
// deliveries, dropping the oldest finished ones first.
type MemoryStore struct {
	mu            sync.Mutex
	endpoints     map[string]*Endpoint
	deliveries    map[string]*Delivery
	maxDeliveries int
}

// NewMemoryStore returns an empty store (maxDeliveries default 10000).
func NewMemoryStore(maxDeliveries int) *MemoryStore {
	if maxDeliveries <= 0 {
		maxDeliveries = 10000
	}
	return &MemoryStore{endpoints: map[string]*Endpoint{}, deliveries: map[string]*Delivery{}, maxDeliveries: maxDeliveries}
}

func copyEndpoint(e *Endpoint) *Endpoint {
	c := *e
	c.Events = slices.Clone(e.Events)
	c.Secret = slices.Clone(e.Secret)
	c.PreviousSecret = slices.Clone(e.PreviousSecret)
	return &c
}

// CreateEndpoint implements Store.
func (s *MemoryStore) CreateEndpoint(_ context.Context, e *Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	e.ID, e.CreatedAt, e.UpdatedAt = uuid.NewString(), now, now
	s.endpoints[e.ID] = copyEndpoint(e)
	return nil
}

func (s *MemoryStore) endpoint(owner, id string) (*Endpoint, bool) {
	e, ok := s.endpoints[strings.ToLower(id)]
	if !ok || e.Owner != owner {
		return nil, false
	}
	return e, true
}

// GetEndpoint implements Store.
func (s *MemoryStore) GetEndpoint(_ context.Context, owner, id string) (*Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.endpoint(owner, id)
	if !ok {
		return nil, ErrNotFound
	}
	return copyEndpoint(e), nil
}

// ListEndpoints implements Store.
func (s *MemoryStore) ListEndpoints(_ context.Context, owner string) ([]*Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Endpoint
	for _, e := range s.endpoints {
		if e.Owner == owner {
			out = append(out, copyEndpoint(e))
		}
	}
	slices.SortFunc(out, func(a, b *Endpoint) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out, nil
}

// CountEndpoints implements Store.
func (s *MemoryStore) CountEndpoints(_ context.Context, owner string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for _, e := range s.endpoints {
		if e.Owner == owner {
			n++
		}
	}
	return n, nil
}

// UpdateEndpoint implements Store.
func (s *MemoryStore) UpdateEndpoint(_ context.Context, e *Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.endpoint(e.Owner, e.ID)
	if !ok {
		return ErrNotFound
	}
	cur.URL, cur.Description, cur.Events = e.URL, e.Description, slices.Clone(e.Events)
	cur.UpdatedAt = time.Now().UTC()
	return nil
}

// SetEnabled implements Store.
func (s *MemoryStore) SetEnabled(_ context.Context, owner, id string, enabled bool, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.endpoint(owner, id)
	if !ok {
		return ErrNotFound
	}
	cur.Enabled, cur.UpdatedAt = enabled, now
	if enabled {
		cur.PausedAt, cur.ConsecutiveFailures = time.Time{}, 0
	}
	return nil
}

// SetSecrets implements Store.
func (s *MemoryStore) SetSecrets(_ context.Context, owner, id string, secret, previous []byte, previousExpires time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.endpoint(owner, id)
	if !ok {
		return ErrNotFound
	}
	cur.Secret, cur.PreviousSecret, cur.PreviousExpiresAt = slices.Clone(secret), slices.Clone(previous), previousExpires
	cur.UpdatedAt = time.Now().UTC()
	return nil
}

// RecordResult implements Store.
func (s *MemoryStore) RecordResult(_ context.Context, owner, id string, r Result, pauseAfter int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.endpoint(owner, id)
	if !ok {
		return false, ErrNotFound
	}
	cur.LastDeliveryAt, cur.LastStatusCode = r.At, r.StatusCode
	if r.OK {
		cur.ConsecutiveFailures = 0
		return false, nil
	}
	cur.ConsecutiveFailures++
	if cur.Enabled && pauseAfter > 0 && cur.ConsecutiveFailures >= pauseAfter {
		cur.Enabled, cur.PausedAt, cur.UpdatedAt = false, r.At, r.At
		return true, nil
	}
	return false, nil
}

// DeleteEndpoint implements Store.
func (s *MemoryStore) DeleteEndpoint(_ context.Context, owner, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.endpoint(owner, id)
	if !ok {
		return ErrNotFound
	}
	delete(s.endpoints, cur.ID)
	for k, d := range s.deliveries {
		if d.EndpointID == cur.ID {
			delete(s.deliveries, k)
		}
	}
	return nil
}

// CreateDelivery implements Store.
func (s *MemoryStore) CreateDelivery(_ context.Context, d *Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.deliveries) >= s.maxDeliveries {
		s.evictLocked()
	}
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	d.CreatedAt = time.Now().UTC()
	c := *d
	c.Payload = slices.Clone(d.Payload)
	s.deliveries[d.ID] = &c
	return nil
}

// evictLocked drops the oldest finished delivery.
func (s *MemoryStore) evictLocked() {
	var oldest *Delivery
	for _, d := range s.deliveries {
		if d.Status != Pending && (oldest == nil || d.CreatedAt.Before(oldest.CreatedAt)) {
			oldest = d
		}
	}
	if oldest != nil {
		delete(s.deliveries, oldest.ID)
	}
}

// GetDelivery implements Store.
func (s *MemoryStore) GetDelivery(_ context.Context, owner, id string) (*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.deliveries[strings.ToLower(id)]
	if !ok || d.Owner != owner {
		return nil, ErrNotFound
	}
	c := *d
	c.Payload = slices.Clone(d.Payload)
	return &c, nil
}

// newestFirst orders deliveries by CreatedAt then ID, both descending.
func newestFirst(a, b *Delivery) int {
	if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
		return c
	}
	return strings.Compare(b.ID, a.ID)
}

// ListDeliveries implements Store.
func (s *MemoryStore) ListDeliveries(_ context.Context, owner, endpointID string, status DeliveryStatus, after *Cursor, limit int) ([]*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Delivery
	for _, d := range s.deliveries {
		if d.Owner != owner || d.EndpointID != strings.ToLower(endpointID) || (status != "" && d.Status != status) {
			continue
		}
		if after != nil && newestFirst(&Delivery{CreatedAt: after.CreatedAt, ID: after.ID}, d) >= 0 {
			continue
		}
		c := *d
		c.Payload = nil
		out = append(out, &c)
	}
	slices.SortFunc(out, newestFirst)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ClaimDelivery implements Store.
func (s *MemoryStore) ClaimDelivery(_ context.Context, now time.Time, lease time.Duration) (*Delivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var next *Delivery
	for _, d := range s.deliveries {
		if d.Status == Pending && !d.NextAttemptAt.After(now) &&
			(next == nil || d.CreatedAt.Before(next.CreatedAt)) {
			next = d
		}
	}
	if next == nil {
		return nil, nil
	}
	next.Attempts++
	next.NextAttemptAt = now.Add(lease)
	c := *next
	c.Payload = slices.Clone(next.Payload)
	return &c, nil
}

// FinishAttempt implements Store.
func (s *MemoryStore) FinishAttempt(_ context.Context, d *Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.deliveries[d.ID]
	if !ok {
		return ErrNotFound
	}
	cur.Status, cur.StatusCode, cur.LatencyMS, cur.Error = d.Status, d.StatusCode, d.LatencyMS, d.Error
	cur.LastAttemptAt, cur.NextAttemptAt = d.LastAttemptAt, d.NextAttemptAt
	return nil
}

// SweepDeliveries implements Store.
func (s *MemoryStore) SweepDeliveries(_ context.Context, t time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	for k, d := range s.deliveries {
		if d.CreatedAt.Before(t) {
			delete(s.deliveries, k)
			n++
		}
	}
	return n, nil
}

var _ Store = (*MemoryStore)(nil)
