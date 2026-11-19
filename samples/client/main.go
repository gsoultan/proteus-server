// Command client is a small example client for proteus-server. It shows how
// to call proteus.v1.ConversionService from Go with the generated ConnectRPC
// client, and it doubles as the end-to-end check for the samples in
// samples/docs.
//
//	go run ./samples/client formats                 list the supported conversions
//	go run ./samples/client convert [-o F] PATH      convert synchronously
//	go run ./samples/client job [-o F] PATH          convert through SubmitJob, GetJob, DownloadJobResult, DeleteJob
//	go run ./samples/client samples [-out DIR]       run every sample in samples/docs both ways
//
// PATH is a .html, .htm, .md or .markdown file, or a sample directory holding
// index.html or index.md. Every other file in a sample directory is sent as an
// asset under its relative path, except options.json, which holds the
// request's Options as protobuf JSON.
//
// Global flags (before the command): -url (PROTEUS_URL, default
// http://127.0.0.1:8080), -key (PROTEUS_API_KEY) and -protocol (grpc, connect
// or grpcweb).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"connectrpc.com/grpchealth/v2"
	"google.golang.org/protobuf/encoding/protojson"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "client:", err)
		os.Exit(1)
	}
}

type client struct {
	svc    proteusv1connect.ConversionServiceClient
	health *grpchealth.Client
	out    io.Writer
}

func run(args []string, out io.Writer) error {
	fl := flag.NewFlagSet("client", flag.ContinueOnError)
	url := fl.String("url", envOr("PROTEUS_URL", "http://127.0.0.1:8080"), "server base URL")
	key := fl.String("key", os.Getenv("PROTEUS_API_KEY"), "API key (empty when the server runs without auth)")
	protocol := fl.String("protocol", "grpc", "grpc, connect or grpcweb")
	if err := fl.Parse(args); err != nil {
		return err
	}
	c, err := newClient(*url, *key, *protocol, out)
	if err != nil {
		return err
	}
	rest := fl.Args()
	if len(rest) == 0 {
		return errors.New("missing command: formats, convert, job or samples")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	switch cmd, cargs := rest[0], rest[1:]; cmd {
	case "formats":
		return c.formats(ctx)
	case "convert", "job":
		cf := flag.NewFlagSet(cmd, flag.ContinueOnError)
		o := cf.String("o", "", "output file (default: the server's suggested file name)")
		if err := cf.Parse(cargs); err != nil {
			return err
		}
		if cf.NArg() != 1 {
			return fmt.Errorf("usage: %s [-o FILE] PATH", cmd)
		}
		s, err := loadSample(cf.Arg(0))
		if err != nil {
			return err
		}
		var content []byte
		var name string
		if cmd == "convert" {
			content, name, err = c.convert(ctx, s)
		} else {
			content, name, err = c.job(ctx, s)
		}
		if err != nil {
			return err
		}
		if *o != "" {
			name = *o
		}
		return c.write(name, content)
	case "samples":
		cf := flag.NewFlagSet(cmd, flag.ContinueOnError)
		dir := cf.String("dir", "samples/docs", "directory of sample directories")
		outDir := cf.String("out", "samples/out", "where to write the PDFs")
		if err := cf.Parse(cargs); err != nil {
			return err
		}
		return c.samples(ctx, *dir, *outDir)
	default:
		return fmt.Errorf("unknown command %q (formats, convert, job, samples)", cmd)
	}
}

func envOr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func newClient(url, key, protocol string, out io.Writer) (*client, error) {
	var (
		hc   *http.Client
		opts = []connecthttp.Option{connecthttp.WithReadMaxBytes(0)} // PDFs can pass the 4 MiB default
	)
	switch protocol {
	case "grpc":
		// Native gRPC needs HTTP/2; the server speaks it without TLS (h2c).
		p := new(http.Protocols)
		p.SetUnencryptedHTTP2(true)
		hc = &http.Client{Transport: &http.Transport{Protocols: p}}
		opts = append(opts, connecthttp.WithGRPC())
	case "grpcweb":
		hc = http.DefaultClient
		opts = append(opts, connecthttp.WithGRPCWeb())
	case "connect":
		hc = http.DefaultClient
	default:
		return nil, fmt.Errorf("unknown protocol %q (grpc, connect, grpcweb)", protocol)
	}
	cc := connect.NewClient(connecthttp.NewTransport(hc, strings.TrimRight(url, "/"), opts...), bearer(key))
	return &client{
		svc:    proteusv1connect.NewConversionServiceClient(cc),
		health: grpchealth.NewClient(cc),
		out:    out,
	}, nil
}

// bearer adds "authorization: Bearer KEY" to every call.
func bearer(key string) connect.ClientInterceptor {
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			if info, ok := connect.CallInfoForClientContext(ctx); ok && key != "" {
				info.RequestHeader().Set("Authorization", "Bearer "+key)
			}
			return next(ctx, spec)
		}
	}
}

