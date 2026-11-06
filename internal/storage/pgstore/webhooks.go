// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package pgstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gsoultan/storm/runtime"

	"github.com/gsoultan/proteus-server/internal/storage/store/webhookdelivery"
	"github.com/gsoultan/proteus-server/internal/storage/store/webhookendpoint"
	"github.com/gsoultan/proteus-server/internal/webhooks"
)

// Webhooks implements webhooks.Store on the same database as the jobs. The
// delivery table is also the delivery queue: a claim is one UPDATE of the
// oldest due row chosen with FOR UPDATE SKIP LOCKED.
type Webhooks struct{ ex runtime.Executor }

// Webhooks returns the webhook store sharing s's pool.
func (s *Store) Webhooks() *Webhooks { return &Webhooks{ex: s.ex} }

func fromEndpoint(r webhookendpoint.Row) *webhooks.Endpoint {
	return &webhooks.Endpoint{
		ID: idString(r.ID), Owner: r.Owner, URL: r.URL, Description: r.Description, Events: r.Events,
		Enabled: r.Enabled, PausedAt: r.PausedAt.V, ConsecutiveFailures: int(r.ConsecutiveFailures),
		Secret: r.Secret, PreviousSecret: r.PreviousSecret, PreviousExpiresAt: r.PreviousExpiresAt.V,
		LastDeliveryAt: r.LastDeliveryAt.V, LastStatusCode: int(r.LastStatusCode),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// CreateEndpoint implements webhooks.Store.
func (s *Webhooks) CreateEndpoint(ctx context.Context, e *webhooks.Endpoint) error {
	n := webhookendpoint.Create()
	n.SetOwner(e.Owner)
	n.SetURL(e.URL)
	n.SetDescription(e.Description)
	n.SetEvents(e.Events)
	n.SetEnabled(e.Enabled)
	n.SetConsecutiveFailures(0)
	n.SetSecret(e.Secret)
	n.SetLastStatusCode(0)
	row, err := n.Insert(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: create webhook endpoint: %w", err)
	}
	e.ID, e.CreatedAt, e.UpdatedAt = idString(row.ID), row.CreatedAt, row.UpdatedAt
	return nil
}

func (s *Webhooks) endpointQuery(owner, id string) (webhookendpoint.Query, bool) {
	key, ok := parseID(id)
	if !ok {
		return webhookendpoint.Query{}, false
	}
	return webhookendpoint.New().Where(webhookendpoint.ID.Eq(key), webhookendpoint.Owner.Eq(owner)), true
}

// GetEndpoint implements webhooks.Store.
func (s *Webhooks) GetEndpoint(ctx context.Context, owner, id string) (*webhooks.Endpoint, error) {
	q, ok := s.endpointQuery(owner, id)
	if !ok {
		return nil, webhooks.ErrNotFound
	}
	row, found, err := q.One(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: get webhook endpoint: %w", err)
	}
	if !found {
		return nil, webhooks.ErrNotFound
	}
	return fromEndpoint(row), nil
}

// ListEndpoints implements webhooks.Store.
func (s *Webhooks) ListEndpoints(ctx context.Context, owner string) ([]*webhooks.Endpoint, error) {
	rows, err := webhookendpoint.New().Where(webhookendpoint.Owner.Eq(owner)).
		Order(webhookendpoint.CreatedAt.Asc(), webhookendpoint.ID.Asc()).All(ctx, s.ex, nil)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list webhook endpoints: %w", err)
	}
	out := make([]*webhooks.Endpoint, len(rows))
	for i, r := range rows {
		out[i] = fromEndpoint(r)
	}
	return out, nil
}

// CountEndpoints implements webhooks.Store.
func (s *Webhooks) CountEndpoints(ctx context.Context, owner string) (int64, error) {
	return webhookendpoint.New().Where(webhookendpoint.Owner.Eq(owner)).Count(ctx, s.ex)
}

// update applies m to owner's endpoint id, ErrNotFound when there is none.
func (s *Webhooks) update(ctx context.Context, owner, id string, m webhookendpoint.Mut, what string) error {
	q, ok := s.endpointQuery(owner, id)
	if !ok {
		return webhooks.ErrNotFound
	}
	n, err := q.UpdateAll(ctx, s.ex, m)
	if err != nil {
		return fmt.Errorf("pgstore: %s webhook endpoint: %w", what, err)
	}
	if n == 0 {
		return webhooks.ErrNotFound
	}
	return nil
}

// UpdateEndpoint implements webhooks.Store.
func (s *Webhooks) UpdateEndpoint(ctx context.Context, e *webhooks.Endpoint) error {
	var m webhookendpoint.Mut
	m.SetURL(e.URL)
	m.SetDescription(e.Description)
	m.SetEvents(e.Events)
	m.SetUpdatedAt(time.Now().UTC())
	return s.update(ctx, e.Owner, e.ID, m, "update")
}

// SetEnabled implements webhooks.Store.
func (s *Webhooks) SetEnabled(ctx context.Context, owner, id string, enabled bool, now time.Time) error {
	var m webhookendpoint.Mut
	m.SetEnabled(enabled)
	m.SetUpdatedAt(now)
	if enabled {
		m.SetPausedAtNull()
		m.SetConsecutiveFailures(0)
	}
	return s.update(ctx, owner, id, m, "enable")
}

// SetSecrets implements webhooks.Store.
func (s *Webhooks) SetSecrets(ctx context.Context, owner, id string, secret, previous []byte, previousExpires time.Time) error {
	var m webhookendpoint.Mut
	m.SetSecret(secret)
	m.SetPreviousSecret(previous)
	if previousExpires.IsZero() {
		m.SetPreviousExpiresAtNull()
	} else {
		m.SetPreviousExpiresAt(previousExpires)
	}
	m.SetUpdatedAt(time.Now().UTC())
	return s.update(ctx, owner, id, m, "rotate")
}

// RecordResult implements webhooks.Store. The failure count is incremented
// in SQL, and pausing is a second UPDATE conditional on the count and on
// the endpoint still being enabled, so concurrent workers pause it once.
func (s *Webhooks) RecordResult(ctx context.Context, owner, id string, r webhooks.Result, pauseAfter int) (bool, error) {
	var m webhookendpoint.Mut
	m.SetLastDeliveryAt(r.At)
	m.SetLastStatusCode(int32(r.StatusCode))
	if r.OK {
		m.SetConsecutiveFailures(0)
	} else {
		m.IncConsecutiveFailures()
	}
	if err := s.update(ctx, owner, id, m, "record result for"); err != nil || r.OK || pauseAfter <= 0 {
		return false, err
	}
	q, _ := s.endpointQuery(owner, id)
	var p webhookendpoint.Mut
	p.SetEnabled(false)
	p.SetPausedAt(r.At)
	p.SetUpdatedAt(r.At)
	n, err := q.Where(webhookendpoint.Enabled.Eq(true),
		webhookendpoint.ConsecutiveFailures.Gte(int32(pauseAfter))).UpdateAll(ctx, s.ex, p)
	if err != nil {
		return false, fmt.Errorf("pgstore: pause webhook endpoint: %w", err)
	}
	return n > 0, nil
}

// DeleteEndpoint implements webhooks.Store. Deliveries go first so a
// worker never finds a delivery of an endpoint that is gone for good.
func (s *Webhooks) DeleteEndpoint(ctx context.Context, owner, id string) error {
	q, ok := s.endpointQuery(owner, id)
	if !ok {
		return webhooks.ErrNotFound
	}
	key, _ := parseID(id)
	if _, err := webhookdelivery.New().Where(webhookdelivery.EndpointID.Eq(key),
		webhookdelivery.Owner.Eq(owner)).DeleteAll(ctx, s.ex); err != nil {
		return fmt.Errorf("pgstore: delete webhook deliveries: %w", err)
	}
	n, err := q.DeleteAll(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: delete webhook endpoint: %w", err)
	}
	if n == 0 {
		return webhooks.ErrNotFound
	}
	return nil
}

const deliveryPending = string(webhooks.Pending)

func nullID(id runtime.Null[[16]byte]) string {
	if !id.Valid {
		return ""
	}
	return idString(id.V)
}

func fromDelivery(r webhookdelivery.Row) *webhooks.Delivery {
	return &webhooks.Delivery{
		ID: idString(r.ID), Owner: r.Owner, EndpointID: idString(r.EndpointID), EventID: idString(r.EventID),
		EventType: r.EventType, SubjectID: r.SubjectID, Status: webhooks.DeliveryStatus(r.Status), Payload: r.Payload,
		Attempts: int(r.Attempts), MaxAttempts: int(r.MaxAttempts), StatusCode: int(r.StatusCode), LatencyMS: r.LatencyMs,
		Error: r.Error.V, LastAttemptAt: r.LastAttemptAt.V, NextAttemptAt: r.NextAttemptAt.V,
		RedeliveryOf: nullID(r.RedeliveryOf), Manual: r.Manual, CreatedAt: r.CreatedAt,
	}
}

func fromDeliveryMeta(r webhookdelivery.MetaRow) *webhooks.Delivery {
	return &webhooks.Delivery{
		ID: idString(r.ID), Owner: r.Owner, EndpointID: idString(r.EndpointID), EventID: idString(r.EventID),
		EventType: r.EventType, SubjectID: r.SubjectID, Status: webhooks.DeliveryStatus(r.Status),
		Attempts: int(r.Attempts), MaxAttempts: int(r.MaxAttempts), StatusCode: int(r.StatusCode), LatencyMS: r.LatencyMs,
		Error: r.Error.V, LastAttemptAt: r.LastAttemptAt.V, NextAttemptAt: r.NextAttemptAt.V,
		RedeliveryOf: nullID(r.RedeliveryOf), Manual: r.Manual, CreatedAt: r.CreatedAt,
	}
}

// CreateDelivery implements webhooks.Store.
func (s *Webhooks) CreateDelivery(ctx context.Context, d *webhooks.Delivery) error {
	endpoint, ok := parseID(d.EndpointID)
	if !ok {
		return webhooks.ErrNotFound
	}
	event, ok := parseID(d.EventID)
	if !ok {
		return fmt.Errorf("pgstore: create webhook delivery: bad event id %q", d.EventID)
	}
	n := webhookdelivery.Create()
	if d.ID != "" {
		key, ok := parseID(d.ID)
		if !ok {
			return fmt.Errorf("pgstore: create webhook delivery: bad id %q", d.ID)
		}
		n.SetID(key)
	}
	n.SetOwner(d.Owner)
	n.SetEndpointID(endpoint)
	n.SetEventID(event)
	n.SetEventType(d.EventType)
	n.SetSubjectID(d.SubjectID)
	n.SetStatus(string(d.Status))
	n.SetPayload(d.Payload)
	n.SetAttempts(int32(d.Attempts))
	n.SetMaxAttempts(int32(d.MaxAttempts))
	n.SetStatusCode(int32(d.StatusCode))
	n.SetLatencyMs(d.LatencyMS)
	n.SetManual(d.Manual)
	if d.Error != "" {
		n.SetError(d.Error)
	}
	if !d.LastAttemptAt.IsZero() {
		n.SetLastAttemptAt(d.LastAttemptAt)
	}
	if !d.NextAttemptAt.IsZero() {
		n.SetNextAttemptAt(d.NextAttemptAt)
	}
	if d.RedeliveryOf != "" {
		if orig, ok := parseID(d.RedeliveryOf); ok {
			n.SetRedeliveryOf(orig)
		}
	}
	row, err := n.Insert(ctx, s.ex)
	if err != nil {
		return fmt.Errorf("pgstore: create webhook delivery: %w", err)
	}
	d.ID, d.CreatedAt = idString(row.ID), row.CreatedAt
	return nil
}

// GetDelivery implements webhooks.Store.
func (s *Webhooks) GetDelivery(ctx context.Context, owner, id string) (*webhooks.Delivery, error) {
	key, ok := parseID(id)
	if !ok {
		return nil, webhooks.ErrNotFound
	}
	row, found, err := webhookdelivery.New().Where(webhookdelivery.ID.Eq(key), webhookdelivery.Owner.Eq(owner)).One(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: get webhook delivery: %w", err)
	}
	if !found {
		return nil, webhooks.ErrNotFound
	}
	return fromDelivery(row), nil
}

// ListDeliveries implements webhooks.Store. The (endpoint_id, created_at)
// index serves the filter and the order; the cursor is a keyset position.
func (s *Webhooks) ListDeliveries(ctx context.Context, owner, endpointID string, status webhooks.DeliveryStatus,
	after *webhooks.Cursor, limit int) ([]*webhooks.Delivery, error) {
	endpoint, ok := parseID(endpointID)
	if !ok {
		return nil, webhooks.ErrNotFound
	}
	q := webhookdelivery.New().Where(webhookdelivery.EndpointID.Eq(endpoint), webhookdelivery.Owner.Eq(owner))
	if status != "" {
		q = q.Where(webhookdelivery.Status.Eq(string(status)))
	}
	q = q.Order(webhookdelivery.CreatedAt.Desc(), webhookdelivery.ID.Desc())
	if after != nil {
		key, ok := parseID(after.ID)
		if !ok {
			return nil, nil
		}
		q = q.After(webhookdelivery.Row{ID: key, CreatedAt: after.CreatedAt})
	}
	rows, err := q.Limit(int64(limit)).AllMeta(ctx, s.ex)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list webhook deliveries: %w", err)
	}
	out := make([]*webhooks.Delivery, len(rows))
	for i, r := range rows {
		out[i] = fromDeliveryMeta(r)
	}
	return out, nil
}

