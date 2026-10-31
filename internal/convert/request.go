// Package convert turns API requests into calls to the proteus-converter engine.
// It holds no conversion logic of its own: it parses and validates input,
// builds engine options, and enforces concurrency and time limits.
package convert

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	proteus "github.com/gsoultan/proteus-converter"

	"github.com/gsoultan/proteus-server/internal/loader"
)

// Request is one conversion, independent of how it arrived (JSON or multipart).
type Request struct {
	Format  proteus.Format
	Source  []byte            // document body; empty when URL is set
	URL     string            // fetch the document from here instead
	Assets  map[string][]byte // uploaded resources keyed by path
	Options Options

	// Async jobs only.
	WebhookURL string
}

// Options are the engine options a caller may set. Everything else (fonts,
// resource access) is decided by the server.
type Options struct {
	PageSize    string    `json:"page_size,omitempty"` // A3, A4, A5, Letter, Legal
	Landscape   bool      `json:"landscape,omitempty"`
	MarginMM    *float64  `json:"margin_mm,omitempty"` // all sides
	Margins     *Margins  `json:"margins_mm,omitempty"`
	Stylesheet  string    `json:"stylesheet,omitempty"` // extra CSS
	MediaType   string    `json:"media_type,omitempty"` // print (default) or screen
	Outline     *bool     `json:"outline,omitempty"`
	Tagged      bool      `json:"tagged,omitempty"`
	Metadata    *Metadata `json:"metadata,omitempty"`
	AllowRemote bool      `json:"allow_remote,omitempty"` // fetch http(s) resources, if the server permits it
}

// Margins in millimetres.
type Margins struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// Metadata written to the PDF information dictionary.
type Metadata struct {
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Keywords string `json:"keywords,omitempty"`
	Language string `json:"language,omitempty"`
}

// jsonBody is the application/json request shape.
type jsonBody struct {
	Format     string  `json:"format,omitempty"` // html or markdown; inferred from the field used
	HTML       string  `json:"html,omitempty"`
	Markdown   string  `json:"markdown,omitempty"`
	URL        string  `json:"url,omitempty"`
	Options    Options `json:"options"`
	WebhookURL string  `json:"webhook_url,omitempty"`
}

// BadRequestError is an input problem the caller can fix.
type BadRequestError struct{ Msg string }

func (e *BadRequestError) Error() string { return e.Msg }

func badf(format string, a ...any) error { return &BadRequestError{Msg: fmt.Sprintf(format, a...)} }

// ErrBodyTooLarge is returned when the request body exceeds the configured limit.
var ErrBodyTooLarge = errors.New("request body too large")

// ParseHTTP reads a conversion request from r. The body must already be
// wrapped in http.MaxBytesReader.
func ParseHTTP(r *http.Request, maxAssets int) (*Request, error) {
	ct, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, badf("missing or invalid Content-Type")
	}
	switch ct {
	case "application/json":
		return parseJSON(r.Body)
	case "multipart/form-data":
		return parseMultipart(multipart.NewReader(r.Body, params["boundary"]), maxAssets)
	case "text/html":
		return rawBody(r.Body, proteus.FormatHTML)
	case "text/markdown", "text/x-markdown":
		return rawBody(r.Body, proteus.FormatMarkdown)
	default:
		return nil, badf("unsupported Content-Type %q (use application/json, multipart/form-data, text/html or text/markdown)", ct)
	}
}

func readAll(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(r)
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return nil, ErrBodyTooLarge
	}
	return b, err
}

func rawBody(body io.Reader, f proteus.Format) (*Request, error) {
	b, err := readAll(body)
	if err != nil {
		return nil, err
	}
	req := &Request{Format: f, Source: b}
	return req, req.Validate()
}

func parseJSON(body io.Reader) (*Request, error) {
	b, err := readAll(body)
	if err != nil {
		return nil, err
	}
	var jb jsonBody
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&jb); err != nil {
		return nil, badf("invalid JSON: %v", err)
	}
	set := 0
	req := &Request{Options: jb.Options, WebhookURL: jb.WebhookURL}
	if jb.HTML != "" {
		set++
		req.Format, req.Source = proteus.FormatHTML, []byte(jb.HTML)
	}
	if jb.Markdown != "" {
		set++
		req.Format, req.Source = proteus.FormatMarkdown, []byte(jb.Markdown)
	}
	if jb.URL != "" {
		set++
		req.URL = jb.URL
		req.Format = proteus.FormatHTML
	}
	if set != 1 {
		return nil, badf("exactly one of html, markdown or url is required")
	}
	if jb.Format != "" {
		f, err := parseFormat(jb.Format)
		if err != nil {
			return nil, err
		}
		req.Format = f
	}
	return req, req.Validate()
}

