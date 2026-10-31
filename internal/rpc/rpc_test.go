package rpc

import (
	"bytes"
	"context"
	"crypto/hmac"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"
	"google.golang.org/protobuf/encoding/protojson"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/metrics"
	"github.com/gsoultan/proteus-server/internal/safefetch"
	"github.com/gsoultan/proteus-server/internal/templates"
)

const (
	keyA = "pk_test_key_a"
	keyB = "pk_test_key_b"
)

type env struct {
	url string
	svc *convert.Service
	api *API
}

type envOpts struct {
	workers       int
	allowRemote   bool
	webhookSecret string
	maxRequest    int64
}

func newEnv(t *testing.T, o envOpts) *env {
	t.Helper()
	if o.workers == 0 {
		o.workers = 2
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := metrics.New(o.workers)
	svc := convert.NewService(convert.Settings{
		Workers: o.workers, Timeout: 20 * time.Second, AllowRemote: o.allowRemote,
		Remote: safefetch.New(safefetch.Options{}), LoaderLimits: loader.Limits{},
		SystemFonts: false, Logger: log, Metrics: m,
	})
	var secret []byte
	if o.webhookSecret != "" {
		secret = []byte(o.webhookSecret)
	}
	jm := jobs.NewManager(svc, jobs.NewMemoryStore(0), jobs.Settings{
		Workers: 1, PollInterval: 10 * time.Millisecond, WebhookSecret: secret,
		// Tests deliver webhooks to a loopback server.
		Webhook: safefetch.New(safefetch.Options{AllowPrivate: true}),
		Logger:  log, Observer: m,
	})
	keys, err := auth.ParseKeys("a:" + auth.HashKey(keyA) + ",b:" + auth.HashKey(keyB))
	if err != nil {
		t.Fatal(err)
	}
	ts := templates.NewService(templates.NewMemoryStore(), templates.Settings{MaxPerOwner: 5})
	api := New(Config{MaxRequestBytes: o.maxRequest}, svc, jm, ts, auth.New(keys, 0, 0), m, log, nil)
	api.SetReady(true)
	srv := httptest.NewUnstartedServer(api.Handler())
	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = p
	srv.Start()
	t.Cleanup(func() {
		srv.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		jm.Shutdown(ctx)
	})
	return &env{url: srv.URL, svc: svc, api: api}
}

func h2c() *http.Client {
	p := new(http.Protocols)
	p.SetUnencryptedHTTP2(true)
	return &http.Client{Transport: &http.Transport{Protocols: p}}
}

// withKey adds the API key to every call.
func withKey(key string) connect.ClientInterceptor {
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			if info, ok := connect.CallInfoForClientContext(ctx); ok && key != "" {
				info.RequestHeader().Set("Authorization", "Bearer "+key)
			}
			return next(ctx, spec)
		}
	}
}

// grpcClient speaks native gRPC over h2c.
func (e *env) grpcClient(key string) proteusv1connect.ConversionServiceClient {
	return proteusv1connect.NewConversionServiceClient(connect.NewClient(
		connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC()), withKey(key)))
}

// connectClient speaks the Connect protocol over HTTP/1.1, as a browser does.
func (e *env) connectClient(key string) proteusv1connect.ConversionServiceClient {
	return proteusv1connect.NewConversionServiceClient(connect.NewClient(
		connecthttp.NewTransport(http.DefaultClient, e.url), withKey(key)))
}

func md(s string) *proteusv1.Document {
	return &proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN, Source: &proteusv1.Document_Content{Content: []byte(s)}}
}

func html(s string) *proteusv1.Document {
	return &proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Source: &proteusv1.Document_Content{Content: []byte(s)}}
}

func expectPDF(t *testing.T, b []byte) {
	t.Helper()
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", b[:min(len(b), 40)])
	}
}

func expectCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("got success, want %s", want)
	}
	if got := connect.CodeOf(err); got != want {
		t.Fatalf("code %s, want %s (%v)", got, want, err)
	}
}

