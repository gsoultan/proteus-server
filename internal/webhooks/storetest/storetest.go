// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package storetest is a conformance suite every webhooks.Store must pass.
package storetest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gsoultan/proteus-server/internal/webhooks"
)

// Run exercises s. The store must be empty.
func Run(t *testing.T, s webhooks.Store) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	a := &webhooks.Endpoint{Owner: "alice", URL: "https://a.example/h", Description: "d",
		Events: []string{"job.failed", "job.succeeded"}, Enabled: true, Secret: []byte{0, 's'}}
	t.Run("Endpoints", func(t *testing.T) {
		if err := s.CreateEndpoint(ctx, a); err != nil {
			t.Fatal(err)
		}
		if a.ID == "" || a.CreatedAt.IsZero() {
			t.Fatalf("created %+v", a)
		}
		got, err := s.GetEndpoint(ctx, "alice", a.ID)
		if err != nil || got.URL != a.URL || !slices.Equal(got.Events, a.Events) || !got.Enabled ||
			string(got.Secret) != string(a.Secret) || got.Description != "d" {
			t.Fatalf("Get = %+v, %v", got, err)
		}
		if _, err := s.GetEndpoint(ctx, "bob", a.ID); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bob Get = %v", err)
		}
		if _, err := s.GetEndpoint(ctx, "alice", "not-a-uuid"); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bad id Get = %v", err)
		}
		b := &webhooks.Endpoint{Owner: "alice", URL: "https://b.example/h", Events: []string{"job.queued"}, Secret: []byte{0, 't'}}
		if err := s.CreateEndpoint(ctx, b); err != nil {
			t.Fatal(err)
		}
		list, err := s.ListEndpoints(ctx, "alice")
		if err != nil || len(list) != 2 || list[0].ID != a.ID || list[1].Enabled {
			t.Fatalf("List = %+v, %v", list, err)
		}
		if n, err := s.CountEndpoints(ctx, "alice"); err != nil || n != 2 {
			t.Fatalf("Count = %d, %v", n, err)
		}
		if n, _ := s.CountEndpoints(ctx, "bob"); n != 0 {
			t.Fatalf("bob Count = %d", n)
		}

		up := &webhooks.Endpoint{ID: b.ID, Owner: "alice", URL: "https://c.example/h", Description: "x", Events: []string{"template.updated"}}
		if err := s.UpdateEndpoint(ctx, up); err != nil {
			t.Fatal(err)
		}
		up.Owner = "bob"
		if err := s.UpdateEndpoint(ctx, up); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bob Update = %v", err)
		}
		got, _ = s.GetEndpoint(ctx, "alice", b.ID)
		if got.URL != "https://c.example/h" || got.Description != "x" || !slices.Equal(got.Events, []string{"template.updated"}) {
			t.Fatalf("updated %+v", got)
		}

		exp := now.Add(time.Hour)
		if err := s.SetSecrets(ctx, "alice", b.ID, []byte{0, 'n'}, []byte{0, 't'}, exp); err != nil {
			t.Fatal(err)
		}
		got, _ = s.GetEndpoint(ctx, "alice", b.ID)
		if string(got.Secret) != "\x00n" || string(got.PreviousSecret) != "\x00t" || !got.PreviousExpiresAt.Equal(exp) {
			t.Fatalf("secrets %+v", got)
		}
		if err := s.DeleteEndpoint(ctx, "bob", b.ID); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bob Delete = %v", err)
		}
		if err := s.DeleteEndpoint(ctx, "alice", b.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetEndpoint(ctx, "alice", b.ID); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("deleted Get = %v", err)
		}
	})

	t.Run("HealthAndPause", func(t *testing.T) {
		for i := range 2 {
			paused, err := s.RecordResult(ctx, "alice", a.ID, webhooks.Result{StatusCode: 503, At: now}, 3)
			if err != nil || paused {
				t.Fatalf("failure %d: paused=%v err=%v", i, paused, err)
			}
		}
		if _, err := s.RecordResult(ctx, "alice", a.ID, webhooks.Result{OK: true, StatusCode: 200, At: now}, 3); err != nil {
			t.Fatal(err)
		}
		got, _ := s.GetEndpoint(ctx, "alice", a.ID)
		if got.ConsecutiveFailures != 0 || got.LastStatusCode != 200 || !got.LastDeliveryAt.Equal(now) {
			t.Fatalf("after success %+v", got)
		}
		var pauses int
		for range 4 {
			p, err := s.RecordResult(ctx, "alice", a.ID, webhooks.Result{At: now}, 3)
			if err != nil {
				t.Fatal(err)
			}
			if p {
				pauses++
			}
		}
		got, _ = s.GetEndpoint(ctx, "alice", a.ID)
		if pauses != 1 || got.Enabled || !got.PausedAt.Equal(now) || got.ConsecutiveFailures != 4 || got.LastStatusCode != 0 {
			t.Fatalf("pauses=%d endpoint %+v", pauses, got)
		}
		if err := s.SetEnabled(ctx, "alice", a.ID, true, now); err != nil {
			t.Fatal(err)
		}
		got, _ = s.GetEndpoint(ctx, "alice", a.ID)
		if !got.Enabled || !got.PausedAt.IsZero() || got.ConsecutiveFailures != 0 {
			t.Fatalf("re-enabled %+v", got)
		}
		if err := s.SetEnabled(ctx, "bob", a.ID, false, now); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bob SetEnabled = %v", err)
		}
	})

	var first *webhooks.Delivery
	t.Run("DeliveryQueue", func(t *testing.T) {
		mk := func(status webhooks.DeliveryStatus, next time.Time) *webhooks.Delivery {
			return &webhooks.Delivery{Owner: "alice", EndpointID: a.ID, EventID: uuid.NewString(), EventType: "job.succeeded",
				SubjectID: "job-1", Status: status, Payload: []byte(`{"id":"x"}`), MaxAttempts: 3, NextAttemptAt: next}
		}
		first = mk(webhooks.Pending, now.Add(-time.Second))
		if err := s.CreateDelivery(ctx, first); err != nil {
			t.Fatal(err)
		}
		later := mk(webhooks.Pending, now.Add(time.Hour))
		if err := s.CreateDelivery(ctx, later); err != nil {
			t.Fatal(err)
		}
		done := mk(webhooks.Succeeded, time.Time{})
		done.Manual, done.Attempts, done.StatusCode, done.RedeliveryOf = true, 1, 200, first.ID
		if err := s.CreateDelivery(ctx, done); err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDelivery(ctx, now, time.Minute)
		if err != nil || got == nil || got.ID != first.ID || got.Attempts != 1 || string(got.Payload) != `{"id":"x"}` {
			t.Fatalf("Claim = %+v, %v", got, err)
		}
		// Claimed: not due again until the lease runs out.
		if again, err := s.ClaimDelivery(ctx, now, time.Minute); err != nil || again != nil {
			t.Fatalf("second Claim = %+v, %v", again, err)
		}
		if lapsed, err := s.ClaimDelivery(ctx, now.Add(2*time.Minute), time.Minute); err != nil || lapsed == nil ||
			lapsed.ID != first.ID || lapsed.Attempts != 2 {
			t.Fatalf("lapsed Claim = %+v, %v", lapsed, err)
		}

		got.Attempts = 2
		got.Status, got.StatusCode, got.LatencyMS, got.Error = webhooks.Failed, 503, 42, "endpoint answered 503"
		got.LastAttemptAt, got.NextAttemptAt = now, time.Time{}
		if err := s.FinishAttempt(ctx, got); err != nil {
			t.Fatal(err)
		}
		d, err := s.GetDelivery(ctx, "alice", first.ID)
		if err != nil || d.Status != webhooks.Failed || d.StatusCode != 503 || d.LatencyMS != 42 || d.Attempts != 2 ||
			d.Error == "" || !d.LastAttemptAt.Equal(now) || !d.NextAttemptAt.IsZero() || string(d.Payload) != `{"id":"x"}` {
			t.Fatalf("finished %+v, %v", d, err)
		}
		if _, err := s.GetDelivery(ctx, "bob", first.ID); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("bob GetDelivery = %v", err)
		}

		all, err := s.ListDeliveries(ctx, "alice", a.ID, "", nil, 10)
		if err != nil || len(all) != 3 || all[0].ID != done.ID || all[2].ID != first.ID || all[0].Payload != nil ||
			!all[0].Manual || all[0].RedeliveryOf != first.ID {
			t.Fatalf("List = %+v, %v", all, err)
		}
		page, err := s.ListDeliveries(ctx, "alice", a.ID, "", &webhooks.Cursor{CreatedAt: all[0].CreatedAt, ID: all[0].ID}, 1)
		if err != nil || len(page) != 1 || page[0].ID != all[1].ID {
			t.Fatalf("List after = %+v, %v", page, err)
		}
		failed, err := s.ListDeliveries(ctx, "alice", a.ID, webhooks.Failed, nil, 10)
		if err != nil || len(failed) != 1 || failed[0].ID != first.ID {
			t.Fatalf("List failed = %+v, %v", failed, err)
		}
		if l, _ := s.ListDeliveries(ctx, "bob", a.ID, "", nil, 10); len(l) != 0 {
			t.Fatalf("bob List = %+v", l)
		}
	})

	t.Run("SweepAndDelete", func(t *testing.T) {
		if n, err := s.SweepDeliveries(ctx, now.Add(-time.Hour)); err != nil || n != 0 {
			t.Fatalf("Sweep old = %d, %v", n, err)
		}
		if n, err := s.SweepDeliveries(ctx, time.Now().UTC().Add(time.Hour)); err != nil || n != 3 {
			t.Fatalf("Sweep all = %d, %v", n, err)
		}
		d := &webhooks.Delivery{Owner: "alice", EndpointID: a.ID, EventID: uuid.NewString(), EventType: "test", Status: webhooks.Failed}
		if err := s.CreateDelivery(ctx, d); err != nil {
			t.Fatal(err)
		}
		if err := s.DeleteEndpoint(ctx, "alice", a.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.GetDelivery(ctx, "alice", d.ID); !errors.Is(err, webhooks.ErrNotFound) {
			t.Fatalf("delivery of deleted endpoint = %v", err)
		}
	})
}
