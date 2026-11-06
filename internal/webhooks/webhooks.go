// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package webhooks delivers events to the webhook endpoints an API key has
// saved. Publishing an event writes one pending delivery per subscribed,
// enabled endpoint; dispatcher workers claim due deliveries from the store
// (so with the Postgres store every instance shares the queue), POST them
// signed with the endpoint's secrets, retry with backoff, log every
// outcome and pause endpoints that keep failing.
//
// The per-job webhook_url of SubmitJob is separate (internal/jobs) and
// keeps its own payload and server-wide secret.
package webhooks

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// Event type names, as the payload and the store carry them.
const (
	JobQueued       = "job.queued"
	JobSucceeded    = "job.succeeded"
	JobFailed       = "job.failed"
	TemplateUpdated = "template.updated"
	Test            = "test"
)

var eventNames = map[proteusv1.WebhookEventType]string{
	proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_QUEUED:       JobQueued,
	proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_SUCCEEDED:    JobSucceeded,
	proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_FAILED:       JobFailed,
	proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_TEMPLATE_UPDATED: TemplateUpdated,
	proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_TEST:             Test,
}

// EventName returns the payload name of t ("" for UNSPECIFIED).
func EventName(t proteusv1.WebhookEventType) string { return eventNames[t] }

// EventType returns the API enum for a payload name.
func EventType(name string) proteusv1.WebhookEventType {
	for k, v := range eventNames {
		if v == name {
			return k
		}
	}
	return proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_UNSPECIFIED
}

// Observer receives delivery telemetry.
type Observer interface {
	WebhookDone(outcome string)
}

type nopObserver struct{}

func (nopObserver) WebhookDone(string) {}

// Settings configure a Service.
type Settings struct {
	Client       *safefetch.Client // sends deliveries; required
	AllowHTTP    bool              // accept http:// endpoint URLs (tests only)
	Sealer       *Sealer           // protects stored secrets; default stores them as they are
	MaxEndpoints int               // per API key; default 20
	MaxAttempts  int               // per delivery; default 3
	PauseAfter   int               // consecutive failed deliveries before pausing; default 20
	Retention    time.Duration     // how long deliveries are logged; default 7 days
	SecretGrace  time.Duration     // how long a rotated-out secret still signs; default 24h
	RetryBackoff time.Duration     // wait before the second attempt, x6 for each later one; default 10s
	Timeout      time.Duration     // per attempt; default 10s
	Workers      int               // concurrent deliveries; default 2
	PollInterval time.Duration     // how often idle workers look for work; default 1s
	Logger       *slog.Logger
	Observer     Observer
}

// Service manages endpoints and runs the dispatcher.
type Service struct {
	s     Settings
	store Store

	wake    chan struct{}
	stopCtx context.Context
	stop    context.CancelFunc
	wg      sync.WaitGroup
	once    sync.Once
}

// New returns a Service; call Start to run the dispatcher.
func New(store Store, s Settings) *Service {
	if s.Sealer == nil {
		s.Sealer = &Sealer{}
	}
	if s.MaxEndpoints <= 0 {
		s.MaxEndpoints = 20
	}
	if s.MaxAttempts <= 0 {
		s.MaxAttempts = 3
	}
	if s.PauseAfter <= 0 {
		s.PauseAfter = 20
	}
	if s.Retention <= 0 {
		s.Retention = 7 * 24 * time.Hour
	}
	if s.SecretGrace <= 0 {
		s.SecretGrace = 24 * time.Hour
	}
	if s.RetryBackoff <= 0 {
		s.RetryBackoff = 10 * time.Second
	}
	if s.Timeout <= 0 {
		s.Timeout = 10 * time.Second
	}
	if s.Workers <= 0 {
		s.Workers = 2
	}
	if s.PollInterval <= 0 {
		s.PollInterval = time.Second
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Observer == nil {
		s.Observer = nopObserver{}
	}
	svc := &Service{s: s, store: store, wake: make(chan struct{}, s.Workers)}
	svc.stopCtx, svc.stop = context.WithCancel(context.Background())
	return svc
}

// --- endpoints ---

func badRequest(format string, a ...any) error {
	return &convert.BadRequestError{Msg: fmt.Sprintf(format, a...)}
}

func (s *Service) checkURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", badRequest("url is required")
	}
	if len(raw) > 2048 {
		return "", badRequest("url is longer than 2048 characters")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", badRequest("url is not a valid absolute URL")
	}
	if u.Scheme != "https" && !(s.s.AllowHTTP && u.Scheme == "http") {
		return "", badRequest("url must use https")
	}
	if u.Fragment != "" {
		return "", badRequest("url must not have a fragment")
	}
	if err := s.s.Client.CheckURL(u); err != nil {
		return "", badRequest("url: %v", err)
	}
	return u.String(), nil
}

