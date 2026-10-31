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
	Format  proteus.Format
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

// FromProto validates an API document and options and returns the request.
func FromProto(doc *proteusv1.Document, opts *proteusv1.Options, lim Limits) (*Request, error) {
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
	switch doc.GetFormat() {
	case proteusv1.Format_FORMAT_HTML:
		req.Format = proteus.FormatHTML
	case proteusv1.Format_FORMAT_MARKDOWN:
		req.Format = proteus.FormatMarkdown
	case proteusv1.Format_FORMAT_UNSPECIFIED:
		if req.URL == "" {
			return nil, badf("document format is required (FORMAT_HTML or FORMAT_MARKDOWN)")
		}
		req.Format = proteus.FormatHTML
	default:
		return nil, badf("unsupported format %v", doc.GetFormat())
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
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	return req, nil
}

func validateOptions(o *proteusv1.Options) error {
	if _, ok := pageSizes[o.GetPageSize()]; !ok && o.GetPageSize() != proteusv1.PageSize_PAGE_SIZE_UNSPECIFIED {
		return badf("unknown page_size %v", o.GetPageSize())
	}
	check := func(name string, v float64) error {
		if v < 0 || v > 100 {
			return badf("%s must be between 0 and 100 mm", name)
		}
		return nil
	}
	if o.MarginMm != nil {
		if err := check("margin_mm", o.GetMarginMm()); err != nil {
			return err
		}
	}
	if m := o.GetMargins(); m != nil {
		for _, v := range []float64{m.GetTop(), m.GetRight(), m.GetBottom(), m.GetLeft()} {
			if err := check("margins", v); err != nil {
				return err
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

// engineOptions maps caller options to engine options.
func engineOptions(o *proteusv1.Options) []proteus.Option {
	if o == nil {
		return nil
	}
	var opts []proteus.Option
	if o.GetPageSize() != proteusv1.PageSize_PAGE_SIZE_UNSPECIFIED || o.GetLandscape() {
		size := proteus.A4
		if s, ok := pageSizes[o.GetPageSize()]; ok {
			size = s
		}
		if o.GetLandscape() {
			size = size.Landscape()
		}
		opts = append(opts, proteus.WithPageSize(size))
	}
	switch {
	case o.GetMargins() != nil:
		m := o.GetMargins()
		opts = append(opts, proteus.WithMargins(proteus.Margins{
			Top: proteus.Mm(m.GetTop()), Right: proteus.Mm(m.GetRight()),
			Bottom: proteus.Mm(m.GetBottom()), Left: proteus.Mm(m.GetLeft()),
		}))
	case o.MarginMm != nil:
		opts = append(opts, proteus.WithMargins(proteus.UniformMargins(proteus.Mm(o.GetMarginMm()))))
	}
	if o.GetStylesheet() != "" {
		opts = append(opts, proteus.WithStylesheet(o.GetStylesheet()))
	}
	switch o.GetMediaType() {
	case proteusv1.MediaType_MEDIA_TYPE_SCREEN:
		opts = append(opts, proteus.WithMediaType("screen"))
	case proteusv1.MediaType_MEDIA_TYPE_PRINT:
		opts = append(opts, proteus.WithMediaType("print"))
	}
	if o.Outline != nil {
		opts = append(opts, proteus.WithOutline(o.GetOutline()))
	}
	if o.GetTagged() {
		opts = append(opts, proteus.WithTagged(true))
	}
	if m := o.GetMetadata(); m != nil {
		opts = append(opts, proteus.WithMetadata(proteus.Metadata{
			Title: m.GetTitle(), Author: m.GetAuthor(), Subject: m.GetSubject(),
			Keywords: m.GetKeywords(), Language: m.GetLanguage(),
		}))
	}
	return opts
}

// FileName suggests a download name for a result with the given title.
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
	return title + ".pdf"
}
