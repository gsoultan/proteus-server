// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package templates stores reusable HTML and Markdown documents with
// placeholders and fills them with data. Filling produces an ordinary
// proteusv1.Document in the template's input format, so conversion and jobs
// work exactly as for documents sent inline, to any output format the
// converter registry has a route for.
package templates

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/convert"
)

// Settings configure a Service.
type Settings struct {
	MaxPerOwner   int           // templates per API key; default 500
	MaxAssets     int           // assets per template; default 64
	MaxBytes      int64         // content plus assets; default 10 MiB
	MaxOutput     int64         // rendered document; default 20 MiB
	MaxSteps      int64         // list iterations per render; default templates.DefaultMaxSteps
	RenderTimeout time.Duration // default 5s
}

// Service validates, stores and renders templates for API key owners.
type Service struct {
	s     Settings
	store Store
}

// NewService returns a Service over store.
func NewService(store Store, s Settings) *Service {
	if s.MaxPerOwner <= 0 {
		s.MaxPerOwner = 500
	}
	if s.MaxAssets <= 0 {
		s.MaxAssets = 64
	}
	if s.MaxBytes <= 0 {
		s.MaxBytes = 10 << 20
	}
	if s.MaxOutput <= 0 {
		s.MaxOutput = 20 << 20
	}
	if s.RenderTimeout <= 0 {
		s.RenderTimeout = 5 * time.Second
	}
	return &Service{s: s, store: store}
}

func bad(msg string) error { return &convert.BadRequestError{Msg: msg} }

const (
	maxNameLen        = 200
	maxDescriptionLen = 2000
	maxCategoryLen    = 50
	defaultPageSize   = 50
	maxPageSize       = 200
)

// validate checks t and returns the record to store (without owner or id).
func (s *Service) validate(t *proteusv1.Template) (*Record, error) {
	if t == nil {
		return nil, bad("template is required")
	}
	name := strings.TrimSpace(t.GetName())
	switch {
	case name == "":
		return nil, bad("template name is required")
	case utf8.RuneCountInString(name) > maxNameLen:
		return nil, bad("template name is longer than 200 characters")
	case strings.IndexFunc(name, unicode.IsControl) >= 0:
		return nil, bad("template name contains control characters")
	case utf8.RuneCountInString(t.GetDescription()) > maxDescriptionLen:
		return nil, bad("template description is longer than 2000 characters")
	case !utf8.Valid(t.GetContent()):
		return nil, bad("template content is not UTF-8")
	case utf8.RuneCountInString(t.GetCategory()) > maxCategoryLen:
		return nil, bad("template category is longer than 50 characters")
	case strings.IndexFunc(t.GetCategory(), func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }) >= 0:
		return nil, bad("template category must not contain spaces or control characters")
	}
	if err := ValidateFields(t.GetFields()); err != nil {
		return nil, err
	}
	// A visual template is compiled here; the HTML it yields is checked
	// below exactly like hand-written content.
	if t.GetLayout() != nil {
		if t.GetFormat() == proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN {
			return nil, bad("a template with a layout must have format HTML")
		}
		content, err := Compile(t.GetLayout(), t.GetOptions().GetPage())
		if err != nil {
			return nil, bad(err.Error())
		}
		t = proto.CloneOf(t)
		t.Content = content
	}
	// The engine-side checks (format, assets, options) are the same as for
	// an inline document.
	if _, err := convert.FromProto(document(t, t.GetContent()), proteusv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED,
		t.GetOptions(), convert.Limits{MaxAssets: s.s.MaxAssets}); err != nil {
		return nil, err
	}
	size := int64(len(t.GetContent()))
	for _, a := range t.GetAssets() {
		size += int64(len(a.GetData()))
	}
	if size > s.s.MaxBytes {
		return nil, bad("template content and assets are larger than the limit")
	}
	if _, err := Parse(t.GetContent()); err != nil {
		return nil, bad(err.Error())
	}
	// html/template finds context errors (such as an unclosed attribute)
	// only when it first runs, so run it once now. Other failures depend on
	// the data and are reported when rendering.
	ctx, cancel := context.WithTimeout(context.Background(), s.s.RenderTimeout)
	defer cancel()
	if _, err := ExecuteWith(ctx, t.GetContent(), ApplyDefaults(t.GetFields(), t.GetSampleData()), s.limits(), FormatOf(t.GetLayout())); err != nil {
		var te *TemplateError
		if errors.As(err, &te) && te.Escape {
			return nil, bad(te.Msg)
		}
	}
	body, err := proto.Marshal(&proteusv1.Template{
		Content: t.GetContent(), Assets: t.GetAssets(), Options: t.GetOptions(), SampleData: t.GetSampleData(),
		Layout: t.GetLayout(), Fields: t.GetFields(), StarterId: t.GetStarterId(),
	})
	if err != nil {
		return nil, err
	}
	return &Record{
		Name: name, Description: t.GetDescription(), Format: formatName(t.GetFormat()), Category: t.GetCategory(),
		SizeBytes: size, Body: body,
	}, nil
}

