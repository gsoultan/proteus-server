// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package model

import (
	"time"

	"github.com/gsoultan/storm"
)

// WebhookEndpoint is a saved webhook receiver of one API key. Secret and
// PreviousSecret hold the sealed signing secrets (see internal/webhooks).
type WebhookEndpoint struct {
	storm.Model // uuid id + created_at/updated_at

	Owner       string // API key id that created the endpoint
	URL         string
	Description string
	Events      []string // job.queued, job.succeeded, job.failed, template.updated

	Enabled             bool
	PausedAt            *time.Time // set when the server paused it after failures
	ConsecutiveFailures int32

	Secret            []byte
	PreviousSecret    []byte
	PreviousExpiresAt *time.Time

	LastDeliveryAt *time.Time
	LastStatusCode int32
}

func (e *WebhookEndpoint) Schema(t *storm.Table) {
	t.Col(&e.Owner).Size(128)
	t.Col(&e.URL).Size(2048)
	t.Col(&e.Description).Size(500)
	t.Col(&e.Enabled).Default("true")
	t.Col(&e.ConsecutiveFailures).Default("0")
	t.Col(&e.LastStatusCode).Default("0")
	t.Col(&e.PreviousSecret).Nullable()
	// Listing is per owner, oldest first.
	t.Index(&e.Owner, &e.CreatedAt)
}

// WebhookDelivery is one event sent to one endpoint: the delivery log and,
// while pending, the delivery queue. NextAttemptAt doubles as the claim
// lease, so a delivery whose worker died is tried again once it passes.
type WebhookDelivery struct {
	storm.Model // uuid id + created_at/updated_at

	Owner      string // API key id of the endpoint
	EndpointID storm.UUID
	EventID    storm.UUID
	EventType  string
	SubjectID  string // job or template id; empty for test events
	Status     string // pending, succeeded, failed
	Payload    []byte // the JSON body

	Attempts      int32
	MaxAttempts   int32
	StatusCode    int32
	LatencyMs     int64
	Error         *string
	LastAttemptAt *time.Time
	NextAttemptAt *time.Time

	RedeliveryOf *storm.UUID
	Manual       bool
}

func (d *WebhookDelivery) Schema(t *storm.Table) {
	t.Col(&d.Owner).Size(128)
	t.Col(&d.EventType).Size(32)
	t.Col(&d.SubjectID).Size(64)
	t.Col(&d.Attempts).Default("0")
	t.Col(&d.StatusCode).Default("0")
	t.Col(&d.LatencyMs).Default("0")
	t.Col(&d.Manual).Default("false")
	t.Check(storm.RawSQL(`status IN ('pending', 'succeeded', 'failed')`))
	// The log lists an endpoint's deliveries newest first.
	t.Index(&d.EndpointID, &d.CreatedAt)
	// The dispatcher claims due pending deliveries.
	t.Index(&d.Status, &d.NextAttemptAt)
	// The sweeper drops deliveries past retention.
	t.Index(&d.CreatedAt)
	t.Takes()
}

// Projections: Meta is every column but the payload.
func (d *WebhookDelivery) Projections(p *storm.Projections) {
	p.Named("Meta", &d.ID, &d.Owner, &d.EndpointID, &d.EventID, &d.EventType, &d.SubjectID, &d.Status,
		&d.Attempts, &d.MaxAttempts, &d.StatusCode, &d.LatencyMs, &d.Error, &d.LastAttemptAt, &d.NextAttemptAt,
		&d.RedeliveryOf, &d.Manual, &d.CreatedAt, &d.UpdatedAt)
}