func checkEvents(in []proteusv1.WebhookEventType) ([]string, error) {
	var out []string
	for _, t := range in {
		name := EventName(t)
		if name == "" || name == Test {
			return nil, badRequest("events: %s cannot be subscribed to", t)
		}
		if !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil, badRequest("events: subscribe to at least one event")
	}
	slices.Sort(out)
	return out, nil
}

func checkDescription(d string) error {
	if !utf8.ValidString(d) {
		return badRequest("description is not valid UTF-8")
	}
	if utf8.RuneCountInString(d) > 500 {
		return badRequest("description is longer than 500 characters")
	}
	return nil
}

// Create saves an endpoint for owner and returns it with its secret.
func (s *Service) Create(ctx context.Context, owner string, req *proteusv1.CreateWebhookEndpointRequest) (*proteusv1.WebhookEndpoint, string, error) {
	u, err := s.checkURL(req.GetUrl())
	if err != nil {
		return nil, "", err
	}
	events, err := checkEvents(req.GetEvents())
	if err != nil {
		return nil, "", err
	}
	if err := checkDescription(req.GetDescription()); err != nil {
		return nil, "", err
	}
	n, err := s.store.CountEndpoints(ctx, owner)
	if err != nil {
		return nil, "", err
	}
	if n >= int64(s.s.MaxEndpoints) {
		return nil, "", ErrLimit
	}
	secret := NewSecret()
	e := &Endpoint{
		Owner: owner, URL: u, Description: req.GetDescription(), Events: events,
		Enabled: !req.GetDisabled(), Secret: s.s.Sealer.Seal(secret),
	}
	if err := s.store.CreateEndpoint(ctx, e); err != nil {
		return nil, "", err
	}
	return EndpointToProto(e, time.Now()), secret, nil
}

// Get returns owner's endpoint.
func (s *Service) Get(ctx context.Context, owner, id string) (*proteusv1.WebhookEndpoint, error) {
	e, err := s.store.GetEndpoint(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	return EndpointToProto(e, time.Now()), nil
}

// List returns owner's endpoints.
func (s *Service) List(ctx context.Context, owner string) ([]*proteusv1.WebhookEndpoint, error) {
	list, err := s.store.ListEndpoints(ctx, owner)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]*proteusv1.WebhookEndpoint, len(list))
	for i, e := range list {
		out[i] = EndpointToProto(e, now)
	}
	return out, nil
}

// Update changes an endpoint's URL, description and events.
func (s *Service) Update(ctx context.Context, owner string, req *proteusv1.UpdateWebhookEndpointRequest) (*proteusv1.WebhookEndpoint, error) {
	u, err := s.checkURL(req.GetUrl())
	if err != nil {
		return nil, err
	}
	events, err := checkEvents(req.GetEvents())
	if err != nil {
		return nil, err
	}
	if err := checkDescription(req.GetDescription()); err != nil {
		return nil, err
	}
	e := &Endpoint{ID: req.GetId(), Owner: owner, URL: u, Description: req.GetDescription(), Events: events}
	if err := s.store.UpdateEndpoint(ctx, e); err != nil {
		return nil, err
	}
	return s.Get(ctx, owner, req.GetId())
}

// Delete removes an endpoint and its log.
func (s *Service) Delete(ctx context.Context, owner, id string) error {
	return s.store.DeleteEndpoint(ctx, owner, id)
}

// SetEnabled enables or disables an endpoint.
func (s *Service) SetEnabled(ctx context.Context, owner, id string, enabled bool) (*proteusv1.WebhookEndpoint, error) {
	if err := s.store.SetEnabled(ctx, owner, id, enabled, time.Now().UTC()); err != nil {
		return nil, err
	}
	return s.Get(ctx, owner, id)
}

// Reveal returns the current secret and, during a rotation, when the
// previous one stops signing.
func (s *Service) Reveal(ctx context.Context, owner, id string) (string, time.Time, error) {
	e, err := s.store.GetEndpoint(ctx, owner, id)
	if err != nil {
		return "", time.Time{}, err
	}
	secret, err := s.s.Sealer.Open(e.Secret)
	if err != nil {
		return "", time.Time{}, err
	}
	var prevExpires time.Time
	if len(e.PreviousSecret) > 0 && e.PreviousExpiresAt.After(time.Now()) {
		prevExpires = e.PreviousExpiresAt
	}
	return secret, prevExpires, nil
}