func document(t *proteusv1.Template, content []byte) *proteusv1.Document {
	return &proteusv1.Document{
		Format: t.GetFormat(),
		Source: &proteusv1.Document_Content{Content: content},
		Assets: t.GetAssets(),
	}
}

func (s *Service) limits() Limits { return Limits{MaxBytes: s.s.MaxOutput, MaxSteps: s.s.MaxSteps} }

func formatName(f proteusv1.InputFormat) string {
	if f == proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN {
		return string(convert.Markdown)
	}
	return string(convert.HTML)
}

func formatOf(name string) proteusv1.InputFormat {
	return convert.InputToProto(convert.InputFormat(name))
}

// ToProto converts a record; the body is included when the record has one.
func ToProto(r *Record) (*proteusv1.Template, error) {
	t := &proteusv1.Template{}
	if r.Body != nil {
		if err := proto.Unmarshal(r.Body, t); err != nil {
			return nil, err
		}
	}
	t.Id, t.Name, t.Description, t.Format, t.Category = r.ID, r.Name, r.Description, formatOf(r.Format), r.Category
	t.Revision, t.SizeBytes = r.Revision, r.SizeBytes
	t.CreateTime, t.UpdateTime = timestamppb.New(r.CreatedAt), timestamppb.New(r.UpdatedAt)
	return t, nil
}

func hasBlocks(l *proteusv1.Layout) bool {
	return len(l.GetHeader().GetBlocks())+len(l.GetBody().GetBlocks())+len(l.GetFooter().GetBlocks()) > 0
}

// Create stores a new template for owner. With a starter id the starter is
// copied and the fields set in t override it.
func (s *Service) Create(ctx context.Context, owner string, t *proteusv1.Template, starterID string) (*proteusv1.Template, error) {
	if starterID != "" {
		st, ok := Starter(starterID)
		if !ok {
			return nil, bad("unknown starter " + strconv.Quote(starterID))
		}
		merged := proto.CloneOf(st)
		if t != nil {
			proto.Merge(merged, t)
			// A layout with blocks replaces the starter's rather than merging
			// block lists; a layout that only sets the theme (locale, currency,
			// accent) keeps the starter's blocks, which proto.Merge has already
			// combined with the theme.
			if l := t.GetLayout(); l != nil && hasBlocks(l) {
				merged.Layout = l
			}
			if len(t.GetFields()) > 0 {
				merged.Fields = t.GetFields()
			}
			if t.GetSampleData() != nil {
				merged.SampleData = t.GetSampleData()
			}
		}
		merged.StarterId = starterID
		t = merged
	}
	r, err := s.validate(t)
	if err != nil {
		return nil, err
	}
	n, err := s.store.Count(ctx, owner)
	if err != nil {
		return nil, err
	}
	if n >= int64(s.s.MaxPerOwner) {
		return nil, ErrLimit
	}
	r.Owner = owner
	if err := s.store.Create(ctx, r); err != nil {
		return nil, err
	}
	return ToProto(r)
}

// Get returns owner's template.
func (s *Service) Get(ctx context.Context, owner, id string) (*proteusv1.Template, error) {
	r, err := s.store.Get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	return ToProto(r)
}

