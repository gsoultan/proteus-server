// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package metrics exposes Prometheus metrics for the server: RED metrics per
// RPC procedure, conversion outcomes and durations, pool saturation and queue depth.
package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every collector on a private registry.
type Metrics struct {
	reg *prometheus.Registry

	requests    *prometheus.CounterVec
	latency     *prometheus.HistogramVec
	conversions *prometheus.CounterVec
	convTime    *prometheus.HistogramVec
	outBytes    prometheus.Histogram
	blocked     *prometheus.CounterVec
	slotsInUse  prometheus.Gauge
	slotsTotal  prometheus.Gauge
	queueDepth  prometheus.Gauge
	webhooks    *prometheus.CounterVec
}

// New registers all collectors. slots is the size of the conversion pool.
func New(slots int) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	f := func(c prometheus.Collector) { reg.MustRegister(c) }
	m := &Metrics{
		reg: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proteus_rpc_requests_total", Help: "RPCs by procedure, protocol (grpc, grpcweb, connect) and status code.",
		}, []string{"procedure", "protocol", "code"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "proteus_rpc_duration_seconds", Help: "RPC latency by procedure.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"procedure"}),
		conversions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proteus_conversions_total", Help: "Conversions by input format, output format and outcome.",
		}, []string{"input", "output", "outcome"}),
		convTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "proteus_conversion_duration_seconds", Help: "Converter time per conversion.",
			Buckets: []float64{.01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"input", "output"}),
		outBytes: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "proteus_output_bytes", Help: "Size of produced outputs.",
			Buckets: prometheus.ExponentialBuckets(4<<10, 4, 9),
		}),
		blocked: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proteus_resource_blocked_total", Help: "Resource loads refused, by reason (ssrf, scheme, too_large, ...).",
		}, []string{"reason"}),
		slotsInUse: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "proteus_conversion_slots_in_use", Help: "Conversion slots currently taken.",
		}),
		slotsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "proteus_conversion_slots", Help: "Size of the conversion pool.",
		}),
		queueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "proteus_job_queue_depth", Help: "Async jobs waiting for a worker.",
		}),
		webhooks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "proteus_webhook_deliveries_total", Help: "Webhook deliveries by outcome.",
		}, []string{"outcome"}),
	}
	for _, c := range []prometheus.Collector{m.requests, m.latency, m.conversions, m.convTime, m.outBytes,
		m.blocked, m.slotsInUse, m.slotsTotal, m.queueDepth, m.webhooks} {
		f(c)
	}
	m.slotsTotal.Set(float64(slots))
	return m
}

// Handler serves the metrics in the Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

// ObserveRPC records one RPC. procedure is the registered path, never
// caller input, so label cardinality stays bounded.
func (m *Metrics) ObserveRPC(procedure, protocol, code string, d time.Duration) {
	m.requests.WithLabelValues(procedure, protocol, code).Inc()
	m.latency.WithLabelValues(procedure).Observe(d.Seconds())
}

// ConversionDone implements convert.Observer.
func (m *Metrics) ConversionDone(input, output, outcome string, d time.Duration, bytes int) {
	m.conversions.WithLabelValues(input, output, outcome).Inc()
	m.convTime.WithLabelValues(input, output).Observe(d.Seconds())
	if outcome == "ok" {
		m.outBytes.Observe(float64(bytes))
	}
}

// ResourceBlocked implements convert.Observer.
func (m *Metrics) ResourceBlocked(reason string) { m.blocked.WithLabelValues(reason).Inc() }

// SlotsInUse implements convert.Observer.
func (m *Metrics) SlotsInUse(n int) { m.slotsInUse.Set(float64(n)) }

// QueueDepth implements jobs.Observer.
func (m *Metrics) QueueDepth(n int) { m.queueDepth.Set(float64(n)) }

// WebhookDone implements jobs.Observer.
func (m *Metrics) WebhookDone(outcome string) { m.webhooks.WithLabelValues(outcome).Inc() }