func TestConvertOverGRPCAndConnect(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	for name, c := range map[string]proteusv1connect.ConversionServiceClient{
		"grpc": e.grpcClient(keyA), "connect": e.connectClient(keyA),
	} {
		t.Run(name, func(t *testing.T) {
			ctx, info := connect.NewClientContext(ctx)
			res, err := c.Convert(ctx, &proteusv1.ConvertRequest{
				Document: md("# Title\n\n- one\n- two\n"),
				Options: &proteusv1.Options{Page: &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_LETTER},
					Metadata: &proteusv1.Metadata{Title: "Q3 report"}},
			})
			if err != nil {
				t.Fatal(err)
			}
			expectPDF(t, res.GetContent())
			if res.GetFileName() != "Q3-report.pdf" || res.GetMimeType() != "application/pdf" ||
				res.GetOutputFormat() != proteusv1.OutputFormat_OUTPUT_FORMAT_PDF {
				t.Errorf("file name %q, mime type %q, format %v", res.GetFileName(), res.GetMimeType(), res.GetOutputFormat())
			}
			if info.ResponseHeader().Get("X-Request-Id") == "" {
				t.Error("missing X-Request-Id")
			}
		})
	}
}

func pngPixel() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

func TestConvertWithAssets(t *testing.T) {
	e := newEnv(t, envOpts{})
	doc := html(`<link rel="stylesheet" href="css/site.css"><h1>Invoice</h1><img src="img/logo.png" width="20">`)
	doc.Assets = []*proteusv1.Asset{
		{Path: "css/site.css", Data: []byte("h1 { color: #c00 }")},
		{Path: "img/logo.png", Data: pngPixel()},
	}
	res, err := e.grpcClient(keyA).Convert(context.Background(), &proteusv1.ConvertRequest{
		Document: doc, Options: &proteusv1.Options{Page: &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_A5, Landscape: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	expectPDF(t, res.GetContent())
	if len(res.GetWarnings()) != 0 {
		t.Errorf("unexpected warnings: %v", res.GetWarnings())
	}
	if !bytes.Contains(res.GetContent(), []byte("/Image")) {
		t.Error("uploaded image not embedded")
	}
}

func TestAuthAndHealth(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	req := &proteusv1.ConvertRequest{Document: html("<p>x</p>")}
	_, err := e.grpcClient("").Convert(ctx, req)
	expectCode(t, err, connect.CodeUnauthenticated)
	_, err = e.grpcClient("wrong").Convert(ctx, req)
	expectCode(t, err, connect.CodeUnauthenticated)

	// x-api-key works too.
	c := proteusv1connect.NewConversionServiceClient(connect.NewClient(
		connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC()),
		func(next connect.ClientFunc) connect.ClientFunc {
			return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
				info, _ := connect.CallInfoForClientContext(ctx)
				info.RequestHeader().Set("X-Api-Key", keyA)
				return next(ctx, spec)
			}
		}))
	if _, err := c.Convert(ctx, req); err != nil {
		t.Fatal(err)
	}

	// Health needs no key and follows readiness.
	health := grpchealth.NewClient(connect.NewClient(connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC())))
	res, err := health.Check(ctx, &grpchealth.CheckRequest{Service: proteusv1connect.ConversionServiceName})
	if err != nil || res.Status != grpchealth.StatusServing {
		t.Fatalf("health = %v, %v", res, err)
	}
	e.api.SetReady(false)
	res, err = health.Check(ctx, &grpchealth.CheckRequest{})
	if err != nil || res.Status != grpchealth.StatusNotServing {
		t.Fatalf("health while not ready = %v, %v", res, err)
	}
	_, err = health.Check(ctx, &grpchealth.CheckRequest{Service: "nope.v1.Nope"})
	expectCode(t, err, connect.CodeNotFound)
}

