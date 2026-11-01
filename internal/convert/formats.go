package convert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/internal/loader"
)

// InputFormat is a document format the server reads.
type InputFormat string

// OutputFormat is a document format the server writes.
type OutputFormat string

const (
	HTML     InputFormat = "html"
	Markdown InputFormat = "markdown"

	PDF OutputFormat = "pdf"
)

// Target describes an output format: how it is served and how its bytes are
// sanity-checked before they leave the server.
type Target struct {
	Format    OutputFormat
	MIMEType  string
	Extension string // with the dot
	Paged     bool   // page options apply
	Check     func([]byte) error
}

// targets lists every output format the API knows. Adding a format means
// adding it here and in the proto, then registering a Converter for it.
var targets = map[OutputFormat]Target{
	PDF: {Format: PDF, MIMEType: "application/pdf", Extension: ".pdf", Paged: true, Check: func(b []byte) error {
		if !bytes.HasPrefix(b, []byte("%PDF-")) {
			return errors.New("engine returned invalid PDF output")
		}
		return nil
	}},
}

// TargetOf returns the description of an output format.
func TargetOf(f OutputFormat) (Target, bool) {
	t, ok := targets[f]
	return t, ok
}

var (
	inputFromProto = map[proteusv1.InputFormat]InputFormat{
		proteusv1.InputFormat_INPUT_FORMAT_HTML:     HTML,
		proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN: Markdown,
	}
	outputFromProto = map[proteusv1.OutputFormat]OutputFormat{
		proteusv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED: PDF,
		proteusv1.OutputFormat_OUTPUT_FORMAT_PDF:         PDF,
	}
	inputToProto  = map[InputFormat]proteusv1.InputFormat{HTML: proteusv1.InputFormat_INPUT_FORMAT_HTML, Markdown: proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN}
	outputToProto = map[OutputFormat]proteusv1.OutputFormat{PDF: proteusv1.OutputFormat_OUTPUT_FORMAT_PDF}
)

// InputToProto maps an input format to its API enum.
func InputToProto(f InputFormat) proteusv1.InputFormat { return inputToProto[f] }

// OutputFromProto maps an API output format (UNSPECIFIED is PDF) to the
// server's; false for values the server does not know.
func OutputFromProto(f proteusv1.OutputFormat) (OutputFormat, bool) {
	o, ok := outputFromProto[f]
	return o, ok
}

// OutputToProto maps an output format to its API enum.
func OutputToProto(f OutputFormat) proteusv1.OutputFormat { return outputToProto[f] }

// Input is what a Converter gets: the document already fetched, plus the
// per-request resource loader that enforces the server's security policy.
// A Converter must load every referenced resource through Loader and never
// touch the local disk or network itself.
type Input struct {
	Format      InputFormat
	Source      []byte
	BaseURL     string // set when the document came from a URL
	Loader      *loader.Loader
	AllowRemote bool
	Options     *proteusv1.Options
	Warn        func(string) // non-fatal problems; safe for concurrent use
}

// Converter turns one input format into one output format.
type Converter interface {
	Convert(ctx context.Context, in *Input) ([]byte, error)
}

// ConverterFunc adapts a function to Converter.
type ConverterFunc func(ctx context.Context, in *Input) ([]byte, error)

func (f ConverterFunc) Convert(ctx context.Context, in *Input) ([]byte, error) { return f(ctx, in) }

// Route is an input to output pair.
type Route struct {
	From InputFormat
	To   OutputFormat
}

func (r Route) String() string { return string(r.From) + "->" + string(r.To) }

// Registry maps routes to converters.
type Registry struct {
	routes map[Route]Converter
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{routes: map[Route]Converter{}} }

// Register adds or replaces the converter for from -> to. It panics on an
// output format missing from the target table, which is a programming error.
func (r *Registry) Register(from InputFormat, to OutputFormat, c Converter) {
	if _, ok := targets[to]; !ok {
		panic(fmt.Sprintf("convert: unknown output format %q", to))
	}
	r.routes[Route{from, to}] = c
}

// Lookup returns the converter for a route.
func (r *Registry) Lookup(rt Route) (Converter, bool) {
	c, ok := r.routes[rt]
	return c, ok
}

// Routes returns every registered route, sorted.
func (r *Registry) Routes() []Route {
	out := make([]Route, 0, len(r.routes))
	for rt := range r.routes {
		out = append(out, rt)
	}
	slices.SortFunc(out, func(a, b Route) int { return strings.Compare(a.String(), b.String()) })
	return out
}