// parseMultipart reads a "file" part (the document), an optional "options"
// part (JSON Options), optional "format" and "webhook_url" parts, and any
// number of other file parts, which become assets addressed by file name.
func parseMultipart(mr *multipart.Reader, maxAssets int) (*Request, error) {
	req := &Request{Assets: map[string][]byte{}}
	var fileName, format string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return nil, ErrBodyTooLarge
			}
			return nil, badf("invalid multipart body: %v", err)
		}
		data, err := readAll(p)
		if err != nil {
			return nil, err
		}
		switch name := p.FormName(); name {
		case "file":
			req.Source, fileName = data, p.FileName()
		case "options":
			dec := json.NewDecoder(bytes.NewReader(data))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req.Options); err != nil {
				return nil, badf("invalid options JSON: %v", err)
			}
		case "format":
			format = strings.TrimSpace(string(data))
		case "webhook_url":
			req.WebhookURL = strings.TrimSpace(string(data))
		default:
			fn := rawFileName(p)
			if fn == "" {
				return nil, badf("unknown form field %q", name)
			}
			key, ok := loader.CleanAssetPath(fn)
			if !ok {
				return nil, badf("invalid asset file name %q", fn)
			}
			if len(req.Assets) >= maxAssets {
				return nil, badf("too many assets (max %d)", maxAssets)
			}
			req.Assets[key] = data
		}
		p.Close()
	}
	if req.Source == nil {
		return nil, badf(`multipart body needs a "file" part with the document`)
	}
	switch {
	case format != "":
		f, err := parseFormat(format)
		if err != nil {
			return nil, err
		}
		req.Format = f
	default:
		f, err := proteus.FormatForPath(fileName)
		if err != nil {
			return nil, badf(`cannot tell the format from file name %q; add a "format" field`, fileName)
		}
		req.Format = f
	}
	return req, req.Validate()
}

// rawFileName returns the filename parameter as sent. Part.FileName strips
// directories, but assets keep theirs so "img/logo.png" matches the document.
func rawFileName(p *multipart.Part) string {
	_, params, err := mime.ParseMediaType(p.Header.Get("Content-Disposition"))
	if err != nil {
		return p.FileName()
	}
	return params["filename"]
}

func parseFormat(s string) (proteus.Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "html", "htm":
		return proteus.FormatHTML, nil
	case "markdown", "md":
		return proteus.FormatMarkdown, nil
	}
	return "", badf("unsupported format %q (html or markdown)", s)
}

// Validate checks option values that the engine would otherwise accept silently.
func (r *Request) Validate() error {
	if r.Format != proteus.FormatHTML && r.Format != proteus.FormatMarkdown {
		return badf("unsupported format %q", r.Format)
	}
	if r.URL == "" && len(r.Source) == 0 {
		return badf("document is empty")
	}
	o := r.Options
	if _, ok := pageSizes[strings.ToLower(o.PageSize)]; o.PageSize != "" && !ok {
		return badf("unknown page_size %q (A3, A4, A5, Letter, Legal)", o.PageSize)
	}
	check := func(name string, v float64) error {
		if v < 0 || v > 100 {
			return badf("%s must be between 0 and 100 mm", name)
		}
		return nil
	}
	if o.MarginMM != nil {
		if err := check("margin_mm", *o.MarginMM); err != nil {
			return err
		}
	}
	if m := o.Margins; m != nil {
		for _, v := range []float64{m.Top, m.Right, m.Bottom, m.Left} {
			if err := check("margins_mm", v); err != nil {
				return err
			}
		}
	}
	if o.MediaType != "" && o.MediaType != "print" && o.MediaType != "screen" {
		return badf("media_type must be print or screen")
	}
	if len(o.Stylesheet) > 1<<20 {
		return badf("stylesheet is larger than 1 MiB")
	}
	return nil
}

var pageSizes = map[string]proteus.PageSize{
	"a3": proteus.A3, "a4": proteus.A4, "a5": proteus.A5,
	"letter": proteus.Letter, "legal": proteus.Legal,
}

// engineOptions maps caller options to engine options.
func (o Options) engineOptions() []proteus.Option {
	var opts []proteus.Option
	if o.PageSize != "" || o.Landscape {
		size := proteus.A4
		if s, ok := pageSizes[strings.ToLower(o.PageSize)]; ok {
			size = s
		}
		if o.Landscape {
			size = size.Landscape()
		}
		opts = append(opts, proteus.WithPageSize(size))
	}
	switch {
	case o.Margins != nil:
		m := o.Margins
		opts = append(opts, proteus.WithMargins(proteus.Margins{
			Top: proteus.Mm(m.Top), Right: proteus.Mm(m.Right), Bottom: proteus.Mm(m.Bottom), Left: proteus.Mm(m.Left),
		}))
	case o.MarginMM != nil:
		opts = append(opts, proteus.WithMargins(proteus.UniformMargins(proteus.Mm(*o.MarginMM))))
	}
	if o.Stylesheet != "" {
		opts = append(opts, proteus.WithStylesheet(o.Stylesheet))
	}
	if o.MediaType != "" {
		opts = append(opts, proteus.WithMediaType(o.MediaType))
	}
	if o.Outline != nil {
		opts = append(opts, proteus.WithOutline(*o.Outline))
	}
	if o.Tagged {
		opts = append(opts, proteus.WithTagged(true))
	}
	if m := o.Metadata; m != nil {
		opts = append(opts, proteus.WithMetadata(proteus.Metadata{
			Title: m.Title, Author: m.Author, Subject: m.Subject, Keywords: m.Keywords, Language: m.Language,
		}))
	}
	return opts
}

// FileName suggests a download name for the result.
func FileName(title string) string {
	title = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r == ' ' || r == '.':
			return '-'
		}
		return -1
	}, title)
	if title == "" {
		title = "document"
	}
	if len(title) > 64 {
		title = title[:64]
	}
	return filepath.Base(title) + ".pdf"
}
