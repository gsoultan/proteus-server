// Package safefetch is an HTTP client for fetching untrusted URLs without
// server-side request forgery (SSRF). Only http and https are allowed, every
// connection is checked against the resolved IP address (so DNS rebinding and
// redirects cannot reach internal hosts), and responses are bounded in time and
// size.
package safefetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrBlocked is returned when a URL or the address it resolves to is not allowed.
	ErrBlocked = errors.New("safefetch: destination not allowed")
	// ErrTooLarge is returned when a response body exceeds the size limit.
	ErrTooLarge = errors.New("safefetch: response too large")
)

// Options configures a Client.
type Options struct {
	Timeout      time.Duration // whole request, including the body; default 10s
	MaxBytes     int64         // per response; default 10 MiB
	MaxRedirects int           // default 5
	// AllowedHosts, when non-empty, restricts fetches to these host names.
	// An entry ".example.com" also matches every subdomain.
	AllowedHosts []string
	// AllowPrivate disables the IP range checks. Only for tests and trusted networks.
	AllowPrivate bool
	UserAgent    string
}

// Client fetches untrusted URLs safely. It is safe for concurrent use.
type Client struct {
	opts Options
	http *http.Client
}

// New returns a Client with opts applied over the defaults.
func New(opts Options) *Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 10 << 20
	}
	if opts.MaxRedirects <= 0 {
		opts.MaxRedirects = 5
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "proteus-server"
	}
	c := &Client{opts: opts}
	dialer := &net.Dialer{Timeout: 5 * time.Second, Control: c.control}
	transport := &http.Transport{
		// No proxy: a proxy would make the dialer see the proxy's address, not the target's.
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: opts.Timeout,
	}
	c.http = &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return fmt.Errorf("%w: too many redirects", ErrBlocked)
			}
			return c.CheckURL(req.URL)
		},
	}
	return c
}

// CheckURL reports whether u may be fetched, before any DNS lookup: the scheme
// must be http or https, there must be no credentials, and the host must pass
// the allowlist. Literal IP hosts are checked against the blocked ranges.
func (c *Client) CheckURL(u *url.URL) error {
	if u == nil {
		return ErrBlocked
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme %q", ErrBlocked, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: credentials in URL", ErrBlocked)
	}
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	if host == "" {
		return fmt.Errorf("%w: empty host", ErrBlocked)
	}
	if !c.hostAllowed(host) {
		return fmt.Errorf("%w: host %q not in allowlist", ErrBlocked, host)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		if !c.opts.AllowPrivate {
			return fmt.Errorf("%w: host %q", ErrBlocked, host)
		}
	}
	if ip, err := netip.ParseAddr(host); err == nil && !c.opts.AllowPrivate && !IsPublic(ip) {
		return fmt.Errorf("%w: address %s", ErrBlocked, ip)
	}
	return nil
}

func (c *Client) hostAllowed(host string) bool {
	if len(c.opts.AllowedHosts) == 0 {
		return true
	}
	for _, h := range c.opts.AllowedHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if strings.HasPrefix(h, ".") {
			if host == h[1:] || strings.HasSuffix(host, h) {
				return true
			}
		} else if host == h {
			return true
		}
	}
	return false
}

// control runs for every outgoing connection, after DNS resolution, so it sees
// the address actually dialed.
func (c *Client) control(network, address string, _ syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" {
		return fmt.Errorf("%w: network %s", ErrBlocked, network)
	}
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBlocked, err)
	}
	if !c.opts.AllowPrivate && !IsPublic(ap.Addr()) {
		return fmt.Errorf("%w: address %s", ErrBlocked, ap.Addr())
	}
	return nil
}

// Get fetches rawURL and returns the body and its Content-Type.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrBlocked, err)
	}
	if err := c.CheckURL(u); err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.opts.UserAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("safefetch: %s: status %d", u.Redacted(), resp.StatusCode)
	}
	if resp.ContentLength > c.opts.MaxBytes {
		return nil, "", ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.opts.MaxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > c.opts.MaxBytes {
		return nil, "", ErrTooLarge
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// Do sends req (for example a webhook POST) through the guarded transport.
// The caller closes the response body.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if err := c.CheckURL(req.URL); err != nil {
		return nil, err
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.opts.UserAgent)
	}
	return c.http.Do(req)
}

// blocked lists ranges that are not globally routable or that reach cloud
// metadata services, beyond what netip's predicates already cover.
var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // "this" network
	netip.MustParsePrefix("100.64.0.0/10"),   // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),    // IETF protocol assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // TEST-NET-1
	netip.MustParsePrefix("198.18.0.0/15"),   // benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // TEST-NET-3
	netip.MustParsePrefix("240.0.0.0/4"),     // reserved, includes broadcast
	netip.MustParsePrefix("64:ff9b::/96"),    // NAT64, can embed private IPv4
	netip.MustParsePrefix("64:ff9b:1::/48"),  // local-use NAT64
	netip.MustParsePrefix("100::/64"),        // discard-only
	netip.MustParsePrefix("2001::/23"),       // IETF protocol assignments (Teredo etc.)
	netip.MustParsePrefix("2001:db8::/32"),   // documentation
	netip.MustParsePrefix("2002::/16"),       // 6to4, can embed private IPv4
	netip.MustParsePrefix("fc00::/7"),        // unique local
	netip.MustParsePrefix("fec0::/10"),       // deprecated site-local
}

// IsPublic reports whether ip is a globally routable unicast address that is
// safe to connect to on behalf of an untrusted caller.
func IsPublic(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() || ip.IsMulticast() ||
		ip.IsUnspecified() || !ip.IsGlobalUnicast() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}
