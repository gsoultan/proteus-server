package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/safefetch"
)

const (
	keyA = "pk_test_key_a"
	keyB = "pk_test_key_b"
)

type testEnv struct {
	srv  *httptest.Server
	svc  *convert.Service
	jobs *jobs.Manager
}

type envOpts struct {
	workers       int
	allowRemote   bool
	webhookSecret string
	maxBody       int64
}

func newEnv(t *testing.T, o envOpts) *testEnv {
	t.Helper()
	if o.workers == 0 {
		o.workers = 2
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := metrics.New(o.workers)
	svc := convert.NewService(convert.Settings{
		Workers: o.workers, Timeout: 20 * time.Second, AllowRemote: o.allowRemote,
		Remote:       safefetch.New(safefetch.Options{}),
		LoaderLimits: loader.Limits{}, SystemFonts: false, Logger: log, Metrics: m,
	})
	var secret []byte
	if o.webhookSecret != "" {
		secret = []byte(o.webhookSecret)
	}
	jm := jobs.NewManager(svc, jobs.Settings{
		Workers: 1, WebhookSecret: secret,
		// Tests deliver webhooks to a loopback server.
		Webhook: safefetch.New(safefetch.Options{AllowPrivate: true}),
		Logger:  log, Observer: m,
	})
	keys, err := auth.ParseKeys("a:" + auth.HashKey(keyA) + ",b:" + auth.HashKey(keyB))
	if err != nil {
		t.Fatal(err)
	}
	api := New(Config{MaxBodyBytes: o.maxBody}, svc, jm, auth.New(keys, 0, 0), m, log)
	api.SetReady(true)
	srv := httptest.NewServer(api.Handler())
	t.Cleanup(func() {
		srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		jm.Shutdown(ctx)
	})
	return &testEnv{srv: srv, svc: svc, jobs: jm}
}

func (e *testEnv) do(t *testing.T, method, path, key, ct string, body []byte) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, e.srv.URL+path, bytes.NewReader(body))
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func readBody(t *testing.T, r *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func expectPDF(t *testing.T, r *http.Response) []byte {
	t.Helper()
	b := readBody(t, r)
	if r.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", r.StatusCode, b)
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("content type %q", ct)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", b[:min(len(b), 40)])
	}
	return b
}

func TestConvertJSON(t *testing.T) {
	e := newEnv(t, envOpts{})
	for name, body := range map[string]map[string]any{
		"html":     {"html": "<h1>Hello</h1><p>World</p>", "options": map[string]any{"page_size": "Letter", "margin_mm": 15}},
		"markdown": {"markdown": "# Title\n\n- one\n- two\n", "options": map[string]any{"metadata": map[string]any{"title": "My Doc"}}},
	} {
		t.Run(name, func(t *testing.T) {
			r := e.do(t, "POST", "/v1/convert", keyA, "application/json", mustJSON(body))
			expectPDF(t, r)
			if r.Header.Get("X-Request-ID") == "" {
				t.Error("missing X-Request-ID")
			}
		})
	}
	r := e.do(t, "POST", "/v1/convert", keyA, "application/json", mustJSON(map[string]any{"markdown": "x", "options": map[string]any{"metadata": map[string]any{"title": "Q3 report"}}}))
	expectPDF(t, r)
	if cd := r.Header.Get("Content-Disposition"); !strings.Contains(cd, "Q3-report.pdf") {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestConvertRawBody(t *testing.T) {
	e := newEnv(t, envOpts{})
	expectPDF(t, e.do(t, "POST", "/v1/convert", keyA, "text/markdown; charset=utf-8", []byte("# Hi")))
	expectPDF(t, e.do(t, "POST", "/v1/convert", keyA, "text/html", []byte("<p>Hi</p>")))
}

func pngPixel() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestConvertMultipartWithAssets(t *testing.T) {
	e := newEnv(t, envOpts{})
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "index.html")
	fw.Write([]byte(`<link rel="stylesheet" href="css/site.css"><h1>Invoice</h1><img src="img/logo.png" width="20">`))
	aw, _ := mw.CreateFormFile("asset", "css/site.css")
	aw.Write([]byte("h1 { color: #c00 }"))
	iw, _ := mw.CreateFormFile("asset", "img/logo.png")
	iw.Write(pngPixel())
	ow, _ := mw.CreateFormField("options")
	ow.Write([]byte(`{"page_size":"A5","landscape":true}`))
	mw.Close()
	r := e.do(t, "POST", "/v1/convert", keyA, mw.FormDataContentType(), buf.Bytes())
	pdf := expectPDF(t, r)
	if w := r.Header.Get("X-Proteus-Warnings"); w != "" {
		t.Errorf("unexpected warnings: %s", w)
	}
	if !bytes.Contains(pdf, []byte("/Image")) && !bytes.Contains(pdf, []byte("/XObject")) {
		t.Error("uploaded image not embedded")
	}
}

func TestAuth(t *testing.T) {
	e := newEnv(t, envOpts{})
	body := mustJSON(map[string]any{"html": "<p>x</p>"})
	if r := e.do(t, "POST", "/v1/convert", "", "application/json", body); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no key: %d", r.StatusCode)
	}
	if r := e.do(t, "POST", "/v1/convert", "wrong", "application/json", body); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad key: %d", r.StatusCode)
	}
	req, _ := http.NewRequest("POST", e.srv.URL+"/v1/convert", bytes.NewReader(body))
	req.Header.Set("X-API-Key", keyA)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	expectPDF(t, resp)
	// Probes need no key.
	if r := e.do(t, "GET", "/healthz", "", "", nil); r.StatusCode != 200 {
		t.Fatalf("healthz %d", r.StatusCode)
	}
	if r := e.do(t, "GET", "/readyz", "", "", nil); r.StatusCode != 200 {
		t.Fatalf("readyz %d", r.StatusCode)
	}
	r := e.do(t, "GET", "/metrics", "", "", nil)
	if m := string(readBody(t, r)); !strings.Contains(m, "proteus_conversions_total") {
		t.Fatalf("metrics missing conversions: %.200s", m)
	}
}

