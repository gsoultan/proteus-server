// Command proteus-server serves HTML and Markdown to PDF conversion over HTTP.
//
//	proteus-server              run the server (configured by PROTEUS_* variables)
//	proteus-server keygen ID    print a new API key and its PROTEUS_API_KEYS entry
//	proteus-server hash KEY     print the SHA-256 of an existing key
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

	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/config"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/httpapi"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/safefetch"
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
		// For container HEALTHCHECK in images without curl.
		addr := os.Getenv("PROTEUS_ADDR")
		switch {
		case addr == "":
			addr = "127.0.0.1:8080"
		case strings.HasPrefix(addr, ":"):
			addr = "127.0.0.1" + addr
		}
		c := http.Client{Timeout: 3 * time.Second}
		resp, err := c.Get("http://" + addr + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			return 1
		}
		resp.Body.Close()
		return 0
	}
	fmt.Fprintf(os.Stderr, "unknown command %q (keygen, hash, healthcheck)\n", args[0])
	return 2
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

	m := metrics.New(workers)
	fetch := safefetch.New(safefetch.Options{
		Timeout: cfg.FetchTimeout, MaxBytes: cfg.MaxResource, AllowedHosts: cfg.AllowedHosts,
		UserAgent: "proteus-server/" + httpapi.Version,
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
	jm := jobs.NewManager(svc, jobs.Settings{
		Workers: jobWorkers, QueueSize: cfg.JobQueue, ResultTTL: cfg.JobTTL, MaxJobs: cfg.MaxJobs,
		WebhookSecret: secret,
		Webhook:       safefetch.New(safefetch.Options{Timeout: 10 * time.Second, MaxBytes: 1 << 20, UserAgent: "proteus-server/" + httpapi.Version}),
		Logger:        log, Observer: m,
	})
	api := httpapi.New(httpapi.Config{MaxBodyBytes: cfg.MaxBodyBytes, MaxAssets: cfg.MaxAssets},
		svc, jm, auth.New(keys, cfg.RateLimit, cfg.RateBurst), m, log)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	// Warm up before reporting ready: the first conversion scans the system
	// fonts, which would otherwise land on the first caller.
	if err := warmUp(ctx, svc); err != nil {
		log.Warn("warm-up conversion failed", "error", err)
	}
	api.SetReady(true)
	log.Info("listening", "addr", ln.Addr().String(), "workers", workers, "job_workers", jobWorkers,
		"remote_resources", cfg.AllowRemote, "auth", len(keys) > 0, "version", httpapi.Version)

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
	if err := errors.Join(httpErr, jobErr); err != nil {
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
	_, err = svc.Run(ctx, &convert.Request{Format: "markdown", Source: []byte("# warm-up")})
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
