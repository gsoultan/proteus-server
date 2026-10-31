// Package httpapi is the HTTP transport: routing, authentication, request
// limits, logging and metrics around the convert and jobs packages.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/metrics"
)

// Version is reported by /healthz; set at build time with -ldflags.
var Version = "dev"

// Config for the HTTP layer.
type Config struct {
	MaxBodyBytes int64 // request body limit; default 20 MiB
	MaxAssets    int   // uploaded asset files per request; default 64
}

// API wires handlers to their dependencies.
type API struct {
	cfg     Config
	svc     *convert.Service
	jobs    *jobs.Manager
	auth    *auth.Authenticator
	metrics *metrics.Metrics
	log     *slog.Logger
	ready   atomic.Bool
}

// New builds the API. Call SetReady(true) once startup is complete.
func New(cfg Config, svc *convert.Service, jm *jobs.Manager, a *auth.Authenticator, m *metrics.Metrics, log *slog.Logger) *API {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 20 << 20
	}
	if cfg.MaxAssets <= 0 {
		cfg.MaxAssets = 64
	}
	return &API{cfg: cfg, svc: svc, jobs: jm, auth: a, metrics: m, log: log}
}

// SetReady flips the readiness probe; set false at the start of shutdown so
// the load balancer stops sending traffic before the listener closes.
func (a *API) SetReady(v bool) { a.ready.Store(v) }

// Handler returns the root handler.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, h http.HandlerFunc, authed bool) {
		var hh http.Handler = h
		if authed {
			hh = a.requireKey(hh)
		}
		mux.Handle(pattern, a.observe(pattern, hh))
	}
	route("POST /v1/convert", a.handleConvert, true)
	route("POST /v1/jobs", a.handleSubmitJob, true)
	route("GET /v1/jobs/{id}", a.handleGetJob, true)
	route("GET /v1/jobs/{id}/result", a.handleJobResult, true)
	route("DELETE /v1/jobs/{id}", a.handleDeleteJob, true)
	route("GET /healthz", a.handleHealth, false)
	route("GET /readyz", a.handleReady, false)
	mux.Handle("GET /metrics", a.metrics.Handler())
	mux.Handle("/", a.observe("unmatched", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "no such endpoint")
	})))
	return a.requestID(a.recoverer(mux))
}

// --- middleware ---

type ctxKey int

const (
	requestIDKey ctxKey = iota + 1
	logInfoKey
)

// logInfo is filled in by inner middleware (the caller id is only known after
// authentication) and read by observe when it writes the access log.
type logInfo struct{ caller string }

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func (a *API) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validRequestID.MatchString(id) {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func (a *API) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				a.log.Error("handler panic", "request_id", requestIDFrom(r.Context()), "panic", v)
				writeError(w, r, http.StatusInternalServerError, "internal", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	code  int
	bytes int
}

func (s *statusWriter) WriteHeader(code int) {
	if s.code == 0 {
		s.code = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if s.code == 0 {
		s.code = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// observe logs each request and records RED metrics under a fixed route
// label (the pattern, never the raw path, to keep label cardinality bounded).
func (a *API) observe(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		info := &logInfo{}
		next.ServeHTTP(sw, r.WithContext(context.WithValue(r.Context(), logInfoKey, info)))
		if sw.code == 0 {
			sw.code = http.StatusOK
		}
		d := time.Since(start)
		a.metrics.ObserveRequest(route, r.Method, strconv.Itoa(sw.code), d)
		level := slog.LevelInfo
		if sw.code >= 500 {
			level = slog.LevelError
		}
		if route == "GET /healthz" || route == "GET /readyz" {
			level = slog.LevelDebug
		}
		a.log.Log(r.Context(), level, "request",
			"request_id", requestIDFrom(r.Context()),
			"route", route, "method", r.Method, "status", sw.code,
			"bytes", sw.bytes, "duration_ms", d.Milliseconds(),
			"caller", info.caller, "remote", r.RemoteAddr)
	})
}

func (a *API) requireKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := r.Header.Get("X-API-Key")
		if h := r.Header.Get("Authorization"); secret == "" && h != "" {
			if v, ok := strings.CutPrefix(h, "Bearer "); ok {
				secret = strings.TrimSpace(v)
			}
		}
		id, err := a.auth.Check(secret)
		switch {
		case errors.Is(err, auth.ErrUnauthorized):
			w.Header().Set("WWW-Authenticate", `Bearer realm="proteus"`)
			writeError(w, r, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		case errors.Is(err, auth.ErrRateLimited):
			w.Header().Set("Retry-After", "1")
			writeError(w, r, http.StatusTooManyRequests, "rate_limited", err.Error())
			return
		case err != nil:
			writeError(w, r, http.StatusInternalServerError, "internal", "internal error")
			return
		}
		if info, ok := r.Context().Value(logInfoKey).(*logInfo); ok {
			info.caller = id
		}
		next.ServeHTTP(w, r.WithContext(auth.WithCaller(r.Context(), id)))
	})
}

// --- responses ---

type errorBody struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, code int, kind, msg string) {
	writeJSON(w, code, errorBody{Error: kind, Message: msg, RequestID: requestIDFrom(r.Context())})
}
