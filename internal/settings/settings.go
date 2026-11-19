// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package settings keeps what the web UI configures: the saved PROTEUS_*
// values, whether first-run setup is done, and the API keys created in the
// UI. It lives in Postgres when the server has a database (shared by every
// instance) and otherwise in a file in the data directory.
//
// The database location itself cannot live in the database, so the /setup
// wizard writes it to a small bootstrap file next to it (see ReadBootstrap).
package settings

import (
	"context"
	"crypto/sha256"
	"errors"
	"regexp"
	"strings"
	"time"
)

// Doc is the saved configuration.
type Doc struct {
	// Values are PROTEUS_* names to their text, only editable fields.
	Values map[string]string `json:"values,omitempty"`
	// SetupCompletedAt is set by the /setup wizard; zero means first run.
	SetupCompletedAt time.Time `json:"setup_completed_at,omitzero"`
	// Revision counts saves; 0 means nothing was ever saved.
	Revision int64 `json:"revision"`
}

// SetupDone reports whether the first-run wizard finished.
func (d Doc) SetupDone() bool { return !d.SetupCompletedAt.IsZero() }

// Roles an API key can have.
const (
	RoleAdmin  = "admin"  // everything, including settings and keys
	RoleClient = "client" // conversions, jobs and templates
)

// ValidRole reports whether r is a known role.
func ValidRole(r string) bool { return r == RoleAdmin || r == RoleClient }

// APIKey is a key created in the UI. Only the SHA-256 of the key is kept.
type APIKey struct {
	ID        string // owner id of the key's jobs and templates; never reused
	Name      string
	Role      string
	Hash      [32]byte
	Prefix    string // first characters of the key, to recognise it
	CreatedAt time.Time
}

// Errors from a Store.
var (
	ErrRevisionMismatch = errors.New("settings were changed by someone else; reload and try again")
	ErrNotFound         = errors.New("api key not found")
	ErrKeyExists        = errors.New("api key id already exists")
)

// Store holds the saved settings and the UI's API keys.
type Store interface {
	// Load returns the saved settings, or a zero Doc when nothing was saved.
	Load(ctx context.Context) (Doc, error)
	// Save replaces the settings if the stored revision is still expected,
	// and returns them with the new revision.
	Save(ctx context.Context, d Doc, expected int64) (Doc, error)
	// ListKeys returns every key, oldest first.
	ListKeys(ctx context.Context) ([]APIKey, error)
	CreateKey(ctx context.Context, k APIKey) error
	DeleteKey(ctx context.Context, id string) error
}

// MaxNameRunes bounds a key's display name.
const MaxNameRunes = 100

var nameOK = regexp.MustCompile(`^[\p{L}\p{N} ._@-]+$`)

// CleanName trims a key name and checks it.
func CleanName(s string) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", errors.New("name is required")
	case len([]rune(s)) > MaxNameRunes:
		return "", errors.New("name is longer than 100 characters")
	case !nameOK.MatchString(s):
		return "", errors.New("name may use letters, digits, spaces and . _ @ -")
	}
	return s, nil
}

// Hash returns the SHA-256 of a plain key.
func Hash(key string) [32]byte { return sha256.Sum256([]byte(key)) }
