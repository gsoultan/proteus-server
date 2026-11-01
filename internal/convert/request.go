// Package convert turns API requests into calls to the proteus-converter engine.
// It holds no conversion logic of its own: it validates input, builds engine
// options, and enforces concurrency and time limits.
package convert

import (
	"fmt"
	"strings"

	proteus "github.com/gsoultan/proteus-converter"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/loader"
)

// Request is one validated conversion.
type Request struct {
	Format  InputFormat
	Output  OutputFormat
	Source  []byte            // document body; empty when URL is set
	URL     string            // fetch the document from here instead
	Assets  map[string][]byte // uploaded resources keyed by clean path
	Options *proteusv1.Options
}

// BadRequestError is an input problem the caller can fix.
type BadRequestError struct{ Msg string }

func (e *BadRequestError) Error() string { return e.Msg }

func badf(format string, a ...any) error { return &BadRequestError{Msg: fmt.Sprintf(format, a...)} }

// Limits bound what a request may carry.
type Limits struct {
	MaxAssets int // default 64
}

// Route is the conversion the request asks for.
func (r *Request) Route() Route { return Route{From: r.Format, To: r.Output} }

// FromProto validates an API document, output format and options and returns
// the request. Whether the server supports the route is checked by
// Service.Check.
func FromProto(doc *proteusv1.Document, out proteusv1.OutputFormat, opts *proteusv1.Options, lim Limits) (*Request, error) {
	if lim.MaxAssets <= 0 {
		lim.MaxAssets = 64
	}
	if doc == nil {
		return nil, badf("document is required")
	}
	if opts == nil {
		opts = &proteusv1.Options{}
	}
	req := &Request{Options: opts}
	var ok bool
	if req.Output, ok = outputFromProto[out]; !ok {
		return nil, badf("unsupported output_format %v", out)
	}
	switch src := doc.GetSource().(type) {
	case *proteusv1.Document_Content:
		if len(src.Content) == 0 {
			return nil, badf("document content is empty")
		}
		req.Source = src.Content
	case *proteusv1.Document_Url:
		if strings.TrimSpace(src.Url) == "" {
			return nil, badf("document url is empty")
		}
		req.URL = strings.TrimSpace(src.Url)
	default:
		return nil, badf("document needs content or url")
	}
	switch f := doc.GetFormat(); {
	case f == proteusv1.InputFormat_INPUT_FORMAT_UNSPECIFIED:
		if req.URL == "" {
			return nil, badf("document format is required (INPUT_FORMAT_HTML or INPUT_FORMAT_MARKDOWN)")
		}
		req.Format = HTML
	default:
		if req.Format, ok = inputFromProto[f]; !ok {
			return nil, badf("unsupported document format %v", f)
		}
	}
	if n := len(doc.GetAssets()); n > lim.MaxAssets {
		return nil, badf("too many assets: %d (max %d)", n, lim.MaxAssets)
	}
	if len(doc.GetAssets()) > 0 {
		req.Assets = make(map[string][]byte, len(doc.GetAssets()))
		for _, a := range doc.GetAssets() {
			key, ok := loader.CleanAssetPath(a.GetPath())
			if !ok {
				return nil, badf("invalid asset path %q", a.GetPath())
			}
			if _, dup := req.Assets[key]; dup {
				return nil, badf("duplicate asset path %q", key)
			}
			req.Assets[key] = a.GetData()
		}
	}
	if err := validateOptions(opts, targets[req.Output]); err != nil {
		return nil, err
	}
	return req, nil
}

func validateOptions(o *proteusv1.Options, t Target) error {
	if o.GetPdf() != nil && t.Format != PDF {
		return badf("pdf options need output_format PDF")
	}
	if p := o.GetPage(); p != nil {
		if !t.Paged {
			return badf("page options do not apply to %s output", t.Format)
		}
		if _, ok := pageSizes[p.GetSize()]; !ok && p.GetSize() != proteusv1.PageSize_PAGE_SIZE_UNSPECIFIED {
			return badf("unknown page size %v", p.GetSize())
		}
		check := func(name string, v float64) error {
			if v < 0 || v > 100 {
				return badf("%s must be between 0 and 100 mm", name)
			}
			return nil
		}
		if p.MarginMm != nil {
			if err := check("margin_mm", p.GetMarginMm()); err != nil {
				return err
			}
		}
		if m := p.GetMargins(); m != nil {
			for _, v := range []float64{m.GetTop(), m.GetRight(), m.GetBottom(), m.GetLeft()} {
				if err := check("margins", v); err != nil {
					return err
				}
			}
		}
	}
	switch o.GetMediaType() {
	case proteusv1.MediaType_MEDIA_TYPE_UNSPECIFIED, proteusv1.MediaType_MEDIA_TYPE_PRINT, proteusv1.MediaType_MEDIA_TYPE_SCREEN:
	default:
		return badf("unknown media_type %v", o.GetMediaType())
	}
	if len(o.GetStylesheet()) > 1<<20 {
		return badf("stylesheet is larger than 1 MiB")
	}
	return nil
}

var pageSizes = map[proteusv1.PageSize]proteus.PageSize{
	proteusv1.PageSize_PAGE_SIZE_A3:     proteus.A3,
	proteusv1.PageSize_PAGE_SIZE_A4:     proteus.A4,
	proteusv1.PageSize_PAGE_SIZE_A5:     proteus.A5,
	proteusv1.PageSize_PAGE_SIZE_LETTER: proteus.Letter,
	proteusv1.PageSize_PAGE_SIZE_LEGAL:  proteus.Legal,
}

// FileName suggests a download name for a result with the given title and
// output format.
func FileName(title string, f OutputFormat) string {
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
	return title + targets[f].Extension
}
