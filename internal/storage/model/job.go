// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package model is the storm model for the Postgres job queue. Generated
// code lives in internal/storage/store; regenerate with `storm generate
// internal/storage/store` after editing this file.
package model

import (
	"time"

	"github.com/gsoultan/storm"
)

// Job is one asynchronous conversion. Payload is the serialized
// SubmitJobRequest; Result is the converted document once the job has
// succeeded.
type Job struct {
	storm.Model // uuid id + created_at/updated_at

	Owner        string // API key id that submitted the job
	Status       string // queued, running, succeeded, failed
	InputFormat  string // html, markdown
	OutputFormat string // pdf
	FileName     string // suggested download name of the result
	Payload      []byte
	WebhookURL   *string

	Attempts     int32
	LeaseExpires *time.Time
	StartedAt    *time.Time
	FinishedAt   *time.Time
	ExpiresAt    *time.Time

	Error       *string
	Warnings    []string
	ResultBytes int64
	Result      []byte
}

func (j *Job) Schema(t *storm.Table) {
	t.Col(&j.Owner).Size(128)
	t.Col(&j.Attempts).Default("0")
	t.Col(&j.ResultBytes).Default("0")
	t.Col(&j.Result).Nullable()
	t.Col(&j.Warnings).Nullable()
	t.Check(storm.RawSQL(`status IN ('queued', 'running', 'succeeded', 'failed')`))
	// The claim scans queued and leased rows oldest first.
	t.Index(&j.Status, &j.CreatedAt)
	t.Index(&j.ExpiresAt)
	t.Takes()
}

// Projections: Meta is every column but the payload and the result, so status
// reads never pull the result bytes.
func (j *Job) Projections(p *storm.Projections) {
	p.Named("Meta", &j.ID, &j.Owner, &j.Status, &j.InputFormat, &j.OutputFormat, &j.FileName, &j.WebhookURL, &j.Attempts,
		&j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.ExpiresAt, &j.Error, &j.Warnings, &j.ResultBytes)
}
