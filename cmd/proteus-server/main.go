// Command proteus-server serves document conversion (today HTML and Markdown
// to PDF) over gRPC (ConnectRPC: gRPC, gRPC-Web and the Connect protocol).
//
//	proteus-server              run the server (configured by PROTEUS_* variables)
//	proteus-server keygen ID    print a new API key and its PROTEUS_API_KEYS entry
//	proteus-server hash KEY     print the SHA-256 of an existing key
//	proteus-server healthcheck  gRPC health check against PROTEUS_ADDR (for Docker)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"

	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/config"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/rpc"
	"github.com/gsoultan/proteus-server/internal/safefetch"
	"github.com/gsoultan/proteus-server/internal/storage/pgstore"
	"github.com/gsoultan/proteus-server/internal/templates"
)

func main() {
	if len(os.Args) > 1 {
		os.Exit(subcommand(os.Args[1:]))
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "proteus-server:", err)
		os.Exit(1)
	}
}

func subcommand(args []string) int {
	switch args[0] {
	case "keygen":
		id := "default"
		if len(args) > 1 {
			id = args[1]
		}
		key := auth.NewKey()
		fmt.Printf("API key (give to the client, shown once): %s\n", key)
		fmt.Printf("PROTEUS_API_KEYS entry:                    %s:%s\n", id, auth.HashKey(key))
		return 0
	case "hash":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: proteus-server hash KEY")
			return 2
		}
		fmt.Println(auth.HashKey(args[1]))
		return 0
	case "healthcheck":
		if err := healthcheck(os.Getenv("PROTEUS_ADDR")); err != nil {
			fmt.Fprintln(os.Stderr, "unhealthy:", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown command %q (keygen, hash, healthcheck)\n", args[0])
	return 2
}

// h2cClient speaks HTTP/2 without TLS, which gRPC needs on a plain listener.
func h2cClient(timeout time.Duration) *http.Client {
	p := new(http.Protocols)
	p.SetUnencryptedHTTP2(true)
	return &http.Client{Timeout: timeout, Transport: &http.Transport{Protocols: p}}
}

func healthcheck(addr string) error {
	switch {
	case addr == "":
		addr = "127.0.0.1:8080"
	case strings.HasPrefix(addr, ":"):
		addr = "127.0.0.1" + addr
	}
	client := grpchealth.NewClient(connect.NewClient(
		connecthttp.NewTransport(h2cClient(3*time.Second), "http://"+addr, connecthttp.WithGRPC())))
	res, err := client.Check(context.Background(), &grpchealth.CheckRequest{})
	if err != nil {
		return err
	}
	if res.Status != grpchealth.StatusServing {
		return fmt.Errorf("status %s", res.Status)
	}
	return nil
}

func run() error {
	cfg, err := config.FromEnv(nil)
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	slog.SetDefault(log)

	keys, err := auth.ParseKeys(cfg.APIKeys)
	if err != nil {
		return err
	}
	if len(keys) == 0 && !cfg.AllowNoAuth {
		return errors.New("no API keys configured: set PROTEUS_API_KEYS (see `proteus-server keygen`) " +
			"or PROTEUS_ALLOW_NO_AUTH=true for local development")
	}
	if len(keys) == 0 {
		log.Warn("authentication disabled (PROTEUS_ALLOW_NO_AUTH=true); do not expose this server")
	}

	workers := cfg.Workers
	if workers == 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	jobWorkers := cfg.JobWorkers
	if jobWorkers == 0 {
		jobWorkers = max(1, workers/2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var (
		store    jobs.Store
		tplStore templates.Store
		pingDB   func(context.Context) error
	)
	if cfg.DatabaseURL != "" {
		octx, cancel := context.WithTimeout(ctx, 60*time.Second)
		pg, err := pgstore.Open(octx, cfg.DatabaseURL, cfg.DatabaseSchema, cfg.AutoMigrate, func(f string, a ...any) {
			log.Info("migrate: " + fmt.Sprintf(f, a...))
		})
		cancel()
		if err != nil {
			return err
		}
		defer pg.Close()
		store, tplStore, pingDB = pg, pg.Templates(), pg.Ping
		log.Info("job and template store: postgres", "auto_migrate", cfg.AutoMigrate)
	} else {
		store, tplStore = jobs.NewMemoryStore(cfg.MaxJobs), templates.NewMemoryStore()
		log.Warn("job and template store: memory; templates are lost on restart " +
			"(set PROTEUS_DATABASE_URL to keep them and share jobs across instances)")
	}

	m := metrics.New(workers)
	fetch := safefetch.New(safefetch.Options{
		Timeout: cfg.FetchTimeout, MaxBytes: cfg.MaxResource, AllowedHosts: cfg.AllowedHosts,
		UserAgent: "proteus-server/" + rpc.Version,
	})
	svc := convert.NewService(convert.Settings{
		Workers:        workers,
		Timeout:        cfg.Timeout,
		MaxOutputBytes: cfg.MaxOutput,
		AllowRemote:    cfg.AllowRemote,
		Remote:         fetch,
		LoaderLimits:   loader.Limits{MaxResources: cfg.MaxResources, MaxTotal: cfg.MaxTotalBytes, MaxEach: cfg.MaxResource},
		FontDirs:       cfg.FontDirs,
		SystemFonts:    cfg.SystemFonts,
		Logger:         log,
		Metrics:        m,
	})
	var secret []byte
	if cfg.WebhookSecret != "" {
		secret = []byte(cfg.WebhookSecret)
	}
	jm := jobs.NewManager(svc, store, jobs.Settings{
		Workers: jobWorkers, QueueSize: int64(cfg.JobQueue), ResultTTL: cfg.JobTTL, MaxAssets: cfg.MaxAssets,
		WebhookSecret: secret,
		Webhook:       safefetch.New(safefetch.Options{Timeout: 10 * time.Second, MaxBytes: 1 << 20, UserAgent: "proteus-server/" + rpc.Version}),
		Logger:        log, Observer: m,
	})
	ts := templates.NewService(tplStore, templates.Settings{
		MaxPerOwner: cfg.MaxTemplates, MaxAssets: cfg.MaxAssets, MaxBytes: cfg.MaxTemplateBytes,
		MaxOutput: cfg.MaxRequestBytes, RenderTimeout: cfg.TemplateRenderTimeout,
	})
	api := rpc.New(rpc.Config{MaxRequestBytes: cfg.MaxRequestBytes, MaxAssets: cfg.MaxAssets, MaxOutputBytes: cfg.MaxOutput},
		svc, jm, ts, auth.New(keys, cfg.RateLimit, cfg.RateBurst), m, log, pingDB)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)            // Connect and gRPC-Web from browsers
	protocols.SetUnencryptedHTTP2(true) // gRPC over h2c; terminate TLS at the load balancer
	protocols.SetHTTP2(true)
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      cfg.Timeout + 60*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	errc := make(chan error, 2)
	go func() { errc <- srv.Serve(ln) }()

	var admin *http.Server
	if cfg.AdminAddr != "" {
		mux := http.NewServeMux()
		mux.Handle("GET /metrics", m.Handler())
		admin = &http.Server{Addr: cfg.AdminAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		aln, err := net.Listen("tcp", cfg.AdminAddr)
		if err != nil {
			return err
		}
		go func() { errc <- admin.Serve(aln) }()
	}

	// Warm up before reporting healthy: the first conversion scans the
	// system fonts, which would otherwise land on the first caller.
	if err := warmUp(ctx, svc); err != nil {
		log.Warn("warm-up conversion failed", "error", err)
	}
	api.SetReady(true)
	log.Info("listening", "addr", ln.Addr().String(), "admin", cfg.AdminAddr, "workers", workers,
		"job_workers", jobWorkers, "remote_resources", cfg.AllowRemote, "auth", len(keys) > 0, "version", rpc.Version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	api.SetReady(false)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	httpErr := srv.Shutdown(sctx)
	jobErr := jm.Shutdown(sctx)
	var adminErr error
	if admin != nil {
		adminErr = admin.Shutdown(sctx)
	}
	if err := errors.Join(httpErr, jobErr, adminErr); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}

func warmUp(ctx context.Context, svc *convert.Service) error {
	release, err := svc.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()
	_, err = svc.Run(ctx, &convert.Request{Format: convert.Markdown, Output: convert.PDF, Source: []byte("# warm-up")})
	return err
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}
