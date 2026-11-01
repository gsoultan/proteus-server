// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package pgstore is the Postgres job store, built on storm. Every server
// instance can share one database: a claim is a single UPDATE of the oldest
// claimable row chosen with FOR UPDATE SKIP LOCKED, so two workers never take
// the same job and neither waits for the other.
package pgstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gsoultan/storm"
	"github.com/gsoultan/storm/migrate"
	"github.com/gsoultan/storm/runtime"
	"github.com/gsoultan/storm/runtime/pgxdrv"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/storage/model"
	"github.com/gsoultan/proteus-server/internal/storage/store/job"
)

// Store implements jobs.Store on PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
	ex   runtime.Executor
}

// Open connects to dsn and uses the jobs table in schema (default
// "public"). With autoMigrate set it creates or updates the table (storm's
// automigrate: advisory lock, one transaction, destructive steps refused);
// otherwise the schema must already match.
func Open(ctx context.Context, dsn, schema string, autoMigrate bool, logf func(string, ...any)) (*Store, error) {
	if schema == "" {
		schema = "public"
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("pgstore: parse database url: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxdrv.NewPoolConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pgstore: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgstore: ping: %w", err)
	}
	if autoMigrate {
		if err := Migrate(ctx, pool, schema, logf); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return New(pool), nil
}

// Migrate brings the jobs table in schema up to date with the model.
func Migrate(ctx context.Context, pool *pgxpool.Pool, schema string, logf func(string, ...any)) error {
	s, err := storm.Build(&model.Job{})
	if err != nil {
		return fmt.Errorf("pgstore: build schema: %w", err)
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if _, err := migrate.AutoPool(ctx, pool, s, migrate.AutoOptions{Schema: schema, Logf: logf, DeclaredOnly: true}); err != nil {
		return fmt.Errorf("pgstore: migrate: %w", err)
	}
	return nil
}

// New wraps an existing pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool, ex: pgxdrv.Pool{P: pool}} }

// Close closes the pool.
func (s *Store) Close() { s.pool.Close() }

// Ping checks the database is reachable (for readiness).
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func parseID(id string) ([16]byte, bool) {
	u, err := uuid.Parse(id)
	if err != nil {
		return [16]byte{}, false
	}
	return u, true
}

func idString(b [16]byte) string { return uuid.UUID(b).String() }

const (
	statusQueued    = string(jobs.Queued)
	statusRunning   = string(jobs.Running)
	statusSucceeded = string(jobs.Succeeded)
	statusFailed    = string(jobs.Failed)
)

// Create implements jobs.Store.
func (s *Store) Create(ctx context.Context, r *jobs.Record) error {
	n := job.Create()
	n.SetOwner(r.Owner)
	n.SetStatus(statusQueued)
	n.SetInputFormat(r.InputFormat)
	n.SetOutputFormat(r.OutputFormat)
	n.SetFileName(r.FileName)
	n.SetPayload(r.Payload)
	if r.WebhookURL != "" {
		n.SetWebhookURL(r.WebhookURL)
	}
	row, err := n.Insert(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: create job: %w", err)
	}
	r.ID, r.CreatedAt, r.Status = idString(row.ID), row.CreatedAt, jobs.Queued
	return nil
}

// CountQueued implements jobs.Store.
func (s *Store) CountQueued(ctx context.Context) (int64, error) {
	return job.New().Where(job.Status.Eq(statusQueued)).Count(ctx, s.ex)
}

// Claim implements jobs.Store.
func (s *Store) Claim(ctx context.Context, now time.Time, lease time.Duration, maxAttempts int) (*jobs.Record, error) {
	// Jobs whose worker died too often fail instead of looping forever.
	var dead job.Mut
	dead.SetStatus(statusFailed)
	dead.SetError("conversion was interrupted too many times")
	dead.SetFinishedAt(now)
	dead.SetLeaseExpiresNull()
	dead.SetPayload(nil)
	if _, err := job.New().Where(job.Status.Eq(statusRunning), job.LeaseExpires.Lt(now),
		job.Attempts.Gte(int32(maxAttempts))).UpdateAll(ctx, s.ex, dead); err != nil {
		return nil, fmt.Errorf("pgstore: fail lapsed jobs: %w", err)
	}

	var m job.Mut
	m.SetStatus(statusRunning)
	m.SetLeaseExpires(now.Add(lease))
	m.SetStartedAt(now)
	m.IncAttempts()
	got, err := job.New().
		AnyOf(
			job.And(job.Status.Eq(statusQueued)),
			job.And(job.Status.Eq(statusRunning), job.LeaseExpires.Lt(now)),
		).
		Order(job.CreatedAt.Asc()).
		Limit(1).
		ForUpdateSkipLocked().
		Take(ctx, s.ex, m, nil)
	if err != nil {
		return nil, fmt.Errorf("pgstore: claim: %w", err)
	}
	if len(got) == 0 {
		return nil, nil
	}
	r := fromRow(got[0])
	r.Payload = got[0].Payload
	return r, nil
}

// Finish implements jobs.Store.
func (s *Store) Finish(ctx context.Context, id string, o jobs.Outcome) error {
	key, ok := parseID(id)
	if !ok {
		return jobs.ErrNotFound
	}
	m := job.MutateKey(key)
	m.SetStatus(string(o.Status))
	m.SetFinishedAt(o.FinishedAt)
	m.SetExpiresAt(o.ExpiresAt)
	m.SetLeaseExpiresNull()
	m.SetPayload(nil)
	if o.Error != "" {
		m.SetError(o.Error)
	}
	if len(o.Warnings) > 0 {
		m.SetWarnings(o.Warnings)
	}
	if o.Status == jobs.Succeeded {
		m.SetResult(o.Result)
		m.SetResultBytes(int64(len(o.Result)))
	}
	if err := m.Update(ctx, s.ex); err != nil {
		return fmt.Errorf("pgstore: finish job: %w", err)
	}
	return nil
}

// Release implements jobs.Store.
func (s *Store) Release(ctx context.Context, id string) error {
	key, ok := parseID(id)
	if !ok {
		return jobs.ErrNotFound
	}
	var m job.Mut
	m.SetStatus(statusQueued)
	m.SetLeaseExpiresNull()
	m.SetStartedAtNull()
	_, err := job.New().Where(job.ID.Eq(key), job.Status.Eq(statusRunning)).UpdateAll(ctx, s.ex, m)
	return err
}

// Get implements jobs.Store.
func (s *Store) Get(ctx context.Context, owner, id string) (*jobs.Record, error) {
	key, ok := parseID(id)
	if !ok {
		return nil, jobs.ErrNotFound
	}
	row, found, err := job.New().Where(job.ID.Eq(key), job.Owner.Eq(owner)).OneMeta(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: get job: %w", err)
	}
	if !found {
		return nil, jobs.ErrNotFound
	}
	return fromMeta(row), nil
}

// Result implements jobs.Store.
func (s *Store) Result(ctx context.Context, owner, id string) ([]byte, error) {
	key, ok := parseID(id)
	if !ok {
		return nil, jobs.ErrNotFound
	}
	row, found, err := job.New().Where(job.ID.Eq(key), job.Owner.Eq(owner), job.Status.Eq(statusSucceeded)).One(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: read result: %w", err)
	}
	if !found {
		return nil, jobs.ErrNotFound
	}
	return row.Result, nil
}

// Delete implements jobs.Store.
func (s *Store) Delete(ctx context.Context, owner, id string) error {
	key, ok := parseID(id)
	if !ok {
		return jobs.ErrNotFound
	}
	n, err := job.New().Where(job.ID.Eq(key), job.Owner.Eq(owner),
		job.Status.In(statusSucceeded, statusFailed)).DeleteAll(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: delete job: %w", err)
	}
	if n == 0 {
		return jobs.ErrNotFound
	}
	return nil
}

// Sweep implements jobs.Store, deleting in batches so no statement holds
// locks over the whole table.
func (s *Store) Sweep(ctx context.Context, now time.Time) (int64, error) {
	var total int64
	for {
		n, err := job.New().Where(job.ExpiresAt.Lt(now)).Limit(500).DeleteBatch(ctx, s.ex)
		total += n
		if err != nil {
			return total, fmt.Errorf("pgstore: sweep: %w", err)
		}
		if n < 500 {
			return total, nil
		}
	}
}

func fromRow(r job.Row) *jobs.Record {
	return &jobs.Record{
		ID: idString(r.ID), Owner: r.Owner, Status: jobs.Status(r.Status),
		InputFormat: r.InputFormat, OutputFormat: r.OutputFormat, FileName: r.FileName,
		WebhookURL: r.WebhookURL.V, Attempts: int(r.Attempts), CreatedAt: r.CreatedAt,
		StartedAt: r.StartedAt.V, FinishedAt: r.FinishedAt.V, ExpiresAt: r.ExpiresAt.V,
		Error: r.Error.V, Warnings: r.Warnings, ResultBytes: r.ResultBytes,
	}
}

func fromMeta(r job.MetaRow) *jobs.Record {
	return &jobs.Record{
		ID: idString(r.ID), Owner: r.Owner, Status: jobs.Status(r.Status),
		InputFormat: r.InputFormat, OutputFormat: r.OutputFormat, FileName: r.FileName,
		WebhookURL: r.WebhookURL.V, Attempts: int(r.Attempts), CreatedAt: r.CreatedAt,
		StartedAt: r.StartedAt.V, FinishedAt: r.FinishedAt.V, ExpiresAt: r.ExpiresAt.V,
		Error: r.Error.V, Warnings: r.Warnings, ResultBytes: r.ResultBytes,
	}
}

var _ jobs.Store = (*Store)(nil)