// List returns a page of owner's templates ordered by name. The page token
// is the last name of the previous page.
func (s *Service) List(ctx context.Context, owner, category string, pageSize int32, pageToken string) ([]*proteusv1.Template, string, error) {
	size := int(pageSize)
	switch {
	case size < 0:
		return nil, "", bad("page_size must not be negative")
	case size == 0:
		size = defaultPageSize
	case size > maxPageSize:
		size = maxPageSize
	}
	after := ""
	if pageToken != "" {
		b, err := base64.RawURLEncoding.DecodeString(pageToken)
		if err != nil || !utf8.Valid(b) {
			return nil, "", bad("invalid page_token")
		}
		after = string(b)
	}
	recs, err := s.store.List(ctx, owner, category, after, size+1)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(recs) > size {
		recs = recs[:size]
		next = base64.RawURLEncoding.EncodeToString([]byte(recs[size-1].Name))
	}
	out := make([]*proteusv1.Template, len(recs))
	for i, r := range recs {
		r.Body = nil
		if out[i], err = ToProto(r); err != nil {
			return nil, "", err
		}
	}
	return out, next, nil
}

// Update replaces owner's template t.Id.
func (s *Service) Update(ctx context.Context, owner string, t *proteusv1.Template) (*proteusv1.Template, error) {
	if t.GetId() == "" {
		return nil, bad("template id is required")
	}
	r, err := s.validate(t)
	if err != nil {
		return nil, err
	}
	r.ID, r.Owner = t.GetId(), owner
	if err := s.store.Update(ctx, r, t.GetRevision()); err != nil {
		return nil, err
	}
	return ToProto(r)
}

// Delete removes owner's template.
func (s *Service) Delete(ctx context.Context, owner, id string) error {
	return s.store.Delete(ctx, owner, id)
}

// Rendered is a filled-in template.
type Rendered struct {
	Name     string // template name ("" for an unsaved template)
	Document *proteusv1.Document
	Options  *proteusv1.Options // template options with the request's merged over them
	data     *structpb.Struct   // the data the document was rendered with, defaults applied
}

// Render fills owner's stored template id with data and merges opts over
// the template's options for conversion to out.
func (s *Service) Render(ctx context.Context, owner, id string, data *structpb.Struct,
	out proteusv1.OutputFormat, opts *proteusv1.Options) (*Rendered, error) {
	if id == "" {
		return nil, bad("template id is required")
	}
	t, err := s.Get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	r, err := s.render(ctx, t, data)
	if err != nil {
		return nil, err
	}
	return r, s.finish(ctx, r, t, out, opts)
}

// finish merges the request's options over the template's and fills a
// watermark text that is itself a template, such as one that stamps PAID
// only on paid invoices; an empty result drops the watermark.
func (s *Service) finish(ctx context.Context, r *Rendered, t *proteusv1.Template, out proteusv1.OutputFormat, opts *proteusv1.Options) error {
	r.Options = mergeOptions(layoutOptions(t), out, opts)
	wm := r.Options.GetPage().GetWatermark()
	text := wm.GetText()
	if !strings.Contains(text, "{{") {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, s.s.RenderTimeout)
	defer cancel()
	got, err := ExecuteWith(ctx, []byte(text), r.data, Limits{MaxBytes: 1 << 10, MaxSteps: 100}, FormatOf(t.GetLayout()))
	if err != nil {
		return bad("watermark text: " + err.Error())
	}
	if rendered := strings.TrimSpace(html.UnescapeString(string(got))); rendered != "" {
		wm.Mark = &proteusv1.Watermark_Text{Text: rendered}
	} else {
		r.Options.Page.Watermark = nil
		if proto.Size(r.Options.Page) == 0 {
			r.Options.Page = nil
		}
	}
	return nil
}

// mergeOptions lays the request's options over the template's defaults.
// Page and PDF defaults that do not apply to the output are dropped, so a
// template written for PDF still converts to other formats; the same
// options in the request are kept and refused by validation.
func mergeOptions(defaults *proteusv1.Options, out proteusv1.OutputFormat, req *proteusv1.Options) *proteusv1.Options {
	merged := &proteusv1.Options{}
	if defaults != nil {
		merged = proto.CloneOf(defaults)
	}
	if target, ok := outputTarget(out); ok {
		if !target.Paged {
			merged.Page = nil
		}
		if target.Format != convert.PDF {
			merged.Pdf = nil
		}
	}
	if req != nil {
		proto.Merge(merged, req)
	}
	return merged
}

