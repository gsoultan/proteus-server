package templates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"iter"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template/parse"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// Placeholder syntax is Go's html/template, so every value inserted from
// data is escaped for where it lands (text, attribute, URL, CSS, script) and
// data can never add markup. Markdown templates are treated as HTML text,
// which is correct for Markdown outside code blocks.
//
// Templates are written by API key holders and run on the shared server, so
// the language is narrowed to keep the work of one render bounded:
//   - no {{define}}, {{template}} or {{block}}: nested templates could call
//     each other an exponential number of times;
//   - at most maxRangeDepth nested {{range}}, and none over a number;
//   - data numbers are decimals (Number), which {{range}} refuses;
//   - data lists are iterators (List) that count every step against a
//     budget and stop when the deadline passes, so a render that loops too
//     much fails instead of running on;
//   - data objects have at most maxObjectKeys keys;
//   - output is capped.

const (
	maxRangeDepth = 3
	maxObjectKeys = 500
	// DefaultMaxSteps is the default budget of list iterations per render.
	DefaultMaxSteps = 1_000_000
)

var (
	// ErrTooLarge is returned when the rendered document exceeds the limit.
	ErrTooLarge = errors.New("rendered document is too large")
	// ErrRenderTimeout is returned when rendering exceeds its deadline.
	ErrRenderTimeout = errors.New("rendering the template took too long")
	// ErrTooManySteps is returned when loops exceed the step budget.
	ErrTooManySteps = errors.New("the template loops over the data too many times")
)

// TemplateError is a problem in the template text or with the data, reported
// to the caller.
type TemplateError struct {
	Msg string
	// Escape is set for html/template context errors, which are faults in
	// the template text found only when it first runs.
	Escape bool
}

func (e *TemplateError) Error() string { return e.Msg }

// Number is a data number. It prints without an exponent (1234567, 19.99)
// and, being a float, cannot drive {{range}}. Compare it with decimals:
// {{if gt .total 100.0}}.
type Number float64

func (n Number) String() string { return strconv.FormatFloat(float64(n), 'f', -1, 64) }

// List is a non-empty data list. Ranging over it yields the elements and
// spends the render's step budget, as do len, index and join, which walk
// it. An empty list is a plain []any so that {{if .items}} is false for it.
type List iter.Seq[any]

func (r *run) list(items []any) List {
	return func(yield func(any) bool) {
		for _, v := range items {
			if !r.step() || !yield(v) {
				return
			}
		}
	}
}

// pairs yields index and element; the parser routes two-variable ranges
// ({{range $i, $e := .items}}) through it.
func (r *run) pairs(v any) (iter.Seq2[any, any], error) {
	switch x := v.(type) {
	case List:
		return func(yield func(any, any) bool) {
			i := 0
			for e := range x {
				if !yield(i, e) {
					return
				}
				i++
			}
		}, nil
	case []any:
		return func(func(any, any) bool) {}, nil
	case map[string]any:
		return func(yield func(any, any) bool) {
			for _, k := range slices.Sorted(maps.Keys(x)) {
				if !yield(k, x[k]) {
					return
				}
			}
		}, nil
	case nil:
		return func(func(any, any) bool) {}, nil
	}
	return nil, fmt.Errorf("can't range over %T", v)
}

// run is the state of one render.
type run struct {
	ctx      context.Context
	left     atomic.Int64
	exceeded atomic.Bool
}

func (r *run) step() bool {
	if r.left.Add(-1) < 0 {
		r.exceeded.Store(true)
		return false
	}
	return r.ctx.Err() == nil
}

// items returns the elements of a data list, spending a step for each.
func (r *run) items(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case List:
		var out []any
		for e := range x {
			out = append(out, e)
		}
		return out, true
	}
	return nil, false
}

func (r *run) funcs() template.FuncMap {
	return template.FuncMap{
		"pairs": r.pairs,
		// join joins a list: {{join ", " .tags}}.
		"join": func(sep string, v any) (string, error) {
			items, ok := r.items(v)
			if !ok && v != nil {
				return "", fmt.Errorf("join: %T is not a list", v)
			}
			parts := make([]string, len(items))
			for i, e := range items {
				parts[i] = fmt.Sprint(e)
			}
			return strings.Join(parts, sep), nil
		},
		// len and index replace the built-ins so that they accept lists.
		"len": func(v any) (int, error) {
			if items, ok := r.items(v); ok {
				return len(items), nil
			}
			switch x := v.(type) {
			case string:
				return len(x), nil
			case map[string]any:
				return len(x), nil
			case nil:
				return 0, nil
			}
			return 0, fmt.Errorf("len of %T", v)
		},
		"index": func(v any, keys ...any) (any, error) {
			for _, k := range keys {
				if items, ok := r.items(v); ok {
					i, ok := toFloat(k)
					if !ok || i != math.Trunc(i) || i < 0 || int(i) >= len(items) {
						return nil, fmt.Errorf("index %v out of range", k)
					}
					v = items[int(i)]
					continue
				}
				m, ok := v.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("can't index %T", v)
				}
				v = m[fmt.Sprint(k)]
			}
			return v, nil
		},
	}
}

