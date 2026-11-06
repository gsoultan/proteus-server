// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package rpc

import (
	"context"
	"crypto/hmac"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/encoding/protojson"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/webhooks"
)

func (e *env) webhookClient(key string) proteusv1connect.WebhookServiceClient {
	return proteusv1connect.NewWebhookServiceClient(connect.NewClient(
		connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC()), withKey(key)))
}

type received struct {
	event, delivery, ts, sig string
	body                     []byte
}

// hookReceiver records every POST and answers with status().
type hookReceiver struct {
	*httptest.Server
	mu     sync.Mutex
	got    []received
	notify chan struct{}
	status atomic.Int32
}

func newReceiver(t *testing.T) *hookReceiver {
	h := &hookReceiver{notify: make(chan struct{}, 100)}
	h.status.Store(http.StatusOK)
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		h.mu.Lock()
		h.got = append(h.got, received{r.Header.Get("X-Proteus-Event"), r.Header.Get("X-Proteus-Delivery"),
			r.Header.Get("X-Proteus-Timestamp"), r.Header.Get("X-Proteus-Signature"), b})
		h.mu.Unlock()
		w.WriteHeader(int(h.status.Load()))
		h.notify <- struct{}{}
	}))
	t.Cleanup(h.Close)
	return h
}

// wait returns the first received delivery of event.
func (h *hookReceiver) wait(t *testing.T, event string) received {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		h.mu.Lock()
		for _, r := range h.got {
			if r.event == event {
				h.mu.Unlock()
				return r
			}
		}
		h.mu.Unlock()
		select {
		case <-h.notify:
		case <-deadline:
			t.Fatalf("no %s delivery", event)
		}
	}
}

func (h *hookReceiver) count(event string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.got {
		if r.event == event {
			n++
		}
	}
	return n
}

// verify checks that one of the signatures in the header matches secret.
func verify(r received, secret string) bool {
	want := "sha256=" + webhooks.Sign([]byte(secret), r.ts, r.body)
	for _, s := range strings.Split(r.sig, ",") {
		if hmac.Equal([]byte(strings.TrimSpace(s)), []byte(want)) {
			return true
		}
	}
	return false
}

var (
	evQueued    = proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_QUEUED
	evSucceeded = proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_SUCCEEDED
	evFailed    = proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_JOB_FAILED
	evTemplate  = proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_TEMPLATE_UPDATED
)

