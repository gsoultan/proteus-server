// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package auth checks API keys and applies per-key rate limits.
//
// Keys are never stored in plain text: the server is configured with
// "<id>:<sha256-hex>" entries and compares the SHA-256 of the presented key.
// Keys are high-entropy random strings, so an unsalted fast hash is enough.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

// Key is one configured API key.
type Key struct {
	ID   string
	Hash [32]byte
}

// ParseKeys parses comma or newline separated "<id>:<sha256-hex>" entries.
func ParseKeys(spec string) ([]Key, error) {
	var keys []Key
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		f = strings.TrimSpace(f)
		if f == "" || strings.HasPrefix(f, "#") {
			continue
		}
		id, h, ok := strings.Cut(f, ":")
		if !ok || id == "" {
			return nil, fmt.Errorf("auth: key entry %q must be <id>:<sha256-hex>", redact(f))
		}
		raw, err := hex.DecodeString(strings.TrimSpace(h))
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("auth: key %q: hash must be 64 hex characters", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("auth: duplicate key id %q", id)
		}
		seen[id] = true
		k := Key{ID: id}
		copy(k.Hash[:], raw)
		keys = append(keys, k)
	}
	return keys, nil
}

func redact(s string) string {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i] + ":…"
	}
	return "…"
}

// HashKey returns the hex SHA-256 of a plain API key, for configuration.
func HashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// NewKey returns a random API key with 256 bits of entropy.
func NewKey() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return "pk_" + base64.RawURLEncoding.EncodeToString(b[:])
}

// ErrUnauthorized means no valid key was presented.
var ErrUnauthorized = errors.New("missing or invalid API key")

// ErrRateLimited means the key exceeded its rate.
var ErrRateLimited = errors.New("rate limit exceeded")

// Authenticator validates keys and holds one limiter per key.
type Authenticator struct {
	keys  []Key
	rps   rate.Limit
	burst int

	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

// New returns an Authenticator. rps <= 0 disables rate limiting.
// With no keys, every request is accepted as the "anonymous" caller.
func New(keys []Key, rps float64, burst int) *Authenticator {
	if burst <= 0 {
		burst = 1
	}
	lim := rate.Limit(rps)
	if rps <= 0 {
		lim = rate.Inf
	}
	return &Authenticator{keys: keys, rps: lim, burst: burst, limiters: map[string]*rate.Limiter{}}
}

// Enabled reports whether keys are required.
func (a *Authenticator) Enabled() bool { return len(a.keys) > 0 }

// Anonymous is the caller id used when authentication is disabled.
const Anonymous = "anonymous"

// Check returns the key id for the presented secret and consumes one token
// from that key's rate limiter.
func (a *Authenticator) Check(secret string) (string, error) {
	id := Anonymous
	if a.Enabled() {
		if secret == "" {
			return "", ErrUnauthorized
		}
		h := sha256.Sum256([]byte(secret))
		id = ""
		for _, k := range a.keys {
			if subtle.ConstantTimeCompare(h[:], k.Hash[:]) == 1 {
				id = k.ID
			}
		}
		if id == "" {
			return "", ErrUnauthorized
		}
	}
	if !a.limiter(id).Allow() {
		return id, ErrRateLimited
	}
	return id, nil
}

func (a *Authenticator) limiter(id string) *rate.Limiter {
	a.mu.Lock()
	defer a.mu.Unlock()
	l, ok := a.limiters[id]
	if !ok {
		l = rate.NewLimiter(a.rps, a.burst)
		a.limiters[id] = l
	}
	return l
}

type ctxKey struct{}

// WithCaller stores the caller id in ctx.
func WithCaller(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Caller returns the caller id stored by WithCaller.
func Caller(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