// staticFuncs need no render state.
var staticFuncs = template.FuncMap{
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"trim":  strings.TrimSpace,
	// default returns def when v is empty: {{default "n/a" .phone}}.
	"default": func(def, v any) any {
		if empty(v) {
			return def
		}
		return v
	},
	// date reformats an RFC 3339 or YYYY-MM-DD string with a Go layout:
	// {{date "2 Jan 2006" .issued}}. Anything else is returned unchanged.
	"date": func(layout string, v any) string {
		s := fmt.Sprint(v)
		for _, in := range []string{time.RFC3339Nano, time.DateOnly} {
			if t, err := time.Parse(in, s); err == nil {
				return t.Format(layout)
			}
		}
		return s
	},
	// number formats a number with a fixed count of decimals and thousands
	// separators: {{number 2 .total}} -> 1,234.50.
	"number": func(decimals int, v any) (string, error) {
		f, ok := toFloat(v)
		if !ok {
			return "", fmt.Errorf("number: %v is not a number", v)
		}
		if decimals < 0 || decimals > 10 {
			return "", errors.New("number: decimals must be 0 to 10")
		}
		return groupThousands(strconv.FormatFloat(f, 'f', decimals, 64)), nil
	},
}

func empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return !x
	case Number:
		return x == 0
	case []any:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case Number:
		return float64(x), true
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}

func groupThousands(s string) string {
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	intPart, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if hasFrac {
		return sign + b.String() + "." + frac
	}
	return sign + b.String()
}

// Parse compiles and checks a template.
func Parse(content []byte) (*template.Template, error) { return (&run{}).parse(content) }