func TestWebhookEndpointsLifecycle(t *testing.T) {
	recv := newReceiver(t)
	e := newEnv(t, envOpts{})
	a, b := e.webhookClient(keyA), e.webhookClient(keyB)
	jobsA := e.grpcClient(keyA)
	ctx := context.Background()

	created, err := a.CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
		Url: recv.URL + "/hooks", Description: "ERP", Events: []proteusv1.WebhookEventType{evSucceeded, evQueued, evSucceeded},
	})
	if err != nil {
		t.Fatal(err)
	}
	ep, secret := created.GetEndpoint(), created.GetSecret()
	if !strings.HasPrefix(secret, "whsec_") || len(secret) != 6+43 || !ep.GetEnabled() || len(ep.GetEvents()) != 2 ||
		ep.GetUrl() != recv.URL+"/hooks" || ep.GetCreateTime() == nil {
		t.Fatalf("created %v, secret %q", ep, secret)
	}

	// Other keys cannot see or use it.
	_, err = b.GetWebhookEndpoint(ctx, &proteusv1.GetWebhookEndpointRequest{Id: ep.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	_, err = b.RevealWebhookSecret(ctx, &proteusv1.RevealWebhookSecretRequest{Id: ep.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	if l, err := b.ListWebhookEndpoints(ctx, &proteusv1.ListWebhookEndpointsRequest{}); err != nil || len(l.GetEndpoints()) != 0 {
		t.Fatalf("b list = %v, %v", l, err)
	}

	rev, err := a.RevealWebhookSecret(ctx, &proteusv1.RevealWebhookSecretRequest{Id: ep.GetId()})
	if err != nil || rev.GetSecret() != secret || rev.GetPreviousSecretExpireTime() != nil {
		t.Fatalf("reveal = %v, %v", rev, err)
	}

	// A job sends job.queued and job.succeeded, signed with the endpoint's secret.
	sub, err := jobsA.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# hook")})
	if err != nil {
		t.Fatal(err)
	}
	jobID := sub.GetJob().GetId()
	q := recv.wait(t, "job.queued")
	done := recv.wait(t, "job.succeeded")
	for _, r := range []received{q, done} {
		if !verify(r, secret) || strings.Contains(r.sig, ",") {
			t.Fatalf("signature %q does not verify", r.sig)
		}
		var ev proteusv1.WebhookEvent
		if err := protojson.Unmarshal(r.body, &ev); err != nil || ev.GetJob().GetId() != jobID || ev.GetId() == "" ||
			ev.GetType() != r.event || ev.GetCreateTime() == nil {
			t.Fatalf("event %s: %v", r.body, err)
		}
	}
	// Key B's jobs do not reach key A's endpoint.
	if _, err := e.grpcClient(keyB).SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# b")}); err != nil {
		t.Fatal(err)
	}

	// The log shows both deliveries with their outcome.
	var log []*proteusv1.WebhookDelivery
	deadline := time.Now().Add(10 * time.Second)
	for {
		l, err := a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{
			EndpointId: ep.GetId(), Status: proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED})
		if err != nil {
			t.Fatal(err)
		}
		if log = l.GetDeliveries(); len(log) == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(log) != 2 {
		t.Fatalf("log %v", log)
	}
	for _, d := range log {
		if d.GetStatusCode() != 200 || d.GetAttempts() != 1 || d.GetMaxAttempts() != 3 || d.GetSubjectId() != jobID ||
			d.GetLastAttemptTime() == nil || d.GetManual() {
			t.Fatalf("delivery %v", d)
		}
	}
	if log[0].GetCreateTime().AsTime().Before(log[1].GetCreateTime().AsTime()) {
		t.Fatal("log is not newest first")
	}
	_, err = b.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{EndpointId: ep.GetId()})
	expectCode(t, err, connect.CodeNotFound)

	// Pages.
	p1, err := a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{EndpointId: ep.GetId(), PageSize: 1})
	if err != nil || len(p1.GetDeliveries()) != 1 || p1.GetNextPageToken() == "" {
		t.Fatalf("page 1 = %v, %v", p1, err)
	}
	p2, err := a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{EndpointId: ep.GetId(), PageSize: 1, PageToken: p1.GetNextPageToken()})
	if err != nil || len(p2.GetDeliveries()) != 1 || p2.GetDeliveries()[0].GetId() == p1.GetDeliveries()[0].GetId() {
		t.Fatalf("page 2 = %v, %v", p2, err)
	}

	// Redeliver repeats the event under a new delivery id.
	red, err := a.RedeliverWebhookDelivery(ctx, &proteusv1.RedeliverWebhookDeliveryRequest{Id: log[0].GetId()})
	if err != nil {
		t.Fatal(err)
	}
	d := red.GetDelivery()
	if d.GetRedeliveryOf() != log[0].GetId() || d.GetEventId() != log[0].GetEventId() || !d.GetManual() ||
		d.GetStatus() != proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_SUCCEEDED || d.GetId() == log[0].GetId() {
		t.Fatalf("redelivery %v", d)
	}
	_, err = b.RedeliverWebhookDelivery(ctx, &proteusv1.RedeliverWebhookDeliveryRequest{Id: log[0].GetId()})
	expectCode(t, err, connect.CodeNotFound)

	// Rotation: both secrets sign for the grace period.
	rot, err := a.RotateWebhookSecret(ctx, &proteusv1.RotateWebhookSecretRequest{Id: ep.GetId()})
	if err != nil || rot.GetSecret() == secret || rot.GetEndpoint().GetPreviousSecretExpireTime() == nil {
		t.Fatalf("rotate = %v, %v", rot, err)
	}
	if exp := rot.GetEndpoint().GetPreviousSecretExpireTime().AsTime(); exp.Sub(time.Now()) < 23*time.Hour {
		t.Fatalf("previous secret expires at %v", exp)
	}
	test, err := a.SendTestWebhookEvent(ctx, &proteusv1.SendTestWebhookEventRequest{Id: ep.GetId()})
	if err != nil || test.GetDelivery().GetStatusCode() != 200 || test.GetDelivery().GetEventType() != proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_TEST {
		t.Fatalf("test = %v, %v", test, err)
	}
	tr := recv.wait(t, "test")
	if !verify(tr, rot.GetSecret()) || !verify(tr, secret) || !strings.HasPrefix(tr.sig, "sha256="+webhooks.Sign([]byte(rot.GetSecret()), tr.ts, tr.body)) {
		t.Fatalf("rotated signatures %q", tr.sig)
	}
	if tr.delivery != test.GetDelivery().GetId() {
		t.Fatalf("delivery header %q, want %q", tr.delivery, test.GetDelivery().GetId())
	}
	rev, err = a.RevealWebhookSecret(ctx, &proteusv1.RevealWebhookSecretRequest{Id: ep.GetId()})
	if err != nil || rev.GetSecret() != rot.GetSecret() || rev.GetPreviousSecretExpireTime() == nil {
		t.Fatalf("reveal after rotate = %v, %v", rev, err)
	}

	// Update the events; disabled endpoints get nothing.
	up, err := a.UpdateWebhookEndpoint(ctx, &proteusv1.UpdateWebhookEndpointRequest{
		Id: ep.GetId(), Url: ep.GetUrl(), Description: "ERP v2", Events: []proteusv1.WebhookEventType{evSucceeded}})
	if err != nil || up.GetEndpoint().GetDescription() != "ERP v2" || len(up.GetEndpoint().GetEvents()) != 1 {
		t.Fatalf("update = %v, %v", up, err)
	}
	dis, err := a.DisableWebhookEndpoint(ctx, &proteusv1.DisableWebhookEndpointRequest{Id: ep.GetId()})
	if err != nil || dis.GetEndpoint().GetEnabled() {
		t.Fatalf("disable = %v, %v", dis, err)
	}
	before := recv.count("job.succeeded")
	sub2, err := jobsA.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# off")})
	if err != nil {
		t.Fatal(err)
	}
	waitJob(t, jobsA, sub2.GetJob().GetId())
	time.Sleep(200 * time.Millisecond)
	if n := recv.count("job.succeeded"); n != before {
		t.Fatalf("disabled endpoint got %d more deliveries", n-before)
	}
	en, err := a.EnableWebhookEndpoint(ctx, &proteusv1.EnableWebhookEndpointRequest{Id: ep.GetId()})
	if err != nil || !en.GetEndpoint().GetEnabled() {
		t.Fatalf("enable = %v, %v", en, err)
	}

	if _, err := a.DeleteWebhookEndpoint(ctx, &proteusv1.DeleteWebhookEndpointRequest{Id: ep.GetId()}); err != nil {
		t.Fatal(err)
	}
	_, err = a.GetWebhookEndpoint(ctx, &proteusv1.GetWebhookEndpointRequest{Id: ep.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	_, err = a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{EndpointId: ep.GetId()})
	expectCode(t, err, connect.CodeNotFound)
}

