package pgstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gsoultan/storm/runtime"

	"github.com/gsoultan/proteus-server/internal/storage/store/template"
	"github.com/gsoultan/proteus-server/internal/templates"
)

// Templates implements templates.Store on the same database as the jobs.
type Templates struct{ ex runtime.Executor }

// Templates returns the template store sharing s's pool.
func (s *Store) Templates() *Templates { return &Templates{ex: s.ex} }

func nameErr(err error, what string) error {
	if errors.Is(err, runtime.ErrUniqueViolation) {
		return templates.ErrNameTaken
	}
	return fmt.Errorf("pgstore: %s template: %w", what, err)
}

// Create implements templates.Store.
func (s *Templates) Create(ctx context.Context, r *templates.Record) error {
	n := template.Create()
	n.SetOwner(r.Owner)
	n.SetName(r.Name)
	n.SetDescription(r.Description)
	n.SetFormat(r.Format)
	n.SetRevision(1)
	n.SetSizeBytes(r.SizeBytes)
	n.SetBody(r.Body)
	row, err := n.Insert(ctx, s.ex)
	if err != nil {
		return nameErr(err, "create")
	}
	r.ID, r.Revision, r.CreatedAt, r.UpdatedAt = idString(row.ID), row.Revision, row.CreatedAt, row.UpdatedAt
	return nil
}

// Get implements templates.Store.
func (s *Templates) Get(ctx context.Context, owner, id string) (*templates.Record, error) {
	key, ok := parseID(id)
	if !ok {
		return nil, templates.ErrNotFound
	}
	row, found, err := template.New().Where(template.ID.Eq(key), template.Owner.Eq(owner)).One(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: get template: %w", err)
	}
	if !found {
		return nil, templates.ErrNotFound
	}
	return &templates.Record{
		ID: idString(row.ID), Owner: row.Owner, Name: row.Name, Description: row.Description,
		Format: row.Format, Revision: row.Revision, SizeBytes: row.SizeBytes,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Body: row.Body,
	}, nil
}

func fromTemplateMeta(row template.MetaRow) *templates.Record {
	return &templates.Record{
		ID: idString(row.ID), Owner: row.Owner, Name: row.Name, Description: row.Description,
		Format: row.Format, Revision: row.Revision, SizeBytes: row.SizeBytes,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// List implements templates.Store. The (owner, name) unique index serves
// both the filter and the order.
func (s *Templates) List(ctx context.Context, owner, after string, limit int) ([]*templates.Record, error) {
	rows, err := template.New().Where(template.Owner.Eq(owner), template.Name.Gt(after)).
		Order(template.Name.Asc()).Limit(int64(limit)).AllMeta(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list templates: %w", err)
	}
	out := make([]*templates.Record, len(rows))
	for i, row := range rows {
		out[i] = fromTemplateMeta(row)
	}
	return out, nil
}

// Count implements templates.Store.
func (s *Templates) Count(ctx context.Context, owner string) (int64, error) {
	return template.New().Where(template.Owner.Eq(owner)).Count(ctx, s.ex)
}

// Update implements templates.Store. The write is conditional on the
// revision it read, so of two concurrent updates one gets
// ErrRevisionMismatch rather than silently losing the other's edit.
func (s *Templates) Update(ctx context.Context, r *templates.Record, expected int64) error {
	key, ok := parseID(r.ID)
	if !ok {
		return templates.ErrNotFound
	}
	cur, found, err := template.New().Where(template.ID.Eq(key), template.Owner.Eq(r.Owner)).OneMeta(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: read template: %w", err)
	}
	if !found {
		return templates.ErrNotFound
	}
	if expected != 0 && cur.Revision != expected {
		return templates.ErrRevisionMismatch
	}
	now := time.Now().UTC()
	var m template.Mut
	m.SetName(r.Name)
	m.SetDescription(r.Description)
	m.SetFormat(r.Format)
	m.SetSizeBytes(r.SizeBytes)
	m.SetBody(r.Body)
	m.SetRevision(cur.Revision + 1)
	m.SetUpdatedAt(now)
	n, err := template.New().Where(template.ID.Eq(key), template.Owner.Eq(r.Owner),
		template.Revision.Eq(cur.Revision)).UpdateAll(ctx, s.ex, m)
	if err != nil {
		return nameErr(err, "update")
	}
	if n == 0 {
		return templates.ErrRevisionMismatch
	}
	r.Revision, r.CreatedAt, r.UpdatedAt = cur.Revision+1, cur.CreatedAt, now
	return nil
}

// Delete implements templates.Store.
func (s *Templates) Delete(ctx context.Context, owner, id string) error {
	key, ok := parseID(id)
	if !ok {
		return templates.ErrNotFound
	}
	n, err := template.New().Where(template.ID.Eq(key), template.Owner.Eq(owner)).DeleteAll(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: delete template: %w", err)
	}
	if n == 0 {
		return templates.ErrNotFound
	}
	return nil
}

var _ templates.Store = (*Templates)(nil)
