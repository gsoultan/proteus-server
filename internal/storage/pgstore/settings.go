// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package pgstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gsoultan/storm/runtime"

	"github.com/gsoultan/proteus-server/internal/settings"
	"github.com/gsoultan/proteus-server/internal/storage/store/apikey"
	"github.com/gsoultan/proteus-server/internal/storage/store/serversetting"
)

// Settings implements settings.Store on the same database, so every
// instance sharing it sees the same settings and keys.
type Settings struct{ ex runtime.Executor }

// Settings returns the settings store sharing s's pool.
func (s *Store) Settings() *Settings { return &Settings{ex: s.ex} }

const scopeServer = "server"

// Load implements settings.Store.
func (s *Settings) Load(ctx context.Context) (settings.Doc, error) {
	row, found, err := serversetting.New().Where(serversetting.Scope.Eq(scopeServer)).One(ctx, s.ex)
	if err != nil {
		return settings.Doc{}, fmt.Errorf("pgstore: load settings: %w", err)
	}
	if !found {
		return settings.Doc{}, nil
	}
	var d settings.Doc
	if err := json.Unmarshal(row.Body, &d); err != nil {
		return settings.Doc{}, fmt.Errorf("pgstore: settings body: %w", err)
	}
	d.Revision = row.Revision
	return d, nil
}

// Save implements settings.Store. The first save inserts the row (a second
// concurrent first save hits the unique scope); later saves are an UPDATE
// conditional on the revision read.
func (s *Settings) Save(ctx context.Context, d settings.Doc, expected int64) (settings.Doc, error) {
	d.Revision = expected + 1
	body, err := json.Marshal(d)
	if err != nil {
		return settings.Doc{}, err
	}
	if expected == 0 {
		n := serversetting.Create()
		n.SetScope(scopeServer)
		n.SetRevision(1)
		n.SetBody(body)
		if _, err := n.Insert(ctx, s.ex); err != nil {
			if errors.Is(err, runtime.ErrUniqueViolation) {
				return settings.Doc{}, settings.ErrRevisionMismatch
			}
			return settings.Doc{}, fmt.Errorf("pgstore: save settings: %w", err)
		}
		return d, nil
	}
	var m serversetting.Mut
	m.SetRevision(d.Revision)
	m.SetBody(body)
	m.SetUpdatedAt(time.Now().UTC())
	n, err := serversetting.New().Where(serversetting.Scope.Eq(scopeServer), serversetting.Revision.Eq(expected)).UpdateAll(ctx, s.ex, m)
	if err != nil {
		return settings.Doc{}, fmt.Errorf("pgstore: save settings: %w", err)
	}
	if n == 0 {
		return settings.Doc{}, settings.ErrRevisionMismatch
	}
	return d, nil
}

// ListKeys implements settings.Store.
func (s *Settings) ListKeys(ctx context.Context) ([]settings.APIKey, error) {
	rows, err := apikey.New().Order(apikey.CreatedAt.Asc(), apikey.KeyID.Asc()).All(ctx, s.ex, nil)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list api keys: %w", err)
	}
	out := make([]settings.APIKey, 0, len(rows))
	for _, r := range rows {
		if len(r.Hash) != 32 {
			return nil, fmt.Errorf("pgstore: api key %q: bad hash", r.KeyID)
		}
		k := settings.APIKey{ID: r.KeyID, Name: r.Name, Role: r.Role, Prefix: r.Prefix, CreatedAt: r.CreatedAt}
		copy(k.Hash[:], r.Hash)
		out = append(out, k)
	}
	return out, nil
}

// CreateKey implements settings.Store.
func (s *Settings) CreateKey(ctx context.Context, k settings.APIKey) error {
	n := apikey.Create()
	n.SetKeyID(k.ID)
	n.SetName(k.Name)
	n.SetRole(k.Role)
	n.SetHash(k.Hash[:])
	n.SetPrefix(k.Prefix)
	if !k.CreatedAt.IsZero() {
		n.SetCreatedAt(k.CreatedAt)
	}
	if _, err := n.Insert(ctx, s.ex); err != nil {
		if errors.Is(err, runtime.ErrUniqueViolation) {
			return settings.ErrKeyExists
		}
		return fmt.Errorf("pgstore: create api key: %w", err)
	}
	return nil
}

// DeleteKey implements settings.Store.
func (s *Settings) DeleteKey(ctx context.Context, id string) error {
	n, err := apikey.New().Where(apikey.KeyID.Eq(id)).DeleteAll(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: delete api key: %w", err)
	}
	if n == 0 {
		return settings.ErrNotFound
	}
	return nil
}

var _ settings.Store = (*Settings)(nil)
