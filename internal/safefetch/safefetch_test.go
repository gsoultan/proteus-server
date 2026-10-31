package safefetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
)

func TestIsPublic(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8":                true,
		"1.1.1.1":                true,
		"2606:4700:4700::1111":   true,
		"127.0.0.1":              false,
		"10.1.2.3":               false,
		"172.16.0.1":             false,
		"192.168.1.1":            false,
		"169.254.169.254":        false, // cloud metadata
		"100.64.0.1":             false,
		"0.0.0.0":                false,
		"255.255.255.255":        false,
		"224.0.0.1":              false,
		"::1":                    false,
		"::":                     false,
		"fe80::1":                false,
		"fd00:ec2::254":          false, // AWS IPv6 metadata
		"::ffff:127.0.0.1":       false, // IPv4-mapped loopback
		"::ffff:169.254.169.254": false,
		"64:ff9b::a00:1":         false, // NAT64 of 10.0.0.1
		"2002:a00:1::":           false, // 6to4 of 10.0.0.1
	}
	for s, want := range cases {
		if got := IsPublic(netip.MustParseAddr(s)); got != want {
			t.Errorf("IsPublic(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestCheckURL(t *testing.T) {
	c := New(Options{})
	bad := []string{
		"file:///etc/passwd", "gopher://x", "ftp://example.com/a", "http://user:pw@example.com/",
		"http://localhost/", "http://foo.localhost/", "http://127.0.0.1/", "http://[::1]/",
		"http://169.254.169.254/latest/meta-data/", "http:///nohost",
	}
	for _, s := range bad {
		u, _ := url.Parse(s)
		if err := c.CheckURL(u); !errors.Is(err, ErrBlocked) {
			t.Errorf("CheckURL(%s) = %v, want ErrBlocked", s, err)
		}
	}
	u, _ := url.Parse("https://example.com/a.css")
	if err := c.CheckURL(u); err != nil {
		t.Errorf("public URL blocked: %v", err)
	}
}

func TestAllowlist(t *testing.T) {
	c := New(Options{AllowedHosts: []string{"cdn.example.com", ".assets.example.org"}})
	for s, ok := range map[string]bool{
		"https://cdn.example.com/x":       true,
		"https://evil.com/x":              false,
		"https://a.assets.example.org/x":  true,
		"https://assets.example.org/x":    true,
		"https://notassets.example.org/x": false,
	} {
		u, _ := url.Parse(s)
		if err := c.CheckURL(u); (err == nil) != ok {
			t.Errorf("CheckURL(%s) err=%v, want allowed=%v", s, err, ok)
		}
	}
}

// The dial-time check is what stops DNS names that resolve to internal
// addresses, so test it directly.
func TestControlBlocksPrivateAddresses(t *testing.T) {
	c := New(Options{})
	for _, addr := range []string{"127.0.0.1:80", "10.0.0.5:443", "[::1]:80", "169.254.169.254:80"} {
		if err := c.control("tcp4", addr, nil); !errors.Is(err, ErrBlocked) {
			t.Errorf("control(%s) = %v, want ErrBlocked", addr, err)
		}
	}
	if err := c.control("tcp4", "93.184.216.34:443", nil); err != nil {
		t.Errorf("public address blocked: %v", err)
	}
	if err := c.control("udp", "93.184.216.34:53", nil); err == nil {
		t.Error("udp allowed")
	}
}

func TestGetBlocksLoopbackServer(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++ }))
	defer srv.Close()
	c := New(Options{})
	if _, _, err := c.Get(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("Get loopback = %v, want ErrBlocked", err)
	}
	if hits != 0 {
		t.Fatalf("server was reached %d times", hits)
	}
}

func TestGetLimitsAndRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 2048))) })
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("p{}"))
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/tofile", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(Options{AllowPrivate: true, MaxBytes: 1024})
	ctx := context.Background()

	data, ct, err := c.Get(ctx, srv.URL+"/ok")
	if err != nil || string(data) != "p{}" || ct != "text/css" {
		t.Fatalf("ok: %q %q %v", data, ct, err)
	}
	if _, _, err := c.Get(ctx, srv.URL+"/big"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big: %v, want ErrTooLarge", err)
	}
	if _, _, err := c.Get(ctx, srv.URL+"/loop"); !errors.Is(err, ErrBlocked) {
		t.Errorf("loop: %v, want ErrBlocked", err)
	}
	if _, _, err := c.Get(ctx, srv.URL+"/tofile"); !errors.Is(err, ErrBlocked) {
		t.Errorf("redirect to file: %v, want ErrBlocked", err)
	}
}
