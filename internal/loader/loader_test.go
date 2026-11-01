// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package loader

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	proteus "github.com/gsoultan/proteus-converter"

	"github.com/gsoultan/proteus-server/internal/safefetch"
)

func TestCleanAssetPath(t *testing.T) {
	ok := map[string]string{
		"a.png": "a.png", "./img/a.png": "img/a.png", "/img/a.png": "img/a.png",
		"img\\a.png": "img/a.png", "img/a%20b.png": "img/a b.png", "a.css?v=1": "a.css",
	}
	for in, want := range ok {
		if got, good := CleanAssetPath(in); !good || got != want {
			t.Errorf("CleanAssetPath(%q) = %q, %v; want %q", in, got, good, want)
		}
	}
	for _, in := range []string{"", "../etc/passwd", "img/../../x", "%2e%2e/x", "/"} {
		if got, good := CleanAssetPath(in); good {
			t.Errorf("CleanAssetPath(%q) = %q, want rejection", in, got)
		}
	}
}

func TestLoad(t *testing.T) {
	ctx := context.Background()
	l := New(map[string][]byte{"img/logo.png": []byte("PNG"), "style.css": []byte("p{}")}, nil, Limits{})
	var blocked []string
	l.OnBlocked = func(r string) { blocked = append(blocked, r) }

	for ref, want := range map[string]string{
		"img/logo.png": "PNG", "/style.css": "p{}", "./img/logo.png": "PNG",
		"data:text/plain;base64,aGVsbG8=": "hello", "data:text/css,p%7Bcolor:red%7D": "p{color:red}",
	} {
		got, err := l.Load(ctx, ref)
		if err != nil || string(got) != want {
			t.Errorf("Load(%q) = %q, %v; want %q", ref, got, err, want)
		}
	}
	for _, ref := range []string{"file:///etc/passwd", "http://example.com/a.png", "ftp://x/y", "../secret"} {
		if _, err := l.Load(ctx, ref); !errors.Is(err, proteus.ErrResourceDenied) {
			t.Errorf("Load(%q) = %v, want ErrResourceDenied", ref, err)
		}
	}
	if _, err := l.Load(ctx, "/etc/passwd"); err == nil {
		t.Error("absolute disk path was served")
	}
	if len(blocked) != 4 {
		t.Errorf("blocked reasons = %v, want 4", blocked)
	}
}

func TestLimits(t *testing.T) {
	ctx := context.Background()
	l := New(map[string][]byte{"a": make([]byte, 600), "b": make([]byte, 600)}, nil,
		Limits{MaxResources: 3, MaxTotal: 1000, MaxEach: 700})
	if _, err := l.Load(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Load(ctx, "b"); !errors.Is(err, proteus.ErrResourceTooLarge) {
		t.Fatalf("total budget: %v", err)
	}
	if _, err := l.Load(ctx, "data:,x"); !errors.Is(err, proteus.ErrResourceTooLarge) {
		t.Fatalf("budget stays exhausted: %v", err)
	}
	if _, err := l.Load(ctx, "data:,x"); !errors.Is(err, proteus.ErrResourceDenied) {
		t.Fatalf("count limit: %v", err)
	}
}

func TestRemoteGoesThroughSafefetch(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	l := New(nil, safefetch.New(safefetch.Options{}), Limits{})
	if _, err := l.Load(context.Background(), srv.URL+"/x.png"); !errors.Is(err, proteus.ErrResourceDenied) {
		t.Fatalf("loopback fetch = %v, want ErrResourceDenied", err)
	}
	if hits != 0 {
		t.Fatal("loopback server was reached")
	}
}
