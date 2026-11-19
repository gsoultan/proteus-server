// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package config

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Kind is how a setting's text is parsed, which also picks the input the web
// UI shows for it.
type Kind int

const (
	String Kind = iota
	Int
	Float
	Bool
	Duration
	Bytes
	List   // comma separated
	Secret // never sent back to a client once saved
)

// Field describes one PROTEUS_* setting.
//
// Settings come from three places, highest first: the process environment,
// the bootstrap file in the data directory (written by the /setup wizard, for
// the database location), and the settings saved from the web UI. Editable
// fields are the ones the UI may save; the rest are read before the settings
// store can be opened (listeners, logging, data directory, database) or are
// operator escape hatches, so they stay in the environment.
type Field struct {
	Name     string
	Group    string
	Label    string
	Help     string
	Kind     Kind
	Default  string
	Editable bool
	// Bootstrap fields may also live in the bootstrap file (database only).
	Bootstrap bool
}

// Groups in display order.
var Groups = []string{"Limits", "Conversion", "Remote resources", "Jobs", "Templates", "Server"}

// Fields is every setting FromEnv reads. A test keeps it in sync with FromEnv.
var Fields = []Field{
	{Name: "PROTEUS_RATE_LIMIT", Group: "Limits", Label: "Requests per second per key", Help: "0 turns rate limiting off.", Kind: Float, Default: "10", Editable: true},
	{Name: "PROTEUS_RATE_BURST", Group: "Limits", Label: "Burst per key", Help: "Requests a key may send at once before the rate applies.", Kind: Int, Default: "20", Editable: true},
	{Name: "PROTEUS_MAX_REQUEST_BYTES", Group: "Limits", Label: "Largest request", Help: "Size of one request message, documents and assets included.", Kind: Bytes, Default: "20MiB", Editable: true},
	{Name: "PROTEUS_MAX_ASSETS", Group: "Limits", Label: "Assets per document", Kind: Int, Default: "64", Editable: true},
	{Name: "PROTEUS_MAX_OUTPUT_BYTES", Group: "Limits", Label: "Largest output", Help: "Conversions that produce more fail.", Kind: Bytes, Default: "100MiB", Editable: true},

	{Name: "PROTEUS_WORKERS", Group: "Conversion", Label: "Concurrent conversions", Help: "0 uses one per CPU.", Kind: Int, Default: "0", Editable: true},
	{Name: "PROTEUS_TIMEOUT", Group: "Conversion", Label: "Conversion timeout", Kind: Duration, Default: "30s", Editable: true},
	{Name: "PROTEUS_FONT_DIRS", Group: "Conversion", Label: "Extra font folders", Help: "Folders on the server, comma separated.", Kind: List, Default: "", Editable: true},
	{Name: "PROTEUS_SYSTEM_FONTS", Group: "Conversion", Label: "Use system fonts", Kind: Bool, Default: "true", Editable: true},

	{Name: "PROTEUS_ALLOW_REMOTE", Group: "Remote resources", Label: "Allow remote resources", Help: "Lets requests that ask for it load images, styles and fonts over http(s). Private and local addresses are always refused.", Kind: Bool, Default: "false", Editable: true},
	{Name: "PROTEUS_ALLOWED_HOSTS", Group: "Remote resources", Label: "Allowed hosts", Help: "Empty allows any public host. A leading dot allows subdomains, e.g. .example.com.", Kind: List, Default: "", Editable: true},
	{Name: "PROTEUS_FETCH_TIMEOUT", Group: "Remote resources", Label: "Fetch timeout", Kind: Duration, Default: "10s", Editable: true},
	{Name: "PROTEUS_MAX_RESOURCE_BYTES", Group: "Remote resources", Label: "Largest resource", Kind: Bytes, Default: "10MiB", Editable: true},
	{Name: "PROTEUS_MAX_RESOURCES", Group: "Remote resources", Label: "Resources per conversion", Kind: Int, Default: "256", Editable: true},
	{Name: "PROTEUS_MAX_TOTAL_RESOURCE_BYTES", Group: "Remote resources", Label: "Resource bytes per conversion", Kind: Bytes, Default: "50MiB", Editable: true},

	{Name: "PROTEUS_JOB_WORKERS", Group: "Jobs", Label: "Job workers", Help: "0 uses half the conversion slots.", Kind: Int, Default: "0", Editable: true},
	{Name: "PROTEUS_JOB_QUEUE", Group: "Jobs", Label: "Queue length", Kind: Int, Default: "100", Editable: true},
	{Name: "PROTEUS_JOB_TTL", Group: "Jobs", Label: "Keep results for", Kind: Duration, Default: "1h", Editable: true},
	{Name: "PROTEUS_MAX_JOBS", Group: "Jobs", Label: "Jobs kept in memory", Help: "Only used without a database.", Kind: Int, Default: "1000", Editable: true},
	{Name: "PROTEUS_WEBHOOK_SECRET", Group: "Jobs", Label: "Webhook signing secret", Help: "At least 32 characters. Signs job callbacks with HMAC-SHA256; empty sends them unsigned.", Kind: Secret, Default: "", Editable: true},

	{Name: "PROTEUS_MAX_TEMPLATES", Group: "Templates", Label: "Templates per key", Kind: Int, Default: "500", Editable: true},
	{Name: "PROTEUS_MAX_TEMPLATE_BYTES", Group: "Templates", Label: "Largest template", Help: "Content plus assets.", Kind: Bytes, Default: "10MiB", Editable: true},
	{Name: "PROTEUS_TEMPLATE_RENDER_TIMEOUT", Group: "Templates", Label: "Fill timeout", Kind: Duration, Default: "5s", Editable: true},

	// Read before anything can be loaded, so they stay in the environment.
	{Name: "PROTEUS_ADDR", Group: "Server", Label: "Listen address", Kind: String, Default: ":8080"},
	{Name: "PROTEUS_ADMIN_ADDR", Group: "Server", Label: "Metrics address", Help: "off disables it.", Kind: String, Default: ":9090"},
	{Name: "PROTEUS_SHUTDOWN_TIMEOUT", Group: "Server", Label: "Shutdown timeout", Kind: Duration, Default: "30s"},
	{Name: "PROTEUS_LOG_LEVEL", Group: "Server", Label: "Log level", Kind: String, Default: "info"},
	{Name: "PROTEUS_LOG_FORMAT", Group: "Server", Label: "Log format", Kind: String, Default: "json"},
	{Name: "PROTEUS_DATA_DIR", Group: "Server", Label: "Data folder", Help: "Holds the bootstrap file and, without a database, the saved settings and keys.", Kind: String, Default: "data"},
	{Name: "PROTEUS_SETUP_TOKEN", Group: "Server", Label: "Setup token", Help: "Fixed token for the /setup wizard; random and logged when empty.", Kind: Secret, Default: ""},
	{Name: "PROTEUS_API_KEYS", Group: "Server", Label: "API keys from the environment", Kind: Secret, Default: ""},
	{Name: "PROTEUS_ALLOW_NO_AUTH", Group: "Server", Label: "Run without API keys", Kind: Bool, Default: "false"},
	{Name: "PROTEUS_DATABASE_URL", Group: "Server", Label: "PostgreSQL URL", Kind: Secret, Default: "", Bootstrap: true},
	{Name: "PROTEUS_DATABASE_SCHEMA", Group: "Server", Label: "Database schema", Kind: String, Default: "public", Bootstrap: true},
	{Name: "PROTEUS_AUTO_MIGRATE", Group: "Server", Label: "Create tables at startup", Kind: Bool, Default: "true", Bootstrap: true},
}