// Rotate replaces the secret; the old one keeps signing for SecretGrace.
// Rotating again during the grace period drops the oldest secret.
func (s *Service) Rotate(ctx context.Context, owner, id string) (*proteusv1.WebhookEndpoint, string, error) {
	e, err := s.store.GetEndpoint(ctx, owner, id)
	if err != nil {
		return nil, "", err
	}
	if _, err := s.s.Sealer.Open(e.Secret); err != nil {
		return nil, "", err
	}
	secret := NewSecret()
	expires := time.Now().UTC().Add(s.s.SecretGrace)
	if err := s.store.SetSecrets(ctx, owner, id, s.s.Sealer.Seal(secret), e.Secret, expires); err != nil {
		return nil, "", err
	}
	ep, err := s.Get(ctx, owner, id)
	return ep, secret, err
}

// --- deliveries ---

// ListDeliveries returns a page of an endpoint's log, newest first.
func (s *Service) ListDeliveries(ctx context.Context, owner string, req *proteusv1.ListWebhookDeliveriesRequest) ([]*proteusv1.WebhookDelivery, string, error) {
	if _, err := s.store.GetEndpoint(ctx, owner, req.GetEndpointId()); err != nil {
		return nil, "", err
	}
	size := int(req.GetPageSize())
	if size <= 0 {
		size = 50
	}
	size = min(size, 100)
	var after *Cursor
	if tok := req.GetPageToken(); tok != "" {
		c, err := decodeCursor(tok)
		if err != nil {
			return nil, "", badRequest("page_token is invalid")
		}
		after = c
	}
	var status DeliveryStatus
	switch req.GetStatus() {
	case proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_PENDING:
		status = Pending
	case proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED:
		status = Succeeded
	case proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED:
		status = Failed
	}
	list, err := s.store.ListDeliveries(ctx, owner, req.GetEndpointId(), status, after, size+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(list) > size {
		list = list[:size]
		last := list[size-1]
		next = encodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	out := make([]*proteusv1.WebhookDelivery, len(list))
	for i, d := range list {
		out[i] = DeliveryToProto(d)
	}
	return out, next, nil
}

func encodeCursor(c Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.CreatedAt.UnixNano(), 10) + ":" + c.ID))
}

func decodeCursor(tok string) (*Cursor, error) {
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return nil, err
	}
	ns, id, ok := strings.Cut(string(b), ":")
	if !ok {
		return nil, errors.New("bad cursor")
	}
	n, err := strconv.ParseInt(ns, 10, 64)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, err
	}
	return &Cursor{CreatedAt: time.Unix(0, n).UTC(), ID: id}, nil
}