func TestValidation(t *testing.T) {
	e := newEnv(t, envOpts{maxBody: 4096})
	cases := []struct {
		ct   string
		body string
		code int
	}{
		{"application/json", `{"html":"<p>x</p>","markdown":"y"}`, 422},
		{"application/json", `{}`, 422},
		{"application/json", `{"html":"x","options":{"page_size":"B9"}}`, 422},
		{"application/json", `{"html":"x","options":{"margin_mm":500}}`, 422},
		{"application/json", `{"html":"x","unknown":1}`, 422},
		{"application/json", `{"html":"x","webhook_url":"https://example.com"}`, 422},
		{"application/json", `{"url":"https://example.com"}`, 422}, // remote disabled
		{"application/xml", `<x/>`, 422},
		{"text/html", strings.Repeat("a", 5000), 413},
	}
	for _, c := range cases {
		r := e.do(t, "POST", "/v1/convert", keyA, c.ct, []byte(c.body))
		b := readBody(t, r)
		if r.StatusCode != c.code {
			t.Errorf("%s %.40s: status %d, want %d (%s)", c.ct, c.body, r.StatusCode, c.code, b)
		}
	}
}

// Documents must not be able to read the server's disk or reach internal
// addresses, even with remote resources enabled.
func TestSSRFAndLocalFiles(t *testing.T) {
	var hits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("body{}"))
	}))
	defer internal.Close()
	e := newEnv(t, envOpts{allowRemote: true})
	html := `<link rel="stylesheet" href="` + internal.URL + `/a.css">
<link rel="stylesheet" href="file:///etc/passwd">
<link rel="stylesheet" href="/etc/passwd">
<img src="http://169.254.169.254/latest/meta-data/">
<img src="http://localhost:1/x.png"><p>hi</p>`
	r := e.do(t, "POST", "/v1/convert", keyA, "application/json",
		mustJSON(map[string]any{"html": html, "options": map[string]any{"allow_remote": true}}))
	expectPDF(t, r)
	if n := hits.Load(); n != 0 {
		t.Fatalf("internal server reached %d times", n)
	}
	// Fetching the document itself from an internal URL is refused too.
	r = e.do(t, "POST", "/v1/convert", keyA, "application/json", mustJSON(map[string]any{"url": internal.URL + "/doc.html"}))
	if b := readBody(t, r); r.StatusCode != 422 {
		t.Fatalf("url to loopback: %d %s", r.StatusCode, b)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("internal server reached %d times", n)
	}
}