// ClaimDelivery implements webhooks.Store.
func (s *Webhooks) ClaimDelivery(ctx context.Context, now time.Time, lease time.Duration) (*webhooks.Delivery, error) {
	var m webhookdelivery.Mut
	m.SetNextAttemptAt(now.Add(lease))
	m.IncAttempts()
	got, err := webhookdelivery.New().
		Where(webhookdelivery.Status.Eq(deliveryPending), webhookdelivery.NextAttemptAt.Lte(now)).
		Order(webhookdelivery.NextAttemptAt.Asc()).
		Limit(1).
		ForUpdateSkipLocked().
		Take(ctx, s.ex, m, nil)
	if err != nil {
		return nil, fmt.Errorf("pgstore: claim webhook delivery: %w", err)
	}
	if len(got) == 0 {
		return nil, nil
	}
	return fromDelivery(got[0]), nil
}

// FinishAttempt implements webhooks.Store.
func (s *Webhooks) FinishAttempt(ctx context.Context, d *webhooks.Delivery) error {
	key, ok := parseID(d.ID)
	if !ok {
		return webhooks.ErrNotFound
	}
	m := webhookdelivery.MutateKey(key)
	m.SetStatus(string(d.Status))
	m.SetStatusCode(int32(d.StatusCode))
	m.SetLatencyMs(d.LatencyMS)
	if d.Error != "" {
		m.SetError(d.Error)
	} else {
		m.SetErrorNull()
	}
	if d.LastAttemptAt.IsZero() {
		m.SetLastAttemptAtNull()
	} else {
		m.SetLastAttemptAt(d.LastAttemptAt)
	}
	if d.NextAttemptAt.IsZero() {
		m.SetNextAttemptAtNull()
	} else {
		m.SetNextAttemptAt(d.NextAttemptAt)
	}
	m.SetUpdatedAt(time.Now().UTC())
	if err := m.Update(ctx, s.ex); err != nil {
		return fmt.Errorf("pgstore: finish webhook delivery: %w", err)
	}
	return nil
}

// SweepDeliveries implements webhooks.Store, deleting in batches.
func (s *Webhooks) SweepDeliveries(ctx context.Context, t time.Time) (int64, error) {
	var total int64
	for {
		n, err := webhookdelivery.New().Where(webhookdelivery.CreatedAt.Lt(t)).Limit(500).DeleteBatch(ctx, s.ex)
		total += n
		if err != nil {
			return total, fmt.Errorf("pgstore: sweep webhook deliveries: %w", err)
		}
		if n < 500 {
			return total, nil
		}
	}
}

var _ webhooks.Store = (*Webhooks)(nil)
