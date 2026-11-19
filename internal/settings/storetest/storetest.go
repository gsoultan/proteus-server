// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package storetest is the conformance suite every settings.Store passes.
package storetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gsoultan/proteus-server/internal/settings"
)

// Run tests a fresh, empty store.
func Run(t *testing.T, s settings.Store) {
	ctx := context.Background()

	d, err := s.Load(ctx)
	if err != nil || d.Revision != 0 || d.SetupDone() || len(d.Values) != 0 {
		t.Fatalf("empty Load = %+v, %v", d, err)
	}
	if _, err := s.Save(ctx, settings.Doc{}, 3); !errors.Is(err, settings.ErrRevisionMismatch) {
		t.Fatalf("save with wrong revision: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	d, err = s.Save(ctx, settings.Doc{Values: map[string]string{"PROTEUS_TIMEOUT": "5s"}, SetupCompletedAt: now}, 0)
	if err != nil || d.Revision != 1 {
		t.Fatalf("Save = %+v, %v", d, err)
	}
	d, err = s.Load(ctx)
	if err != nil || d.Revision != 1 || d.Values["PROTEUS_TIMEOUT"] != "5s" || !d.SetupCompletedAt.Equal(now) {
		t.Fatalf("Load = %+v, %v", d, err)
	}
	if _, err := s.Save(ctx, d, 0); !errors.Is(err, settings.ErrRevisionMismatch) {
		t.Fatalf("stale save: %v", err)
	}
	d.Values = nil
	if d, err = s.Save(ctx, d, 1); err != nil || d.Revision != 2 {
		t.Fatalf("second Save = %+v, %v", d, err)
	}
	if d, _ = s.Load(ctx); len(d.Values) != 0 || !d.SetupDone() {
		t.Fatalf("after clearing values: %+v", d)
	}

	keys, err := s.ListKeys(ctx)
	if err != nil || len(keys) != 0 {
		t.Fatalf("empty ListKeys = %v, %v", keys, err)
	}
	a := settings.APIKey{ID: "k_a", Name: "Admin", Role: settings.RoleAdmin, Hash: settings.Hash("one"), Prefix: "pk_one", CreatedAt: now}
	b := settings.APIKey{ID: "k_b", Name: "App", Role: settings.RoleClient, Hash: settings.Hash("two"), Prefix: "pk_two", CreatedAt: now.Add(time.Second)}
	for _, k := range []settings.APIKey{a, b} {
		if err := s.CreateKey(ctx, k); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CreateKey(ctx, a); !errors.Is(err, settings.ErrKeyExists) {
		t.Fatalf("duplicate id: %v", err)
	}
	keys, err = s.ListKeys(ctx)
	if err != nil || len(keys) != 2 {
		t.Fatalf("ListKeys = %v, %v", keys, err)
	}
	got := keys[0]
	if got.ID != a.ID || got.Name != a.Name || got.Role != a.Role || got.Hash != a.Hash || got.Prefix != a.Prefix || !got.CreatedAt.Equal(a.CreatedAt) || keys[1].ID != b.ID {
		t.Fatalf("ListKeys = %+v", keys)
	}
	if err := s.DeleteKey(ctx, "k_a"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteKey(ctx, "k_a"); !errors.Is(err, settings.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if keys, _ = s.ListKeys(ctx); len(keys) != 1 || keys[0].ID != "k_b" {
		t.Fatalf("after delete: %+v", keys)
	}
}