// SendTest sends a test event to the endpoint now, once.
func (s *Service) SendTest(ctx context.Context, owner, id string) (*proteusv1.WebhookDelivery, error) {
	e, err := s.store.GetEndpoint(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	eventID := uuid.NewString()
	body, err := protojson.Marshal(&proteusv1.WebhookEvent{Id: eventID, Type: Test, CreateTime: timestamppb.Now()})
	if err != nil {
		return nil, err
	}
	d := &Delivery{Owner: owner, EndpointID: e.ID, EventID: eventID, EventType: Test, Payload: body, Manual: true}
	return s.sendNow(ctx, e, d)
}

// Redeliver sends a logged delivery's event again, now, once.
func (s *Service) Redeliver(ctx context.Context, owner, id string) (*proteusv1.WebhookDelivery, error) {
	orig, err := s.store.GetDelivery(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	e, err := s.store.GetEndpoint(ctx, owner, orig.EndpointID)
	if err != nil {
		return nil, err
	}
	d := &Delivery{
		Owner: owner, EndpointID: e.ID, EventID: orig.EventID, EventType: orig.EventType,
		SubjectID: orig.SubjectID, Payload: orig.Payload, RedeliveryOf: orig.ID, Manual: true,
	}
	return s.sendNow(ctx, e, d)
}

// sendNow makes the one attempt of a manual delivery and logs it. Manual
// deliveries do not change the endpoint's health.
func (s *Service) sendNow(ctx context.Context, e *Endpoint, d *Delivery) (*proteusv1.WebhookDelivery, error) {
	d.ID = uuid.NewString() // the X-Proteus-Delivery header names it
	a := s.attempt(ctx, e, d)
	d.Attempts, d.MaxAttempts = 1, 1
	d.StatusCode, d.LatencyMS, d.Error, d.LastAttemptAt = a.statusCode, a.latency.Milliseconds(), a.err, a.at
	d.Status = Failed
	if a.ok() {
		d.Status = Succeeded
	}
	s.s.Observer.WebhookDone(outcome(d.Status))
	// The request's context may be gone once the endpoint answered; the
	// log entry is still worth writing.
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := s.store.CreateDelivery(wctx, d); err != nil {
		return nil, err
	}
	return DeliveryToProto(d), nil
}

// --- publishing ---

// Publish queues the event for every enabled endpoint of owner that
// subscribes to it. Errors are logged; publishing never fails the action
// that caused the event.
func (s *Service) Publish(ctx context.Context, owner string, ev *proteusv1.WebhookEvent, subjectID string) {
	ctx = context.WithoutCancel(ctx)
	eps, err := s.store.ListEndpoints(ctx, owner)
	if err != nil {
		s.s.Logger.Warn("webhook publish failed", "event", ev.GetType(), "error", err)
		return
	}
	var body []byte
	now := time.Now().UTC()
	for _, e := range eps {
		if !e.Enabled || !slices.Contains(e.Events, ev.GetType()) {
			continue
		}
		if body == nil {
			if ev.Id == "" {
				ev.Id = uuid.NewString()
			}
			if ev.CreateTime == nil {
				ev.CreateTime = timestamppb.New(now)
			}
			if body, err = protojson.Marshal(ev); err != nil {
				s.s.Logger.Error("webhook event encoding failed", "event", ev.GetType(), "error", err)
				return
			}
		}
		d := &Delivery{
			Owner: owner, EndpointID: e.ID, EventID: ev.GetId(), EventType: ev.GetType(), SubjectID: subjectID,
			Status: Pending, Payload: body, MaxAttempts: s.s.MaxAttempts, NextAttemptAt: now,
		}
		if err := s.store.CreateDelivery(ctx, d); err != nil {
			s.s.Logger.Warn("webhook delivery not queued", "endpoint_id", e.ID, "event", ev.GetType(), "error", err)
			continue
		}
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// NotifyJob publishes a job event (implements jobs.Notifier).
func (s *Service) NotifyJob(ctx context.Context, owner, event string, job *proteusv1.Job) {
	s.Publish(ctx, owner, &proteusv1.WebhookEvent{Type: event, Data: &proteusv1.WebhookEvent_Job{Job: job}}, job.GetId())
}

// NotifyTemplateUpdated publishes template.updated with the template's
// metadata only.
func (s *Service) NotifyTemplateUpdated(ctx context.Context, owner string, t *proteusv1.Template) {
	meta := proto.CloneOf(t)
	meta.Content, meta.Assets, meta.SampleData, meta.Options = nil, nil, nil, nil
	s.Publish(ctx, owner, &proteusv1.WebhookEvent{Type: TemplateUpdated, Data: &proteusv1.WebhookEvent_Template{Template: meta}}, t.GetId())
}

// --- API form ---

func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

// EndpointToProto converts an endpoint (never its secrets) to its API form.
func EndpointToProto(e *Endpoint, now time.Time) *proteusv1.WebhookEndpoint {
	p := &proteusv1.WebhookEndpoint{
		Id: e.ID, Url: e.URL, Description: e.Description, Enabled: e.Enabled, PauseTime: ts(e.PausedAt),
		ConsecutiveFailures: int32(e.ConsecutiveFailures), CreateTime: ts(e.CreatedAt), UpdateTime: ts(e.UpdatedAt),
		LastDeliveryTime: ts(e.LastDeliveryAt), LastStatusCode: int32(e.LastStatusCode),
	}
	if len(e.PreviousSecret) > 0 && e.PreviousExpiresAt.After(now) {
		p.PreviousSecretExpireTime = ts(e.PreviousExpiresAt)
	}
	for _, n := range e.Events {
		if t := EventType(n); t != proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_UNSPECIFIED {
			p.Events = append(p.Events, t)
		}
	}
	return p
}

// DeliveryToProto converts a delivery to its API form.
func DeliveryToProto(d *Delivery) *proteusv1.WebhookDelivery {
	p := &proteusv1.WebhookDelivery{
		Id: d.ID, EndpointId: d.EndpointID, EventId: d.EventID, EventType: EventType(d.EventType),
		SubjectId: d.SubjectID, Attempts: int32(d.Attempts), MaxAttempts: int32(d.MaxAttempts),
		StatusCode: int32(d.StatusCode), LatencyMs: d.LatencyMS, Error: d.Error,
		CreateTime: ts(d.CreatedAt), LastAttemptTime: ts(d.LastAttemptAt), RedeliveryOf: d.RedeliveryOf, Manual: d.Manual,
	}
	switch d.Status {
	case Pending:
		p.Status = proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_PENDING
		p.NextAttemptTime = ts(d.NextAttemptAt)
	case Succeeded:
		p.Status = proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED
	case Failed:
		p.Status = proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED
	}
	return p
}