func (r *run) parse(content []byte) (*template.Template, error) {
	t, err := template.New("template").Funcs(staticFuncs).Funcs(r.funcs()).
		Option("missingkey=zero").Parse(string(content))
	if err != nil {
		return nil, &TemplateError{Msg: cleanErr(err)}
	}
	if n := len(t.Templates()); n > 1 {
		return nil, &TemplateError{Msg: "template: {{define}} and {{block}} are not supported"}
	}
	if t.Tree != nil && t.Tree.Root != nil {
		c := checker{tree: t.Tree}
		if err := c.list(t.Tree.Root, 0); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// checker walks a parse tree, refuses the constructs Parse documents and
// routes two-variable ranges through pairs.
type checker struct{ tree *parse.Tree }

func (c checker) fail(pos parse.Pos, msg string) error {
	loc, _ := c.tree.ErrorContext(&parse.TextNode{NodeType: parse.NodeText, Pos: pos})
	if _, rest, ok := strings.Cut(loc, ":"); ok {
		if line, _, ok := strings.Cut(rest, ":"); ok {
			return &TemplateError{Msg: "template: line " + line + ": " + msg}
		}
	}
	return &TemplateError{Msg: "template: " + msg}
}

func (c checker) list(l *parse.ListNode, depth int) error {
	if l == nil {
		return nil
	}
	for _, n := range l.Nodes {
		if err := c.node(n, depth); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) node(n parse.Node, depth int) error {
	switch x := n.(type) {
	case *parse.TemplateNode:
		return c.fail(x.Pos, "{{template}} is not supported")
	case *parse.RangeNode:
		if depth+1 > maxRangeDepth {
			return c.fail(x.Pos, fmt.Sprintf("{{range}} is nested more than %d deep", maxRangeDepth))
		}
		if x.Pipe != nil {
			for _, cmd := range x.Pipe.Cmds {
				for _, a := range cmd.Args {
					if _, ok := a.(*parse.NumberNode); ok {
						return c.fail(x.Pos, "{{range}} over a number is not supported")
					}
				}
			}
			if len(x.Pipe.Decl) == 2 {
				x.Pipe.Cmds = append(x.Pipe.Cmds, &parse.CommandNode{
					NodeType: parse.NodeCommand, Pos: x.Pos,
					Args: []parse.Node{parse.NewIdentifier("pairs").SetTree(c.tree).SetPos(x.Pos)},
				})
			}
		}
		if err := c.list(x.List, depth+1); err != nil {
			return err
		}
		return c.list(x.ElseList, depth)
	case *parse.IfNode:
		return c.branch(&x.BranchNode, depth)
	case *parse.WithNode:
		return c.branch(&x.BranchNode, depth)
	}
	return nil
}

func (c checker) branch(b *parse.BranchNode, depth int) error {
	if err := c.list(b.List, depth); err != nil {
		return err
	}
	return c.list(b.ElseList, depth)
}

// cleanErr strips Go package prefixes from template errors.
func cleanErr(err error) string {
	s := err.Error()
	s = strings.TrimPrefix(s, "html/template:")
	s = strings.TrimPrefix(s, "template: ")
	return "template: " + strings.TrimPrefix(s, "template:")
}

// data converts a request's data to template values: objects become maps,
// lists List (or []any when empty), numbers Number.
func (r *run) data(s *structpb.Struct) (map[string]any, error) {
	if len(s.GetFields()) > maxObjectKeys {
		return nil, &TemplateError{Msg: fmt.Sprintf("data: an object has more than %d keys; use a list", maxObjectKeys)}
	}
	out := make(map[string]any, len(s.GetFields()))
	for k, v := range s.GetFields() {
		x, err := r.value(v)
		if err != nil {
			return nil, err
		}
		out[k] = x
	}
	return out, nil
}

func (r *run) value(v *structpb.Value) (any, error) {
	switch k := v.GetKind().(type) {
	case *structpb.Value_StringValue:
		return k.StringValue, nil
	case *structpb.Value_NumberValue:
		if math.IsNaN(k.NumberValue) || math.IsInf(k.NumberValue, 0) {
			return nil, nil
		}
		return Number(k.NumberValue), nil
	case *structpb.Value_BoolValue:
		return k.BoolValue, nil
	case *structpb.Value_StructValue:
		return r.data(k.StructValue)
	case *structpb.Value_ListValue:
		vals := k.ListValue.GetValues()
		items := make([]any, len(vals))
		for i, e := range vals {
			x, err := r.value(e)
			if err != nil {
				return nil, err
			}
			items[i] = x
		}
		if len(items) == 0 {
			return items, nil
		}
		return r.list(items), nil
	}
	return nil, nil
}

// limitWriter fails once more than n bytes are written.
type limitWriter struct {
	buf bytes.Buffer
	n   int64
}

func (w *limitWriter) Write(p []byte) (int, error) {
	if int64(w.buf.Len()+len(p)) > w.n {
		return 0, ErrTooLarge
	}
	return w.buf.Write(p)
}

// Limits bound one render.
type Limits struct {
	MaxBytes int64 // output size
	MaxSteps int64 // list iterations; default DefaultMaxSteps
}

// Execute parses content and fills it with data. Output over the limit is
// ErrTooLarge, loops over the budget ErrTooManySteps, and passing ctx's
// deadline ErrRenderTimeout. Other failures are *TemplateError.
func Execute(ctx context.Context, content []byte, data *structpb.Struct, lim Limits) ([]byte, error) {
	if lim.MaxSteps <= 0 {
		lim.MaxSteps = DefaultMaxSteps
	}
	r := &run{ctx: ctx}
	r.left.Store(lim.MaxSteps)
	// Parsed per render: the list functions are bound to this render.
	t, err := r.parse(content)
	if err != nil {
		return nil, err
	}
	values, err := r.data(data)
	if err != nil {
		return nil, err
	}
	w := &limitWriter{n: lim.MaxBytes}
	err = t.Execute(w, values)
	switch {
	case r.exceeded.Load():
		return nil, fmt.Errorf("%w (limit %d steps)", ErrTooManySteps, lim.MaxSteps)
	case ctx.Err() != nil:
		return nil, ErrRenderTimeout
	case err == nil:
		return w.buf.Bytes(), nil
	case errors.Is(err, ErrTooLarge):
		return nil, fmt.Errorf("%w (limit %d bytes)", ErrTooLarge, lim.MaxBytes)
	}
	var esc *template.Error
	return nil, &TemplateError{Msg: cleanErr(err), Escape: errors.As(err, &esc)}
}
