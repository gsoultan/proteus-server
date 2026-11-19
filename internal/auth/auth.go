// Package auth checks API keys and applies per-key rate limits.
//
// Keys are never stored in plain text: the server is configured with
// "<id>:<sha256-hex>" entries and compares the SHA-256 of the presented key.
// Keys are high-entropy random strings, so an unsalted fast hash is enough.
// Keys created in the web UI are kept the same way (internal/settings).
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
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/time/rate"
)

// Roles. An admin key may also change settings and manage keys in the web UI.
const (
	RoleAdmin  = "admin"
	RoleClient = "client"
)

// Key is one configured API key.
type Key struct {
	ID   string
	Hash [32]byte
	Role string // RoleAdmin or RoleClient
}

// ParseKeys parses comma or newline separated "<id>:<sha256-hex>[:<role>]"
// entries. The role is client unless given as admin.
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
		role := RoleClient
		if hh, r, ok := strings.Cut(h, ":"); ok {
			h, role = hh, strings.TrimSpace(r)
			if role != RoleAdmin && role != RoleClient {
				return nil, fmt.Errorf("auth: key %q: role must be admin or client", id)
			}
		}
		raw, err := hex.DecodeString(strings.TrimSpace(h))
		if err != nil || len(raw) != 32 {
			return nil, fmt.Errorf("auth: key %q: hash must be 64 hex characters", id)
		}
		if seen[id] {
			return nil, fmt.Errorf("auth: duplicate key id %q", id)
		}
		seen[id] = true
		k := Key{ID: id, Role: role}
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

// Authenticator validates keys and holds one limiter per key. Keys from the
// environment are fixed; keys created in the web UI are swapped in with
// SetStored whenever they change.
type Authenticator struct {
	keys   []Key
	stored atomic.Pointer[[]Key]
	rps    rate.Limit
	burst  int

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

// SetStored replaces the keys created in the web UI.
func (a *Authenticator) SetStored(keys []Key) {
	keys = slices.Clone(keys)
	a.stored.Store(&keys)
}

func (a *Authenticator) storedKeys() []Key {
	if p := a.stored.Load(); p != nil {
		return *p
	}
	return nil
}

// Enabled reports whether keys are required.
func (a *Authenticator) Enabled() bool { return len(a.keys) > 0 || len(a.storedKeys()) > 0 }

// Anonymous is the caller id used when authentication is disabled.
const Anonymous = "anonymous"

// Check returns the key id for the presented secret and consumes one token
// from that key's rate limiter.
func (a *Authenticator) Check(secret string) (string, error) {
	id, _, err := a.Identify(secret)
	return id, err
}

// Identify is Check that also returns the key's role. With authentication
// disabled the anonymous caller is an admin: whoever can reach the server
// can do everything anyway.
func (a *Authenticator) Identify(secret string) (id, role string, err error) {
	id, role = Anonymous, RoleAdmin
	if a.Enabled() {
		if secret == "" {
			return "", "", ErrUnauthorized
		}
		h := sha256.Sum256([]byte(secret))
		id = ""
		for _, ks := range [][]Key{a.keys, a.storedKeys()} {
			for _, k := range ks {
				if subtle.ConstantTimeCompare(h[:], k.Hash[:]) == 1 {
					id, role = k.ID, k.Role
				}
			}
		}
		if id == "" {
			return "", "", ErrUnauthorized
		}
	}
	if !a.limiter(id).Allow() {
		return id, role, ErrRateLimited
	}
	return id, role, nil
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

type roleKey struct{}

// WithCaller stores the caller id in ctx.
func WithCaller(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Caller returns the caller id stored by WithCaller.
func Caller(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// WithRole stores the caller's role in ctx.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, roleKey{}, role)
}

// IsAdmin reports whether the caller in ctx has the admin role.
func IsAdmin(ctx context.Context) bool {
	r, _ := ctx.Value(roleKey{}).(string)
	return r == RoleAdmin
}
