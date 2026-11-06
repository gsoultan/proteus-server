// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package webhooks

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound is returned for unknown or someone else's endpoints and deliveries.
	ErrNotFound = errors.New("webhook endpoint or delivery not found")
	// ErrLimit is returned when an API key already has the most endpoints allowed.
	ErrLimit = errors.New("too many webhook endpoints for this API key")
)

// DeliveryStatus is where a delivery stands.
type DeliveryStatus string

const (
	Pending   DeliveryStatus = "pending"
	Succeeded DeliveryStatus = "succeeded"
	Failed    DeliveryStatus = "failed"
)

// Endpoint is a saved webhook receiver as a Store holds it. Secret and
// PreviousSecret are sealed (see Sealer); the store treats them as opaque.
type Endpoint struct {
	ID                  string
	Owner               string // API key id
	URL                 string
	Description         string
	Events              []string // event type names
	Enabled             bool
	PausedAt            time.Time
	ConsecutiveFailures int
	Secret              []byte
	PreviousSecret      []byte
	PreviousExpiresAt   time.Time
	LastDeliveryAt      time.Time
	LastStatusCode      int
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Delivery is one event sent to one endpoint.
type Delivery struct {
	ID            string
	Owner         string
	EndpointID    string
	EventID       string
	EventType     string
	SubjectID     string
	Status        DeliveryStatus
	Payload       []byte // JSON body; only set by GetDelivery and ClaimDelivery
	Attempts      int
	MaxAttempts   int
	StatusCode    int
	LatencyMS     int64
	Error         string
	LastAttemptAt time.Time
	NextAttemptAt time.Time
	RedeliveryOf  string
	Manual        bool
	CreatedAt     time.Time
}

// Cursor is a position in a newest-first delivery listing.
type Cursor struct {
	CreatedAt time.Time
	ID        string
}

// Result is how a finished delivery went, for the endpoint's health.
type Result struct {
	OK         bool
	StatusCode int
	At         time.Time
}

// Store persists endpoints and deliveries. The memory store serves one
// instance; the Postgres store (internal/storage/pgstore) shares the queue
// between instances. Implementations must be safe for concurrent use.
type Store interface {
	// CreateEndpoint inserts e and fills in ID, CreatedAt and UpdatedAt.
	CreateEndpoint(ctx context.Context, e *Endpoint) error
	// GetEndpoint returns owner's endpoint; ErrNotFound otherwise.
	GetEndpoint(ctx context.Context, owner, id string) (*Endpoint, error)
	// ListEndpoints returns owner's endpoints, oldest first.
	ListEndpoints(ctx context.Context, owner string) ([]*Endpoint, error)
	// CountEndpoints returns how many endpoints owner has.
	CountEndpoints(ctx context.Context, owner string) (int64, error)
	// UpdateEndpoint writes e's URL, Description and Events.
	UpdateEndpoint(ctx context.Context, e *Endpoint) error
	// SetEnabled turns an endpoint on or off. Enabling also clears PausedAt
	// and ConsecutiveFailures.
	SetEnabled(ctx context.Context, owner, id string, enabled bool, now time.Time) error
	// SetSecrets replaces the sealed secrets.
	SetSecrets(ctx context.Context, owner, id string, secret, previous []byte, previousExpires time.Time) error
	// RecordResult updates the endpoint's health after a finished delivery:
	// success resets ConsecutiveFailures, failure increments it, and an
	// enabled endpoint that reaches pauseAfter failures is disabled with
	// PausedAt = r.At. It reports whether this call paused the endpoint.
	RecordResult(ctx context.Context, owner, id string, r Result, pauseAfter int) (bool, error)
	// DeleteEndpoint removes owner's endpoint and its deliveries.
	DeleteEndpoint(ctx context.Context, owner, id string) error

	// CreateDelivery inserts d (any status) and fills in CreatedAt, and ID
	// unless it is already set.
	CreateDelivery(ctx context.Context, d *Delivery) error
	// GetDelivery returns owner's delivery with its payload.
	GetDelivery(ctx context.Context, owner, id string) (*Delivery, error)
	// ListDeliveries returns an endpoint's deliveries without payload, newest
	// first, after the cursor when it is set, in status when it is set.
	ListDeliveries(ctx context.Context, owner, endpointID string, status DeliveryStatus, after *Cursor, limit int) ([]*Delivery, error)
	// ClaimDelivery takes the oldest pending delivery whose NextAttemptAt
	// is not after now, increments Attempts, moves NextAttemptAt to
	// now+lease (so a dead worker's delivery comes back) and returns it
	// with its payload; nil when there is none.
	ClaimDelivery(ctx context.Context, now time.Time, lease time.Duration) (*Delivery, error)
	// FinishAttempt writes a claimed delivery's Status, StatusCode,
	// LatencyMS, Error, LastAttemptAt and NextAttemptAt.
	FinishAttempt(ctx context.Context, d *Delivery) error
	// SweepDeliveries deletes deliveries created before t.
	SweepDeliveries(ctx context.Context, t time.Time) (int64, error)
}