func TestWebhookPerJobURLStillWorks(t *testing.T) {
	// A saved endpoint and a per-job URL on the same job both get called,
	// each with its own payload and secret.
	saved, perJob := newReceiver(t), newReceiver(t)
	const serverSecret = "0123456789abcdef0123456789abcdef"
	e := newEnv(t, envOpts{webhookSecret: serverSecret})
	ctx := context.Background()
	c, err := e.webhookClient(keyA).CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
		Url: saved.URL, Events: []proteusv1.WebhookEventType{evSucceeded, evFailed}})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := e.grpcClient(keyA).SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# both"), WebhookUrl: perJob.URL})
	if err != nil {
		t.Fatal(err)
	}
	s := saved.wait(t, "job.succeeded")
	if !verify(s, c.GetSecret()) {
		t.Fatal("saved endpoint signature")
	}
	deadline := time.After(10 * time.Second)
	for {
		perJob.mu.Lock()
		n := len(perJob.got)
		perJob.mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-perJob.notify:
		case <-deadline:
			t.Fatal("no per-job webhook")
		}
	}
	r := perJob.got[0]
	var j proteusv1.Job
	if err := protojson.Unmarshal(r.body, &j); err != nil || j.GetId() != sub.GetJob().GetId() || !verify(r, serverSecret) {
		t.Fatalf("per-job body %s: %v", r.body, err)
	}
}