func TestNoRESTSurface(t *testing.T) {
	e := newEnv(t, envOpts{})
	for _, path := range []string{"/v1/convert", "/healthz", "/metrics", "/"} {
		resp, err := http.Post(e.url+path, "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, resp.StatusCode)
		}
	}
}

func TestValidation(t *testing.T) {
	e := newEnv(t, envOpts{maxRequest: 4096})
	c := e.grpcClient(keyA)
	ctx := context.Background()
	margin := 500.0
	cases := map[string]*proteusv1.ConvertRequest{
		"no document":   {},
		"empty":         {Document: html("")},
		"no format":     {Document: &proteusv1.Document{Source: &proteusv1.Document_Content{Content: []byte("x")}}},
		"margin":        {Document: html("x"), Options: &proteusv1.Options{Page: &proteusv1.PageOptions{MarginMm: &margin}}},
		"output format": {Document: html("x"), OutputFormat: 99},
		"asset path":    {Document: &proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Source: &proteusv1.Document_Content{Content: []byte("x")}, Assets: []*proteusv1.Asset{{Path: "../etc/passwd"}}}},
		"url off":       {Document: &proteusv1.Document{Source: &proteusv1.Document_Url{Url: "https://example.com"}}},
	}
	for name, req := range cases {
		_, err := c.Convert(ctx, req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want invalid_argument", name, err)
		}
	}
	_, err := c.Convert(ctx, &proteusv1.ConvertRequest{Document: html(strings.Repeat("a", 5000))})
	expectCode(t, err, connect.CodeResourceExhausted)
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
	c := e.grpcClient(keyA)
	ctx := context.Background()
	doc := html(`<link rel="stylesheet" href="` + internal.URL + `/a.css">
<link rel="stylesheet" href="file:///etc/passwd">
<link rel="stylesheet" href="/etc/passwd">
<img src="http://169.254.169.254/latest/meta-data/">
<img src="http://localhost:1/x.png"><p>hi</p>`)
	res, err := c.Convert(ctx, &proteusv1.ConvertRequest{Document: doc, Options: &proteusv1.Options{AllowRemote: true}})
	if err != nil {
		t.Fatal(err)
	}
	expectPDF(t, res.GetContent())
	if n := hits.Load(); n != 0 {
		t.Fatalf("internal server reached %d times", n)
	}
	_, err = c.Convert(ctx, &proteusv1.ConvertRequest{Document: &proteusv1.Document{Source: &proteusv1.Document_Url{Url: internal.URL + "/doc.html"}}})
	expectCode(t, err, connect.CodeInvalidArgument)
	if n := hits.Load(); n != 0 {
		t.Fatalf("internal server reached %d times", n)
	}
}