func outputTarget(out proteusv1.OutputFormat) (convert.Target, bool) {
	f, ok := convert.OutputFromProto(out)
	if !ok {
		return convert.Target{}, false
	}
	return convert.TargetOf(f)
}

// RenderDraft fills an unsaved template, validating it as Create would.
// With no data it uses the template's sample data.
func (s *Service) RenderDraft(ctx context.Context, t *proteusv1.Template, data *structpb.Struct) (*Rendered, error) {
	r, err := s.RenderConvert(ctx, t, data, proteusv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED, nil)
	if err != nil {
		return nil, err
	}
	r.Options = nil
	return r, nil
}

func (s *Service) render(ctx context.Context, t *proteusv1.Template, data *structpb.Struct) (*Rendered, error) {
	if len(data.GetFields()) == 0 {
		data = t.GetSampleData()
	}
	data = ApplyDefaults(t.GetFields(), data)
	if err := CheckData(t.GetFields(), data); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.s.RenderTimeout)
	defer cancel()
	out, err := ExecuteWith(ctx, t.GetContent(), data, s.limits(), FormatOf(t.GetLayout()))
	if err != nil {
		// Every render failure is down to the template or the data.
		return nil, bad(err.Error())
	}
	return &Rendered{Name: t.GetName(), Document: document(t, out), data: data}, nil
}

// layoutOptions returns the options to convert a template with. A compiled
// layout carries its page size and margins (plus the header and footer
// bands) in its own @page rule, so those are dropped from the options and
// only the watermark is kept.
func layoutOptions(t *proteusv1.Template) *proteusv1.Options {
	o := t.GetOptions()
	if t.GetLayout() == nil || o.GetPage() == nil {
		return o
	}
	o = proto.CloneOf(o)
	o.Page = &proteusv1.PageOptions{Watermark: o.GetPage().GetWatermark()}
	if o.Page.Watermark == nil {
		o.Page = nil
	}
	return o
}

// RenderConvert fills an unsaved template, validating it as Create would,
// and merges opts over its options for conversion to out, for exact
// previews while editing.
func (s *Service) RenderConvert(ctx context.Context, t *proteusv1.Template, data *structpb.Struct,
	out proteusv1.OutputFormat, opts *proteusv1.Options) (*Rendered, error) {
	rec, err := s.validate(t)
	if err != nil {
		return nil, err
	}
	compiled, err := ToProto(rec)
	if err != nil {
		return nil, err
	}
	compiled.Name, compiled.Format = t.GetName(), t.GetFormat()
	r, err := s.render(ctx, compiled, data)
	if err != nil {
		return nil, err
	}
	return r, s.finish(ctx, r, compiled, out, opts)
}

// ConvertToCode turns owner's visual template into a code template: the
// compiled HTML stays as content and the layout is dropped. expected is
// the revision the caller saw (0 to skip the check).
func (s *Service) ConvertToCode(ctx context.Context, owner, id string, expected int64) (*proteusv1.Template, error) {
	t, err := s.Get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if t.GetLayout() == nil {
		return nil, bad("template is already a code template")
	}
	if expected != 0 && t.GetRevision() != expected {
		return nil, ErrRevisionMismatch
	}
	// Keep the compiled @page geometry working by moving the layout's
	// locale and currency defaults into the content's reach: the compiled
	// HTML already formats through the helper functions, which fall back to
	// en-US and USD without a layout, so a theme with other settings is
	// written into the template as a comment the editor can see.
	if f := FormatOf(t.GetLayout()); f.Locale != "" || f.Currency != "" {
		t.Content = append([]byte(fmt.Sprintf("<!-- proteus: locale=%s currency=%s -->\n", f.Locale, f.Currency)), t.GetContent()...)
	}
	t.Layout = nil
	t.Revision = expected
	return s.Update(ctx, owner, t)
}