func TestWebhookRetriesAndAutoPause(t *testing.T) {
	recv := newReceiver(t)
	recv.status.Store(http.StatusServiceUnavailable)
	e := newEnv(t, envOpts{pauseAfter: 2})
	a := e.webhookClient(keyA)
	ctx := context.Background()
	c, err := a.CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
		Url: recv.URL, Events: []proteusv1.WebhookEventType{evQueued}})
	if err != nil {
		t.Fatal(err)
	}
	id := c.GetEndpoint().GetId()

	// A manual test failure does not count towards pausing.
	test, err := a.SendTestWebhookEvent(ctx, &proteusv1.SendTestWebhookEventRequest{Id: id})
	if err != nil || test.GetDelivery().GetStatus() != proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED ||
		test.GetDelivery().GetStatusCode() != 503 || test.GetDelivery().GetError() == "" {
		t.Fatalf("test = %v, %v", test, err)
	}

	jobsA := e.grpcClient(keyA)
	for range 2 {
		if _, err := jobsA.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# fail")}); err != nil {
			t.Fatal(err)
		}
	}
	var ep *proteusv1.WebhookEndpoint
	deadline := time.Now().Add(20 * time.Second)
	for {
		g, err := a.GetWebhookEndpoint(ctx, &proteusv1.GetWebhookEndpointRequest{Id: id})
		if err != nil {
			t.Fatal(err)
		}
		if ep = g.GetEndpoint(); !ep.GetEnabled() || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ep.GetEnabled() || ep.GetPauseTime() == nil || ep.GetConsecutiveFailures() != 2 || ep.GetLastStatusCode() != 503 {
		t.Fatalf("not paused: %v", ep)
	}
	// Each failed delivery was tried three times.
	if n := recv.count("job.queued"); n != 6 {
		t.Fatalf("%d attempts, want 6", n)
	}
	l, err := a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{
		EndpointId: id, Status: proteusv1.WebhookDeliveryStatus_WEBHOOK_DELIVERY_STATUS_FAILED})
	if err != nil || len(l.GetDeliveries()) != 3 {
		t.Fatalf("failed log = %v, %v", l, err)
	}
	for _, d := range l.GetDeliveries() {
		if !d.GetManual() && (d.GetAttempts() != 3 || d.GetStatusCode() != 503) {
			t.Fatalf("delivery %v", d)
		}
	}

	// Enabling resumes deliveries and resets the count.
	recv.status.Store(http.StatusOK)
	en, err := a.EnableWebhookEndpoint(ctx, &proteusv1.EnableWebhookEndpointRequest{Id: id})
	if err != nil || !en.GetEndpoint().GetEnabled() || en.GetEndpoint().GetPauseTime() != nil || en.GetEndpoint().GetConsecutiveFailures() != 0 {
		t.Fatalf("enable = %v, %v", en, err)
	}
}

func TestWebhookTemplateUpdated(t *testing.T) {
	recv := newReceiver(t)
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	if _, err := e.webhookClient(keyA).CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
		Url: recv.URL, Events: []proteusv1.WebhookEventType{evTemplate}}); err != nil {
		t.Fatal(err)
	}
	tc := e.templateClient(keyA)
	c, err := tc.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
		Name: "t", Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Content: []byte("<p>{{.x}}</p>")}})
	if err != nil {
		t.Fatal(err)
	}
	tpl := c.GetTemplate()
	tpl.Content = []byte("<p>{{.y}}</p>")
	if _, err := tc.UpdateTemplate(ctx, &proteusv1.UpdateTemplateRequest{Template: tpl}); err != nil {
		t.Fatal(err)
	}
	r := recv.wait(t, "template.updated")
	var ev proteusv1.WebhookEvent
	if err := protojson.Unmarshal(r.body, &ev); err != nil || ev.GetTemplate().GetId() != tpl.GetId() ||
		ev.GetTemplate().GetRevision() != 2 || len(ev.GetTemplate().GetContent()) != 0 {
		t.Fatalf("event %s: %v", r.body, err)
	}
}

func TestWebhookValidation(t *testing.T) {
	e := newEnv(t, envOpts{})
	a := e.webhookClient(keyA)
	ctx := context.Background()
	for _, req := range []*proteusv1.CreateWebhookEndpointRequest{
		{Url: "https://example.com/h"}, // no events
		{Url: "ftp://example.com/h", Events: []proteusv1.WebhookEventType{evQueued}},
		{Url: "https://user:pw@example.com/h", Events: []proteusv1.WebhookEventType{evQueued}},
		{Url: "https://example.com/h", Events: []proteusv1.WebhookEventType{proteusv1.WebhookEventType_WEBHOOK_EVENT_TYPE_TEST}},
		{Url: "https://example.com/h", Events: []proteusv1.WebhookEventType{evQueued}, Description: strings.Repeat("x", 501)},
	} {
		_, err := a.CreateWebhookEndpoint(ctx, req)
		expectCode(t, err, connect.CodeInvalidArgument)
	}
	_, err := a.GetWebhookEndpoint(ctx, &proteusv1.GetWebhookEndpointRequest{Id: "nope"})
	expectCode(t, err, connect.CodeNotFound)
	_, err = a.ListWebhookDeliveries(ctx, &proteusv1.ListWebhookDeliveriesRequest{EndpointId: "nope"})
	expectCode(t, err, connect.CodeNotFound)
	for range 20 {
		if _, err := a.CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
			Url: "https://example.com/h", Events: []proteusv1.WebhookEventType{evQueued}}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = a.CreateWebhookEndpoint(ctx, &proteusv1.CreateWebhookEndpointRequest{
		Url: "https://example.com/h", Events: []proteusv1.WebhookEventType{evQueued}})
	expectCode(t, err, connect.CodeResourceExhausted)
}