func TestListFormats(t *testing.T) {
	e := newEnv(t, envOpts{})
	res, err := e.grpcClient(keyA).ListFormats(context.Background(), &proteusv1.ListFormatsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[proteusv1.InputFormat]*proteusv1.Conversion{}
	for _, c := range res.GetConversions() {
		got[c.GetInputFormat()] = c
	}
	for _, in := range []proteusv1.InputFormat{proteusv1.InputFormat_INPUT_FORMAT_HTML, proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN} {
		c := got[in]
		if c.GetOutputFormat() != proteusv1.OutputFormat_OUTPUT_FORMAT_PDF || c.GetMimeType() != "application/pdf" || c.GetFileExtension() != ".pdf" {
			t.Errorf("%v: %v", in, c)
		}
	}
	if len(res.GetConversions()) != 2 {
		t.Errorf("conversions %v", res.GetConversions())
	}
}

func TestBusy(t *testing.T) {
	e := newEnv(t, envOpts{workers: 1})
	release, err := e.svc.TryAcquire()
	if err != nil {
		t.Fatal(err)
	}
	c := e.grpcClient(keyA)
	_, err = c.Convert(context.Background(), &proteusv1.ConvertRequest{Document: html("<p>x</p>")})
	expectCode(t, err, connect.CodeResourceExhausted)
	release()
	if _, err := c.Convert(context.Background(), &proteusv1.ConvertRequest{Document: html("<p>x</p>")}); err != nil {
		t.Fatal(err)
	}
}

func waitJob(t *testing.T, c proteusv1connect.ConversionServiceClient, id string) *proteusv1.Job {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.GetJob(context.Background(), &proteusv1.GetJobRequest{Id: id})
		if err != nil {
			t.Fatal(err)
		}
		switch res.GetJob().GetStatus() {
		case proteusv1.JobStatus_JOB_STATUS_SUCCEEDED, proteusv1.JobStatus_JOB_STATUS_FAILED:
			return res.GetJob()
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
	a, b := e.grpcClient(keyA), e.grpcClient(keyB)
	ctx := context.Background()

	sub, err := a.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: md("# Async"), WebhookUrl: receiver.URL})
	if err != nil {
		t.Fatal(err)
	}
	id := sub.GetJob().GetId()
	if sub.GetJob().GetStatus() != proteusv1.JobStatus_JOB_STATUS_QUEUED || len(id) != 36 {
		t.Fatalf("submitted %v", sub.GetJob())
	}

	_, err = b.GetJob(ctx, &proteusv1.GetJobRequest{Id: id})
	expectCode(t, err, connect.CodeNotFound)

	job := waitJob(t, a, id)
	if job.GetStatus() != proteusv1.JobStatus_JOB_STATUS_SUCCEEDED || job.GetResultBytes() == 0 ||
		job.GetInputFormat() != proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN ||
		job.GetOutputFormat() != proteusv1.OutputFormat_OUTPUT_FORMAT_PDF ||
		job.GetMimeType() != "application/pdf" || job.GetFileName() != "document.pdf" {
		t.Fatalf("job %v", job)
	}

	stream, err := a.DownloadJobResult(ctx, &proteusv1.DownloadJobResultRequest{Id: id})
	if err != nil {
		t.Fatal(err)
	}
	var pdf []byte
	for {
		msg, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		pdf = append(pdf, msg.GetChunk()...)
	}
	expectPDF(t, pdf)
	if int64(len(pdf)) != job.GetResultBytes() {
		t.Fatalf("downloaded %d bytes, job says %d", len(pdf), job.GetResultBytes())
	}

	// Another key cannot download it.
	bs, err := b.DownloadJobResult(ctx, &proteusv1.DownloadJobResultRequest{Id: id})
	if err == nil {
		_, err = bs.Receive()
	}
	expectCode(t, err, connect.CodeNotFound)

	select {
	case h := <-got:
		if !hmac.Equal([]byte(h.sig), []byte("sha256="+jobs.Sign([]byte(secret), h.ts, h.body))) {
			t.Fatalf("bad signature %q", h.sig)
		}
		var j proteusv1.Job
		if err := protojson.Unmarshal(h.body, &j); err != nil || j.GetId() != id {
			t.Fatalf("webhook body %s: %v", h.body, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no webhook")
	}

	if _, err := a.DeleteJob(ctx, &proteusv1.DeleteJobRequest{Id: id}); err != nil {
		t.Fatal(err)
	}
	_, err = a.GetJob(ctx, &proteusv1.GetJobRequest{Id: id})
	expectCode(t, err, connect.CodeNotFound)
}

func TestJobValidation(t *testing.T) {
	e := newEnv(t, envOpts{webhookSecret: "0123456789abcdef0123456789abcdef"})
	c := e.grpcClient(keyA)
	ctx := context.Background()
	_, err := c.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: html("<p>x</p>"), WebhookUrl: "file:///tmp/x"})
	expectCode(t, err, connect.CodeInvalidArgument)
	_, err = c.GetJob(ctx, &proteusv1.GetJobRequest{Id: "doesnotexist"})
	expectCode(t, err, connect.CodeNotFound)
}
