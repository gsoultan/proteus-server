// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package settings

import (
	"context"
	"maps"
	"slices"
	"sync"
)

// MemoryStore keeps settings in memory, for tests.
type MemoryStore struct {
	mu   sync.Mutex
	doc  Doc
	keys []APIKey
}

// NewMemoryStore returns an empty store.
func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

// Load implements Store.
func (s *MemoryStore) Load(context.Context) (Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.doc
	d.Values = maps.Clone(d.Values)
	return d, nil
}

// Save implements Store.
func (s *MemoryStore) Save(_ context.Context, d Doc, expected int64) (Doc, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.doc.Revision != expected {
		return Doc{}, ErrRevisionMismatch
	}
	d.Revision = expected + 1
	d.Values = maps.Clone(d.Values)
	s.doc = d
	return d, nil
}

// ListKeys implements Store.
func (s *MemoryStore) ListKeys(context.Context) ([]APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.keys), nil
}

// CreateKey implements Store.
func (s *MemoryStore) CreateKey(_ context.Context, k APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slices.ContainsFunc(s.keys, func(x APIKey) bool { return x.ID == k.ID }) {
		return ErrKeyExists
	}
	s.keys = append(s.keys, k)
	return nil
}

// DeleteKey implements Store.
func (s *MemoryStore) DeleteKey(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.keys, func(x APIKey) bool { return x.ID == id })
	if i < 0 {
		return ErrNotFound
	}
	s.keys = slices.Delete(s.keys, i, i+1)
	return nil
}

var _ Store = (*MemoryStore)(nil)