// Lookup returns the field named name.
func Lookup(name string) (Field, bool) {
	i := slices.IndexFunc(Fields, func(f Field) bool { return f.Name == name })
	if i < 0 {
		return Field{}, false
	}
	return Fields[i], true
}

// Layered returns a getenv that reads env first, then each layer in order.
// A variable set to an empty string in the environment counts as unset.
func Layered(env func(string) string, layers ...map[string]string) func(string) string {
	return func(k string) string {
		if v := strings.TrimSpace(env(k)); v != "" {
			return v
		}
		for _, l := range layers {
			if v := strings.TrimSpace(l[k]); v != "" {
				return v
			}
		}
		return ""
	}
}

// ValidateSaved checks values the UI wants to save: only editable fields, and
// the whole configuration must still parse with them on top of env and
// bootstrap. It returns the cleaned map (empty values dropped).
func ValidateSaved(env func(string) string, bootstrap, saved map[string]string) (map[string]string, error) {
	clean := map[string]string{}
	var errs []error
	for _, k := range slices.Sorted(maps.Keys(saved)) {
		f, ok := Lookup(k)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s: unknown setting", k))
		case !f.Editable:
			errs = append(errs, fmt.Errorf("%s: set in the environment, not from the web UI", k))
		default:
			if v := strings.TrimSpace(saved[k]); v != "" {
				clean[k] = v
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	// Check the values themselves, not whatever env would override them with.
	if _, err := FromEnv(Layered(func(string) string { return "" }, clean, bootstrap)); err != nil {
		return nil, err
	}
	return clean, nil
}

// ValidateBootstrap checks database values for the bootstrap file.
func ValidateBootstrap(b map[string]string) (map[string]string, error) {
	clean := map[string]string{}
	for k, v := range b {
		f, ok := Lookup(k)
		if !ok || !f.Bootstrap {
			return nil, fmt.Errorf("%s: not a bootstrap setting", k)
		}
		if v = strings.TrimSpace(v); v != "" {
			clean[k] = v
		}
	}
	if _, err := FromEnv(Layered(func(string) string { return "" }, clean)); err != nil {
		return nil, err
	}
	return clean, nil
}
