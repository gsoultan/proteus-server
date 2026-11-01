// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package loader is the resource loader the server hands to the conversion
// engine. It replaces the engine's default loader so a document can never read
// the server's disk: resources come from data URLs, from files uploaded with
// the request, or (only when enabled) from public http(s) URLs fetched
// through safefetch.
package loader

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"

	proteus "github.com/gsoultan/proteus-converter"

	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// ErrDenied and ErrTooLarge wrap every refused resource, whichever converter
// asked for it. They are the engine's sentinels so its errors match too.
var (
	ErrDenied   = proteus.ErrResourceDenied
	ErrTooLarge = proteus.ErrResourceTooLarge
)

// Limits bound what a single document may load.
type Limits struct {
	MaxResources int   // number of distinct loads per document; default 256
	MaxTotal     int64 // total bytes across all loads; default 50 MiB
	MaxEach      int64 // bytes per resource; default 10 MiB
}

// Loader resolves resources for one conversion. Create one per request.
type Loader struct {
	assets map[string][]byte // uploaded files keyed by clean relative path
	remote *safefetch.Client // nil when remote fetching is disabled
	limits Limits

	mu    sync.Mutex
	count int
	total int64

	// OnBlocked, when set, is called for every refused load (for metrics).
	OnBlocked func(reason string)
}

// New returns a loader serving assets and, when remote is non-nil, public URLs.
func New(assets map[string][]byte, remote *safefetch.Client, limits Limits) *Loader {
	if limits.MaxResources <= 0 {
		limits.MaxResources = 256
	}
	if limits.MaxTotal <= 0 {
		limits.MaxTotal = 50 << 20
	}
	if limits.MaxEach <= 0 {
		limits.MaxEach = 10 << 20
	}
	clean := make(map[string][]byte, len(assets))
	for k, v := range assets {
		if p, ok := CleanAssetPath(k); ok {
			clean[p] = v
		}
	}
	return &Loader{assets: clean, remote: remote, limits: limits}
}

// CleanAssetPath normalizes an uploaded file name or a document reference to
// the key used in the asset map. It rejects paths that escape the root.
func CleanAssetPath(p string) (string, bool) {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimLeft(p, "/")
	if p == "" {
		return "", false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return "", false
		}
	}
	return path.Clean(p), true
}

// Load implements proteus.ResourceLoader.
func (l *Loader) Load(ctx context.Context, ref string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := l.reserve(); err != nil {
		return nil, err
	}
	data, err := l.load(ctx, ref)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > l.limits.MaxEach {
		return nil, l.deny("too_large", fmt.Errorf("%w: %s", proteus.ErrResourceTooLarge, short(ref)))
	}
	l.mu.Lock()
	l.total += int64(len(data))
	over := l.total > l.limits.MaxTotal
	l.mu.Unlock()
	if over {
		return nil, l.deny("total_limit", fmt.Errorf("%w: total resource budget exceeded", proteus.ErrResourceTooLarge))
	}
	return data, nil
}

func (l *Loader) reserve() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count++
	if l.count > l.limits.MaxResources {
		return l.deny("count_limit", fmt.Errorf("%w: too many resources", proteus.ErrResourceDenied))
	}
	return nil
}

func (l *Loader) load(ctx context.Context, ref string) ([]byte, error) {
	if strings.HasPrefix(ref, "data:") {
		return decodeDataURL(ref, l.limits.MaxEach)
	}
	u, err := url.Parse(ref)
	if err == nil && len(u.Scheme) > 1 {
		switch u.Scheme {
		case "http", "https":
			if l.remote == nil {
				return nil, l.deny("remote_disabled", fmt.Errorf("%w: remote resources are disabled", proteus.ErrResourceDenied))
			}
			data, _, err := l.remote.Get(ctx, ref)
			if err != nil {
				if errors.Is(err, safefetch.ErrBlocked) {
					return nil, l.deny("ssrf", fmt.Errorf("%w: %v", proteus.ErrResourceDenied, err))
				}
				if errors.Is(err, safefetch.ErrTooLarge) {
					return nil, l.deny("too_large", fmt.Errorf("%w: %s", proteus.ErrResourceTooLarge, short(ref)))
				}
				return nil, err
			}
			return data, nil
		default:
			return nil, l.deny("scheme", fmt.Errorf("%w: scheme %q", proteus.ErrResourceDenied, u.Scheme))
		}
	}
	key, ok := CleanAssetPath(ref)
	if !ok {
		return nil, l.deny("path", fmt.Errorf("%w: %s", proteus.ErrResourceDenied, short(ref)))
	}
	if data, ok := l.assets[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("proteus-server: resource %q was not uploaded", key)
}

func (l *Loader) deny(reason string, err error) error {
	if l.OnBlocked != nil {
		l.OnBlocked(reason)
	}
	return err
}

func decodeDataURL(ref string, limit int64) ([]byte, error) {
	comma := strings.IndexByte(ref, ',')
	if comma < 0 {
		return nil, errors.New("proteus-server: malformed data URL")
	}
	meta, payload := ref[len("data:"):comma], ref[comma+1:]
	if int64(len(payload)) > limit*2 {
		return nil, fmt.Errorf("%w: data URL", proteus.ErrResourceTooLarge)
	}
	if strings.HasSuffix(meta, ";base64") {
		payload = strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, payload)
		if b, err := base64.StdEncoding.DecodeString(payload); err == nil {
			return b, nil
		}
		return base64.RawStdEncoding.DecodeString(strings.TrimRight(payload, "="))
	}
	s, err := url.PathUnescape(payload)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

func short(s string) string {
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