func TestBusyReturns429(t *testing.T) {
	e := newEnv(t, envOpts{workers: 1})
	release, err := e.svc.TryAcquire()
	if err != nil {
		t.Fatal(err)
	}
	r := e.do(t, "POST", "/v1/convert", keyA, "text/html", []byte("<p>x</p>"))
	if r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Fatalf("status %d", r.StatusCode)
	}
	release()
	expectPDF(t, e.do(t, "POST", "/v1/convert", keyA, "text/html", []byte("<p>x</p>")))
}

func waitJob(t *testing.T, e *testEnv, id, key string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r := e.do(t, "GET", "/v1/jobs/"+id, key, "", nil)
		var j map[string]any
		json.Unmarshal(readBody(t, r), &j)
		if s := j["status"]; s == "succeeded" || s == "failed" {
			return j
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}

func TestJobsLifecycle(t *testing.T) {
	type hook struct {
		sig, ts string
		body    []byte
	}
	got := make(chan hook, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- hook{r.Header.Get("X-Proteus-Signature"), r.Header.Get("X-Proteus-Timestamp"), b}
	}))
	defer receiver.Close()
	const secret = "0123456789abcdef0123456789abcdef"
	e := newEnv(t, envOpts{webhookSecret: secret})

	r := e.do(t, "POST", "/v1/jobs", keyA, "application/json",
		mustJSON(map[string]any{"markdown": "# Async", "webhook_url": receiver.URL}))
	b := readBody(t, r)
	if r.StatusCode != http.StatusAccepted {
		t.Fatalf("submit: %d %s", r.StatusCode, b)
	}
	var j map[string]any
	json.Unmarshal(b, &j)
	id, _ := j["id"].(string)
	if len(id) != 32 || r.Header.Get("Location") != "/v1/jobs/"+id {
		t.Fatalf("bad job %s", b)
	}

	// Another key cannot see the job.
	if r := e.do(t, "GET", "/v1/jobs/"+id, keyB, "", nil); r.StatusCode != 404 {
		t.Fatalf("other key status %d", r.StatusCode)
	}
	if r := e.do(t, "GET", "/v1/jobs/"+id+"/result", keyB, "", nil); r.StatusCode != 404 {
		t.Fatalf("other key result %d", r.StatusCode)
	}

	fin := waitJob(t, e, id, keyA)
	if fin["status"] != "succeeded" || fin["result_url"] != "/v1/jobs/"+id+"/result" {
		t.Fatalf("job: %v", fin)
	}
	expectPDF(t, e.do(t, "GET", "/v1/jobs/"+id+"/result", keyA, "", nil))

	select {
	case h := <-got:
		if h.sig != "sha256="+jobs.Sign([]byte(secret), h.ts, h.body) {
			t.Fatalf("bad signature %q", h.sig)
		}
		if !bytes.Contains(h.body, []byte(id)) {
			t.Fatalf("webhook body %s", h.body)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no webhook")
	}

	if r := e.do(t, "DELETE", "/v1/jobs/"+id, keyA, "", nil); r.StatusCode != 204 {
		t.Fatalf("delete %d", r.StatusCode)
	}
	if r := e.do(t, "GET", "/v1/jobs/"+id, keyA, "", nil); r.StatusCode != 404 {
		t.Fatalf("after delete %d", r.StatusCode)
	}
}

func TestJobFailureAndWebhookValidation(t *testing.T) {
	e := newEnv(t, envOpts{webhookSecret: "0123456789abcdef0123456789abcdef"})
	// file:// webhook is refused up front.
	r := e.do(t, "POST", "/v1/jobs", keyA, "application/json",
		mustJSON(map[string]any{"html": "<p>x</p>", "webhook_url": "file:///tmp/x"}))
	if r.StatusCode != 422 {
		t.Fatalf("file webhook accepted: %d", r.StatusCode)
	}
	r = e.do(t, "GET", "/v1/jobs/doesnotexist", keyA, "", nil)
	if r.StatusCode != 404 {
		t.Fatalf("unknown job: %d", r.StatusCode)
	}
}
