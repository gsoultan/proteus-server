package convert

import (
	"context"
	"fmt"

	proteus "github.com/gsoultan/proteus-converter"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// ProteusEngine converts HTML and Markdown to PDF and XLSX with proteus-converter.
type ProteusEngine struct {
	FontDirs    []string
	SystemFonts bool
	MaxResource int64 // per resource, 0 for the engine default

	// convert is proteus.ConvertBytesTo; tests replace it.
	convert func(ctx context.Context, f proteus.Format, o proteus.OutputFormat, src []byte, opts ...proteus.Option) ([]byte, error)
}

// engineOutputs maps the server's output formats to the engine's.
var engineOutputs = map[OutputFormat]proteus.OutputFormat{PDF: proteus.OutputPDF, XLSX: proteus.OutputXLSX}

// Register adds the engine's routes (html and markdown to pdf and xlsx) to r.
func (e *ProteusEngine) Register(r *Registry) {
	for _, in := range []InputFormat{HTML, Markdown} {
		for _, out := range []OutputFormat{PDF, XLSX} {
			r.Register(in, out, e)
		}
	}
}

// Convert implements Converter.
func (e *ProteusEngine) Convert(ctx context.Context, in *Input) ([]byte, error) {
	conv := e.convert
	if conv == nil {
		conv = proteus.ConvertBytesTo
	}
	out, ok := engineOutputs[in.Output]
	if !ok {
		return nil, fmt.Errorf("convert: engine has no %s output", in.Output)
	}
	opts := []proteus.Option{
		proteus.WithResourceLoader(in.Loader),
		proteus.WithLocalFiles(false),
		proteus.WithHTTP(in.AllowRemote),
		proteus.WithSystemFonts(e.SystemFonts),
	}
	if e.MaxResource > 0 {
		opts = append(opts, proteus.WithMaxResourceSize(e.MaxResource))
	}
	if in.Warn != nil {
		opts = append(opts, proteus.WithWarningHandler(in.Warn))
	}
	if in.BaseURL != "" {
		opts = append(opts, proteus.WithBaseURL(in.BaseURL))
	}
	for _, d := range e.FontDirs {
		opts = append(opts, proteus.WithFontDir(d))
	}
	opts = append(opts, engineOptions(in.Options)...)
	f := proteus.FormatHTML
	if in.Format == Markdown {
		f = proteus.FormatMarkdown
	}
	return conv(ctx, f, out, in.Source, opts...)
}

// engineOptions maps caller options to engine options.
func engineOptions(o *proteusv1.Options) []proteus.Option {
	if o == nil {
		return nil
	}
	var opts []proteus.Option
	if p := o.GetPage(); p != nil {
		if p.GetSize() != proteusv1.PageSize_PAGE_SIZE_UNSPECIFIED || p.GetLandscape() {
			size := proteus.A4
			if s, ok := pageSizes[p.GetSize()]; ok {
				size = s
			}
			if p.GetLandscape() {
				size = size.Landscape()
			}
			opts = append(opts, proteus.WithPageSize(size))
		}
		switch {
		case p.GetMargins() != nil:
			m := p.GetMargins()
			opts = append(opts, proteus.WithMargins(proteus.Margins{
				Top: proteus.Mm(m.GetTop()), Right: proteus.Mm(m.GetRight()),
				Bottom: proteus.Mm(m.GetBottom()), Left: proteus.Mm(m.GetLeft()),
			}))
		case p.MarginMm != nil:
			opts = append(opts, proteus.WithMargins(proteus.UniformMargins(proteus.Mm(p.GetMarginMm()))))
		}
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
	if pdf := o.GetPdf(); pdf != nil {
		if pdf.Outline != nil {
			opts = append(opts, proteus.WithOutline(pdf.GetOutline()))
		}
		if pdf.GetTagged() {
			opts = append(opts, proteus.WithTagged(true))
		}
	}
	if sp := o.GetSpreadsheet(); sp != nil && sp.DetectNumbers != nil {
		opts = append(opts, proteus.WithNumberDetection(sp.GetDetectNumbers()))
	}
	if m := o.GetMetadata(); m != nil {
		opts = append(opts, proteus.WithMetadata(proteus.Metadata{
			Title: m.GetTitle(), Author: m.GetAuthor(), Subject: m.GetSubject(),
			Keywords: m.GetKeywords(), Language: m.GetLanguage(),
		}))
	}
	return opts
}
