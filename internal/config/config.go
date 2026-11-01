// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package config reads server settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is every server setting. See README for the variable names.
type Config struct {
	Addr            string // RPC listener (gRPC, gRPC-Web, Connect)
	AdminAddr       string // Prometheus /metrics listener; empty disables
	ShutdownTimeout time.Duration
	LogLevel        string
	LogFormat       string // json or text

	APIKeys         string // "<id>:<sha256-hex>" entries
	AllowNoAuth     bool   // run without API keys (local development only)
	RateLimit       float64
	RateBurst       int
	MaxRequestBytes int64
	MaxAssets       int
	Workers         int
	Timeout         time.Duration
	MaxOutput       int64
	FontDirs        []string
	SystemFonts     bool
	AllowRemote     bool
	AllowedHosts    []string
	FetchTimeout    time.Duration
	MaxResource     int64
	MaxResources    int
	MaxTotalBytes   int64

	JobWorkers    int
	JobQueue      int
	JobTTL        time.Duration
	MaxJobs       int
	WebhookSecret string

	DatabaseURL    string // Postgres for the shared job queue; empty keeps jobs in memory
	DatabaseSchema string // schema holding the jobs table; default public
	AutoMigrate    bool   // create/update the jobs table at startup
}

// FromEnv reads the configuration, using getenv (os.Getenv when nil).
func FromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	p := parser{get: getenv}
	c := Config{
		Addr:            p.str("PROTEUS_ADDR", ":8080"),
		AdminAddr:       p.str("PROTEUS_ADMIN_ADDR", ":9090"),
		ShutdownTimeout: p.dur("PROTEUS_SHUTDOWN_TIMEOUT", 30*time.Second),
		LogLevel:        p.str("PROTEUS_LOG_LEVEL", "info"),
		LogFormat:       p.str("PROTEUS_LOG_FORMAT", "json"),
		APIKeys:         p.str("PROTEUS_API_KEYS", ""),
		AllowNoAuth:     p.bool("PROTEUS_ALLOW_NO_AUTH", false),
		RateLimit:       p.float("PROTEUS_RATE_LIMIT", 10),
		RateBurst:       p.int("PROTEUS_RATE_BURST", 20),
		MaxRequestBytes: p.bytes("PROTEUS_MAX_REQUEST_BYTES", 20<<20),
		MaxAssets:       p.int("PROTEUS_MAX_ASSETS", 64),
		Workers:         p.int("PROTEUS_WORKERS", 0),
		Timeout:         p.dur("PROTEUS_TIMEOUT", 30*time.Second),
		MaxOutput:       p.bytes("PROTEUS_MAX_OUTPUT_BYTES", 100<<20),
		FontDirs:        p.list("PROTEUS_FONT_DIRS"),
		SystemFonts:     p.bool("PROTEUS_SYSTEM_FONTS", true),
		AllowRemote:     p.bool("PROTEUS_ALLOW_REMOTE", false),
		AllowedHosts:    p.list("PROTEUS_ALLOWED_HOSTS"),
		FetchTimeout:    p.dur("PROTEUS_FETCH_TIMEOUT", 10*time.Second),
		MaxResource:     p.bytes("PROTEUS_MAX_RESOURCE_BYTES", 10<<20),
		MaxResources:    p.int("PROTEUS_MAX_RESOURCES", 256),
		MaxTotalBytes:   p.bytes("PROTEUS_MAX_TOTAL_RESOURCE_BYTES", 50<<20),
		JobWorkers:      p.int("PROTEUS_JOB_WORKERS", 0),
		JobQueue:        p.int("PROTEUS_JOB_QUEUE", 100),
		JobTTL:          p.dur("PROTEUS_JOB_TTL", time.Hour),
		MaxJobs:         p.int("PROTEUS_MAX_JOBS", 1000),
		WebhookSecret:   p.str("PROTEUS_WEBHOOK_SECRET", ""),
		DatabaseURL:     p.str("PROTEUS_DATABASE_URL", ""),
		DatabaseSchema:  p.str("PROTEUS_DATABASE_SCHEMA", "public"),
		AutoMigrate:     p.bool("PROTEUS_AUTO_MIGRATE", true),
	}
	if f := getenv("PROTEUS_API_KEYS_FILE"); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			p.errs = append(p.errs, fmt.Errorf("PROTEUS_API_KEYS_FILE: %w", err))
		} else {
			c.APIKeys = strings.TrimSpace(c.APIKeys + "\n" + string(b))
		}
	}
	if len(p.errs) > 0 {
		return c, errors.Join(p.errs...)
	}
	if c.AdminAddr == "off" {
		c.AdminAddr = ""
	}
	if c.WebhookSecret != "" && len(c.WebhookSecret) < 32 {
		return c, errors.New("PROTEUS_WEBHOOK_SECRET must be at least 32 characters")
	}
	return c, nil
}

type parser struct {
	get  func(string) string
	errs []error
}

func (p *parser) str(k, def string) string {
	if v := strings.TrimSpace(p.get(k)); v != "" {
		return v
	}
	return def
}

func (p *parser) int(k string, def int) int {
	v := strings.TrimSpace(p.get(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: want a non-negative integer, got %q", k, v))
		return def
	}
	return n
}

func (p *parser) float(k string, def float64) float64 {
	v := strings.TrimSpace(p.get(k))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: want a non-negative number, got %q", k, v))
		return def
	}
	return f
}

func (p *parser) bool(k string, def bool) bool {
	v := strings.TrimSpace(p.get(k))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: want true or false, got %q", k, v))
		return def
	}
	return b
}

func (p *parser) dur(k string, def time.Duration) time.Duration {
	v := strings.TrimSpace(p.get(k))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: want a positive duration like 30s, got %q", k, v))
		return def
	}
	return d
}

// bytes accepts plain integers or KiB/MiB/GiB suffixes (also K, M, G).
func (p *parser) bytes(k string, def int64) int64 {
	v := strings.ToUpper(strings.TrimSpace(p.get(k)))
	if v == "" {
		return def
	}
	mult := int64(1)
	for _, s := range []struct {
		suf string
		m   int64
	}{{"KIB", 1 << 10}, {"MIB", 1 << 20}, {"GIB", 1 << 30}, {"K", 1 << 10}, {"M", 1 << 20}, {"G", 1 << 30}} {
		if strings.HasSuffix(v, s.suf) {
			v, mult = strings.TrimSpace(strings.TrimSuffix(v, s.suf)), s.m
			break
		}
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: want a size like 20MiB, got %q", k, p.get(k)))
		return def
	}
	return n * mult
}

func (p *parser) list(k string) []string {
	var out []string
	for _, f := range strings.Split(p.get(k), ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
