// Package rpc is the transport: the ConnectRPC ConversionService and
// TemplateService plus gRPC health and reflection, behind interceptors for
// request ids, logging, metrics, panic recovery and API keys. The same handlers answer gRPC,
// gRPC-Web and the Connect protocol; there is no REST API.
package rpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"
	"connectrpc.com/grpcreflect/v2"

	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/templates"
)

// Version is reported in logs; set at build time with -ldflags.
var Version = "dev"

// Config for the RPC layer.
type Config struct {
	MaxRequestBytes int64 // largest request message; default 20 MiB
	MaxAssets       int   // assets per document; default 64
	MaxOutputBytes  int64 // largest response message (sync PDF); default 100 MiB
}

// API wires the service to its dependencies.
type API struct {
	cfg       Config
	svc       *convert.Service
	jobs      *jobs.Manager
	templates *templates.Service
	auth      *auth.Authenticator
	metrics   *metrics.Metrics
	log       *slog.Logger
	ready     atomic.Bool
	pingDB    func(context.Context) error
}

// New builds the API. pingDB, when set, is part of the health check.
func New(cfg Config, svc *convert.Service, jm *jobs.Manager, ts *templates.Service, a *auth.Authenticator,
	m *metrics.Metrics, log *slog.Logger, pingDB func(context.Context) error) *API {
	if cfg.MaxRequestBytes <= 0 {
		cfg.MaxRequestBytes = 20 << 20
	}
	if cfg.MaxAssets <= 0 {
		cfg.MaxAssets = 64
	}
	if cfg.MaxOutputBytes <= 0 {
		cfg.MaxOutputBytes = 100 << 20
	}
	return &API{cfg: cfg, svc: svc, jobs: jm, templates: ts, auth: a, metrics: m, log: log, pingDB: pingDB}
}

// SetReady flips the health status; set false at the start of shutdown so
// load balancers stop sending traffic before the listener drains.
func (a *API) SetReady(v bool) { a.ready.Store(v) }

// Handler returns the RPC handler: the conversion and template services, gRPC health
// (grpc.health.v1) and reflection (grpc.reflection.v1 and v1alpha).
func (a *API) Handler() http.Handler {
	server := connect.NewServer(a.observe, a.recoverer, a.authenticate)
	conv := &conversionService{api: a}
	proteusv1connect.RegisterConversionServiceHandler(server, conv)
	proteusv1connect.RegisterTemplateServiceHandler(server, &templateService{conv: conv})
	grpchealth.Register(server, healthChecker{a})
	grpcreflect.Register(server)
	mux := http.NewServeMux()
	connecthttp.Mount(mux, server,
		connecthttp.WithReadMaxBytes(int(a.cfg.MaxRequestBytes)),
		connecthttp.WithSendMaxBytes(int(a.cfg.MaxOutputBytes)+(1<<20)),
		connecthttp.WithCompressMinBytes(1<<10),
	)
	return mux
}

// --- health ---

type healthChecker struct{ a *API }

func (h healthChecker) Check(ctx context.Context, req *grpchealth.CheckRequest) (*grpchealth.CheckResponse, error) {
	switch req.Service {
	case "", proteusv1connect.ConversionServiceName, proteusv1connect.TemplateServiceName:
	default:
		return nil, connect.NewError(connect.CodeNotFound, "unknown service")
	}
	if !h.a.ready.Load() {
		return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
	}
	if h.a.pingDB != nil {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := h.a.pingDB(ctx); err != nil {
			return &grpchealth.CheckResponse{Status: grpchealth.StatusNotServing}, nil
		}
	}
	return &grpchealth.CheckResponse{Status: grpchealth.StatusServing}, nil
}

// --- interceptors ---

type ctxKey int

const (
	requestIDKey ctxKey = iota + 1
	logInfoKey
)

// logInfo is filled in by inner interceptors (the caller is only known after
// authentication) and read by observe when it writes the access log.
type logInfo struct{ caller string }

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// observe assigns a request id, then logs and records metrics for every RPC.
func (a *API) observe(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		start := time.Now()
		info, _ := connect.CallInfoForServerContext(ctx)
		id := ""
		protocol := ""
		if info != nil {
			id = info.RequestHeader().Get("X-Request-Id")
			protocol = info.Protocol
		}
		if !validRequestID.MatchString(id) {
			var b [8]byte
			_, _ = rand.Read(b[:])
			id = hex.EncodeToString(b[:])
		}
		if info != nil {
			info.ResponseHeader().Set("X-Request-Id", id)
		}
		li := &logInfo{}
		ctx = context.WithValue(context.WithValue(ctx, requestIDKey, id), logInfoKey, li)
		err := next(ctx, spec, stream)
		code := "ok"
		if err != nil {
			code = connect.CodeOf(err).String()
		}
		d := time.Since(start)
		a.metrics.ObserveRPC(spec.Procedure, protocol, code, d)
		level := slog.LevelInfo
		switch {
		case isOps(spec.Procedure):
			level = slog.LevelDebug
		case err != nil && serverFault(connect.CodeOf(err)):
			level = slog.LevelError
		}
		attrs := []any{"request_id", id, "procedure", spec.Procedure, "protocol", protocol,
			"code", code, "duration_ms", d.Milliseconds(), "caller", li.caller}
		if info != nil {
			attrs = append(attrs, "peer", info.PeerAddr)
		}
		if err != nil {
			attrs = append(attrs, "error", err)
		}
		a.log.Log(ctx, level, "rpc", attrs...)
		return err
	}
}

func serverFault(c connect.Code) bool {
	switch c {
	case connect.CodeInternal, connect.CodeUnknown, connect.CodeDataLoss, connect.CodeUnavailable:
		return true
	}
	return false
}

func (a *API) recoverer(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) (err error) {
		defer func() {
			if r := recover(); r != nil {
				a.log.Error("handler panic", "request_id", requestIDFrom(ctx), "procedure", spec.Procedure, "panic", r)
				err = connect.NewError(connect.CodeInternal, "internal error")
			}
		}()
		return next(ctx, spec, stream)
	}
}

// isOps reports whether a procedure is health or reflection, which need no key.
func isOps(procedure string) bool {
	return strings.HasPrefix(procedure, "/grpc.health.") || strings.HasPrefix(procedure, "/grpc.reflection.")
}

// authenticate checks the API key ("authorization: Bearer <key>" or
// "x-api-key") and the key's rate limit.
func (a *API) authenticate(next connect.ServerFunc) connect.ServerFunc {
	return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		if isOps(spec.Procedure) {
			return next(ctx, spec, stream)
		}
		info, ok := connect.CallInfoForServerContext(ctx)
		if !ok {
			return connect.NewError(connect.CodeUnauthenticated, auth.ErrUnauthorized.Error())
		}
		h := info.RequestHeader()
		secret := h.Get("X-Api-Key")
		if v, ok := strings.CutPrefix(h.Get("Authorization"), "Bearer "); secret == "" && ok {
			secret = strings.TrimSpace(v)
		}
		id, err := a.auth.Check(secret)
		switch {
		case errors.Is(err, auth.ErrUnauthorized):
			return connect.NewError(connect.CodeUnauthenticated, err.Error())
		case errors.Is(err, auth.ErrRateLimited):
			return connect.NewError(connect.CodeResourceExhausted, err.Error())
		case err != nil:
			return connect.NewError(connect.CodeInternal, "internal error").WithCause(err)
		}
		if li, ok := ctx.Value(logInfoKey).(*logInfo); ok {
			li.caller = id
		}
		return next(auth.WithCaller(ctx, id), spec, stream)
	}
}
