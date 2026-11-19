// Command proteus-server serves document conversion (today HTML and Markdown
// to PDF) over gRPC (ConnectRPC: gRPC, gRPC-Web and the Connect protocol).
//
//	proteus-server              run the server (first run: open /setup in a browser;
//	                            PROTEUS_* variables override what the web UI saves)
//	proteus-server keygen ID    print a new API key and its PROTEUS_API_KEYS entry
//	proteus-server hash KEY     print the SHA-256 of an existing key
//	proteus-server healthcheck  gRPC health check against PROTEUS_ADDR (for Docker)
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/config"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/rpc"
	"github.com/gsoultan/proteus-server/internal/safefetch"
	"github.com/gsoultan/proteus-server/internal/settings"
	"github.com/gsoultan/proteus-server/internal/storage/pgstore"
	"github.com/gsoultan/proteus-server/internal/templates"
)

func main() {
	if len(os.Args) > 1 {
		os.Exit(subcommand(os.Args[1:]))
	}
	if err := serve(); err != nil {
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

// errRestart ends run so serve starts it again with fresh settings.
var errRestart = errors.New("restart to apply settings")

// kept survives in-place restarts: the in-memory stores (so saving a setting
// does not lose jobs and templates when there is no database) and the setup
// token (so the one in the log stays valid).
type kept struct {
	jobs       jobs.Store
	templates  templates.Store
	setupToken string
}

// serve runs the server, restarting it in place whenever the web UI saves
// settings or setup completes.
func serve() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var k kept
	for {
		err := run(ctx, &k)
		if !errors.Is(err, errRestart) {
			return err
		}
		slog.Info("restarting to apply settings")
	}
}

// pgDatabase adapts pgstore.Store for the setup wizard.
type pgDatabase struct{ *pgstore.Store }

func (d pgDatabase) Settings() settings.Store { return d.Store.Settings() }

func openDatabase(ctx context.Context, cfg config.Config, migrate bool, log *slog.Logger) (*pgstore.Store, error) {
	return pgstore.Open(ctx, cfg.DatabaseURL, cfg.DatabaseSchema, migrate && cfg.AutoMigrate, func(f string, a ...any) {
		log.Info("migrate: " + fmt.Sprintf(f, a...))
	})
}

func run(parent context.Context, k *kept) error {
	// Settings come in layers: the environment, then the bootstrap file the
	// setup wizard wrote (the database), then what the web UI saved.
	dataDir := strings.TrimSpace(os.Getenv("PROTEUS_DATA_DIR"))
	if dataDir == "" {
		dataDir = "data"
	}
	bootstrap, err := settings.ReadBootstrap(dataDir)
	if err != nil {
		return err
	}
	boot, err := config.FromEnv(config.Layered(os.Getenv, bootstrap))
	if err != nil {
		return err
	}
	log := newLogger(boot)
	slog.SetDefault(log)

	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)

	var (
		store     jobs.Store
		tplStore  templates.Store
		setStore  settings.Store
		pingDB    func(context.Context) error
		storage   = proteusv1.Storage_STORAGE_MEMORY
		fileStore = func() settings.Store { return settings.NewFileStore(dataDir) }
	)
	if boot.DatabaseURL != "" {
		octx, ocancel := context.WithTimeout(ctx, 60*time.Second)
		pg, err := openDatabase(octx, boot, true, log)
		ocancel()
		if err != nil {
			return err
		}
		defer pg.Close()
		store, tplStore, setStore, pingDB = pg, pg.Templates(), pg.Settings(), pg.Ping
		storage = proteusv1.Storage_STORAGE_POSTGRES
		log.Info("job, template and settings store: postgres", "auto_migrate", boot.AutoMigrate)
	} else {
		if k.jobs == nil {
			k.jobs, k.templates = jobs.NewMemoryStore(boot.MaxJobs), templates.NewMemoryStore()
		}
		store, tplStore, setStore = k.jobs, k.templates, fileStore()
		log.Warn("job and template store: memory; templates are lost on restart "+
			"(choose PostgreSQL in /setup or set PROTEUS_DATABASE_URL to keep them and share jobs across instances)",
			"settings", filepath.Join(dataDir, settings.SettingsFile))
	}
	doc, err := setStore.Load(ctx)
	if err != nil {
		return err
	}
	cfg, err := config.FromEnv(config.Layered(os.Getenv, bootstrap, doc.Values))
	if err != nil {
		return fmt.Errorf("saved settings: %w", err)
	}

	envKeys, err := auth.ParseKeys(cfg.APIKeys)
	if err != nil {
		return err
	}
	uiKeys, err := setStore.ListKeys(ctx)
	if err != nil {
		return err
	}
	switch {
	case !doc.SetupDone():
		if k.setupToken == "" {
			k.setupToken = cfg.SetupToken
		}
		if k.setupToken == "" {
			k.setupToken = newSetupToken()
		}
		log.Warn("first run: open /setup in a browser to set up proteus-server", "setup_token", k.setupToken)
	case len(envKeys)+len(uiKeys) == 0 && !cfg.AllowNoAuth:
		log.Error("no API keys: every call is refused; set PROTEUS_API_KEYS with an admin key (`proteus-server keygen ops`, then add :admin)")
	case len(envKeys)+len(uiKeys) == 0:
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
	authn := auth.New(envKeys, cfg.RateLimit, cfg.RateBurst)
	api := rpc.New(rpc.Config{MaxRequestBytes: cfg.MaxRequestBytes, MaxAssets: cfg.MaxAssets, MaxOutputBytes: cfg.MaxOutput, AllowNoAuth: cfg.AllowNoAuth},
		svc, jm, ts, authn, m, log, pingDB)
	api.WithControl(&rpc.Control{
		Store: setStore, Env: os.Getenv, Bootstrap: bootstrap, DataDir: dataDir,
		Storage: storage, StorageFromEnv: strings.TrimSpace(os.Getenv("PROTEUS_DATABASE_URL")) != "",
		EnvKeys: envKeys, SetupDone: doc.SetupDone(), SetupToken: k.setupToken,
		OpenDatabase: func(ctx context.Context, values map[string]string, migrate bool) (rpc.Database, error) {
			c, err := config.FromEnv(config.Layered(func(string) string { return "" }, values))
			if err != nil {
				return nil, err
			}
			pg, err := openDatabase(ctx, c, migrate, log)
			if err != nil {
				return nil, err
			}
			return pgDatabase{pg}, nil
		},
		FileStore: fileStore,
		Restart:   func() { cancel(errRestart) },
	})
	authn.SetStored(rpcKeys(uiKeys))
	if storage == proteusv1.Storage_STORAGE_POSTGRES {
		go watchSettings(ctx, setStore, api, doc.Revision, func() { cancel(errRestart) }, log)
	}

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
		"job_workers", jobWorkers, "remote_resources", cfg.AllowRemote, "auth", authn.Enabled(), "version", rpc.Version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	restart := errors.Is(context.Cause(ctx), errRestart)
	if restart {
		log.Info("applying settings: stopping services")
	} else {
		log.Info("shutting down")
	}
	api.SetReady(false)
	sctx, scancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer scancel()
	httpErr := srv.Shutdown(sctx)
	jobErr := jm.Shutdown(sctx)
	var adminErr error
	if admin != nil {
		adminErr = admin.Shutdown(sctx)
	}
	if err := errors.Join(httpErr, jobErr, adminErr); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if restart {
		return errRestart
	}
	log.Info("stopped")
	return nil
}

// newSetupToken returns about 99 random bits as five groups of four letters and
// digits, easy to copy from a log.
func newSetupToken() string {
	var b [20]byte
	_, _ = rand.Read(b[:])
	const alphabet = "abcdefghjkmnpqrstuvwxyz23456789"
	var sb strings.Builder
	for i, c := range b {
		if i > 0 && i%4 == 0 {
			sb.WriteByte('-')
		}
		sb.WriteByte(alphabet[int(c)%len(alphabet)])
	}
	return sb.String()
}

func rpcKeys(keys []settings.APIKey) []auth.Key {
	out := make([]auth.Key, len(keys))
	for i, k := range keys {
		out[i] = auth.Key{ID: k.ID, Hash: k.Hash, Role: k.Role}
	}
	return out
}

// watchSettings keeps instances that share a database in step: it reloads
// the API keys and restarts when another instance saved new settings.
func watchSettings(ctx context.Context, store settings.Store, api *rpc.API, revision int64, restart func(), log *slog.Logger) {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if err := api.ReloadKeys(ctx); err != nil {
			log.Warn("reload api keys", "error", err)
		}
		d, err := store.Load(ctx)
		if err != nil {
			log.Warn("reload settings", "error", err)
			continue
		}
		if d.Revision != revision {
			log.Info("settings changed on another instance")
			restart()
			return
		}
	}
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