// sample is one document ready to send.
type sample struct {
	name    string
	doc     *proteusv1.Document
	options *proteusv1.Options
}

func inputFormat(path string) (proteusv1.InputFormat, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm":
		return proteusv1.InputFormat_INPUT_FORMAT_HTML, true
	case ".md", ".markdown":
		return proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN, true
	}
	return proteusv1.InputFormat_INPUT_FORMAT_UNSPECIFIED, false
}

// loadSample reads a document file, or a sample directory with its index
// document, assets and optional options.json.
func loadSample(path string) (*sample, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		f, ok := inputFormat(path)
		if !ok {
			return nil, fmt.Errorf("%s: want a .html, .htm, .md or .markdown file", path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		return &sample{name: name, doc: &proteusv1.Document{Format: f, Source: &proteusv1.Document_Content{Content: b}}}, nil
	}

	s := &sample{name: filepath.Base(path), doc: &proteusv1.Document{}}
	err = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		switch {
		case rel == "options.json":
			s.options = new(proteusv1.Options)
			if err := protojson.Unmarshal(b, s.options); err != nil {
				return fmt.Errorf("%s: %w", p, err)
			}
		case strings.HasPrefix(rel, "index."):
			f, ok := inputFormat(rel)
			if !ok {
				return fmt.Errorf("%s: index must be .html, .htm, .md or .markdown", p)
			}
			if s.doc.Source != nil {
				return fmt.Errorf("%s: more than one index document", path)
			}
			s.doc.Format = f
			s.doc.Source = &proteusv1.Document_Content{Content: b}
		default:
			s.doc.Assets = append(s.doc.Assets, &proteusv1.Asset{Path: rel, Data: b})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.doc.Source == nil {
		return nil, fmt.Errorf("%s: no index.html or index.md", path)
	}
	return s, nil
}

func (c *client) formats(ctx context.Context) error {
	res, err := c.svc.ListFormats(ctx, &proteusv1.ListFormatsRequest{})
	if err != nil {
		return err
	}
	for _, cv := range res.GetConversions() {
		fmt.Fprintf(c.out, "%-22s -> %-18s %s (%s)\n", cv.GetInputFormat(), cv.GetOutputFormat(),
			cv.GetMimeType(), cv.GetFileExtension())
	}
	return nil
}

func (c *client) convert(ctx context.Context, s *sample) ([]byte, string, error) {
	ctx, info := connect.NewClientContext(ctx)
	res, err := c.svc.Convert(ctx, &proteusv1.ConvertRequest{Document: s.doc, Options: s.options})
	if err != nil {
		return nil, "", fmt.Errorf("convert %s: %w", s.name, err)
	}
	fmt.Fprintf(c.out, "convert %-12s %7d bytes in %4d ms  request %s\n", s.name, len(res.GetContent()),
		res.GetDurationMs(), info.ResponseHeader().Get("X-Request-Id"))
	for _, w := range res.GetWarnings() {
		fmt.Fprintf(c.out, "  warning: %s\n", w)
	}
	return res.GetContent(), res.GetFileName(), nil
}

// job runs the asynchronous flow: submit, poll, download the stream, delete.
func (c *client) job(ctx context.Context, s *sample) ([]byte, string, error) {
	sub, err := c.svc.SubmitJob(ctx, &proteusv1.SubmitJobRequest{Document: s.doc, Options: s.options})
	if err != nil {
		return nil, "", fmt.Errorf("submit %s: %w", s.name, err)
	}
	id := sub.GetJob().GetId()
	var job *proteusv1.Job
	for wait := 50 * time.Millisecond; ; wait = min(wait*2, time.Second) {
		res, err := c.svc.GetJob(ctx, &proteusv1.GetJobRequest{Id: id})
		if err != nil {
			return nil, "", fmt.Errorf("get job %s: %w", id, err)
		}
		job = res.GetJob()
		if st := job.GetStatus(); st == proteusv1.JobStatus_JOB_STATUS_SUCCEEDED || st == proteusv1.JobStatus_JOB_STATUS_FAILED {
			break
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(wait):
		}
	}
	if job.GetStatus() == proteusv1.JobStatus_JOB_STATUS_FAILED {
		return nil, "", fmt.Errorf("job %s for %s failed: %s", id, s.name, job.GetError())
	}

	stream, err := c.svc.DownloadJobResult(ctx, &proteusv1.DownloadJobResultRequest{Id: id})
	if err != nil {
		return nil, "", fmt.Errorf("download %s: %w", id, err)
	}
	var content []byte
	for {
		msg, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("download %s: %w", id, err)
		}
		content = append(content, msg.GetChunk()...)
	}
	if int64(len(content)) != job.GetResultBytes() {
		return nil, "", fmt.Errorf("job %s: downloaded %d bytes, job reports %d", id, len(content), job.GetResultBytes())
	}
	if _, err := c.svc.DeleteJob(ctx, &proteusv1.DeleteJobRequest{Id: id}); err != nil {
		return nil, "", fmt.Errorf("delete %s: %w", id, err)
	}
	fmt.Fprintf(c.out, "job     %-12s %7d bytes            job %s (deleted)\n", s.name, len(content), id)
	return content, job.GetFileName(), nil
}

func (c *client) write(name string, content []byte) error {
	if err := os.WriteFile(name, content, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "wrote %s\n", name)
	return nil
}

// samples checks health, lists formats, then converts every sample both
// synchronously and as a job, checking each result is a PDF.
func (c *client) samples(ctx context.Context, dir, outDir string) error {
	h, err := c.health.Check(ctx, &grpchealth.CheckRequest{})
	if err != nil {
		return fmt.Errorf("health: %w", err)
	}
	if h.Status != grpchealth.StatusServing {
		return fmt.Errorf("health: status %s", h.Status)
	}
	fmt.Fprintln(c.out, "health  SERVING")
	if err := c.formats(ctx); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	if len(names) == 0 {
		return fmt.Errorf("no sample directories in %s", dir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	for _, n := range names {
		s, err := loadSample(filepath.Join(dir, n))
		if err != nil {
			return err
		}
		for _, mode := range []string{"convert", "job"} {
			var content []byte
			if mode == "convert" {
				content, _, err = c.convert(ctx, s)
			} else {
				content, _, err = c.job(ctx, s)
			}
			if err != nil {
				return err
			}
			if !strings.HasPrefix(string(content[:min(len(content), 5)]), "%PDF-") {
				return fmt.Errorf("%s (%s): result is not a PDF", n, mode)
			}
			file := n + ".pdf"
			if mode == "job" {
				file = n + ".job.pdf"
			}
			if err := c.write(filepath.Join(outDir, file), content); err != nil {
				return err
			}
		}
	}
	fmt.Fprintf(c.out, "ok: %d samples converted with Convert and with jobs\n", len(names))
	return nil
}
