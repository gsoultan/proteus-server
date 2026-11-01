package templates

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

func render(t *testing.T, src string, data map[string]any) (string, error) {
	t.Helper()
	out, err := Execute(context.Background(), []byte(src), data_(t, data), Limits{MaxBytes: 1 << 20})
	return string(out), err
}

func data_(t *testing.T, data map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRender(t *testing.T) {
	data := map[string]any{
		"name":  `Ann <script>alert(1)</script> & "Co"`,
		"url":   "javascript:alert(1)",
		"total": 1234567.5,
		"qty":   3,
		"paid":  true,
		"items": []any{map[string]any{"sku": "A"}, map[string]any{"sku": "B"}},
		"tags":  []any{"x", "y"},
		"date":  "2026-10-31",
		"empty": "",
		"none":  []any{},
		"obj":   map[string]any{"b": 2, "a": 1},
	}
	cases := []struct{ src, want string }{
		{`<p>{{.name}}</p>`, `<p>Ann &lt;script&gt;alert(1)&lt;/script&gt; &amp; &#34;Co&#34;</p>`},
		{`<a href="{{.url}}">x</a>`, `<a href="#ZgotmplZ">x</a>`},
		{`{{.total}} {{.qty}}`, `1234567.5 3`},
		{`{{number 2 .total}}`, `1,234,567.50`},
		{`{{if .paid}}PAID{{end}}{{if gt .qty 2.0}} many{{end}}`, `PAID many`},
		{`{{range .items}}[{{.sku}}]{{else}}none{{end}}`, `[A][B]`},
		{`{{range $i, $e := .items}}{{$i}}={{$e.sku}} {{end}}`, `0=A 1=B `},
		{`{{join ", " .tags}} {{len .tags}} {{index .tags 1}} {{index .items 0 "sku"}} {{len .none}}`, `x, y 2 y A 0`},
		{`{{if .none}}full{{else}}empty{{end}} {{if .tags}}full{{end}}`, `empty full`},
		{`{{range $k, $v := .obj}}{{$k}}={{$v}};{{end}}`, `a=1;b=2;`},
		{`{{date "2 Jan 2006" .date}}`, `31 Oct 2026`},
		{`{{upper "a"}}{{lower "B"}}{{trim "  c "}}`, `Abc`},
		{`{{default "n/a" .empty}} {{default "n/a" .missing}} {{default "n/a" .qty}}`, `n/a n/a 3`},
		{`[{{.missing}}]`, `[]`},
		{"# Title\n\n- {{.name}}\n", "# Title\n\n- Ann &lt;script&gt;alert(1)&lt;/script&gt; &amp; &#34;Co&#34;\n"},
	}
	for _, c := range cases {
		got, err := render(t, c.src, data)
		if err != nil || got != c.want {
			t.Errorf("%s\n got %q, %v\nwant %q", c.src, got, err, c.want)
		}
	}
}

func TestRefused(t *testing.T) {
	cases := map[string]string{
		`{{define "x"}}a{{end}}{{template "x"}}`: "not supported",
		`{{block "x" .}}a{{end}}`:                "not supported",
		`{{range 1000000000}}{{end}}`:            "over a number",
		`{{range .a}}{{range .b}}{{range .c}}{{range .d}}{{end}}{{end}}{{end}}{{end}}`:                                   "nested more than 3",
		`{{if .x}}{{range .a}}{{with .y}}{{range .b}}{{range .c}}{{range .d}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}`: "nested more than 3",
		`{{.x`:             "unclosed action",
		`{{nosuchfunc 1}}`: "not defined",
	}
	for src, want := range cases {
		_, err := Parse([]byte(src))
		var te *TemplateError
		if !errors.As(err, &te) || !strings.Contains(te.Msg, want) {
			t.Errorf("Parse(%q) = %v, want error containing %q", src, err, want)
		}
	}
	// Three levels are fine, and an else branch does not count as a level.
	ok := `{{range .a}}{{range .b}}{{range .c}}x{{else}}{{range .d}}{{end}}{{end}}{{end}}{{end}}`
	if _, err := Parse([]byte(ok)); err != nil {
		t.Errorf("Parse(%q) = %v", ok, err)
	}
	// A field of a missing object is an error, not a silent blank.
	if _, err := render(t, `{{.missing.deeper}}`, nil); err == nil {
		t.Error("field of a missing object rendered")
	}
	// Numbers from data cannot drive a loop.
	if _, err := render(t, `{{range .n}}x{{end}}`, map[string]any{"n": 1e12}); err == nil {
		t.Error("range over a data number was allowed")
	}
}

func TestLimits(t *testing.T) {
	ctx := context.Background()
	big := make([]any, 1000)
	for i := range big {
		big[i] = strings.Repeat("x", 100)
	}
	_, err := Execute(ctx, []byte(`{{range .a}}{{.}}{{end}}`), data_(t, map[string]any{"a": big}), Limits{MaxBytes: 10_000})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over the output limit = %v", err)
	}

	// Nested loops that print nothing still spend the step budget.
	many := make([]any, 2000)
	for i := range many {
		many[i] = ""
	}
	nested := []byte(`{{range $.a}}{{range $.a}}{{range $.a}}{{end}}{{end}}{{end}}`)
	start := time.Now()
	_, err = Execute(ctx, nested, data_(t, map[string]any{"a": many}), Limits{MaxBytes: 1 << 20, MaxSteps: 100_000})
	if !errors.Is(err, ErrTooManySteps) {
		t.Fatalf("nested loops = %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("budget stop took %v", d)
	}
	// len, index and join walk lists and spend steps too.
	walk := []byte(`{{range $.a}}{{len $.a}}{{end}}`)
	if _, err := Execute(ctx, walk, data_(t, map[string]any{"a": many}), Limits{MaxBytes: 1 << 30, MaxSteps: 100_000}); !errors.Is(err, ErrTooManySteps) {
		t.Fatalf("len in a loop = %v", err)
	}

	// The deadline stops a render between steps.
	tctx, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	start = time.Now()
	_, err = Execute(tctx, nested, data_(t, map[string]any{"a": many}), Limits{MaxBytes: 1 << 20, MaxSteps: 1 << 40})
	if !errors.Is(err, ErrRenderTimeout) || time.Since(start) > 2*time.Second {
		t.Fatalf("deadline = %v after %v", err, time.Since(start))
	}

	wide := map[string]any{}
	for i := range maxObjectKeys + 1 {
		wide[strconv.Itoa(i)] = 1
	}
	if _, err := Execute(ctx, []byte(`x`), data_(t, wide), Limits{MaxBytes: 1 << 20}); err == nil {
		t.Fatal("object with too many keys accepted")
	}
}

func TestEscapeErrorIsTemplateError(t *testing.T) {
	_, err := Execute(context.Background(), []byte(`<a href="{{.x}}`), nil, Limits{MaxBytes: 1 << 20})
	var te *TemplateError
	if !errors.As(err, &te) || !te.Escape {
		t.Fatalf("Execute = %#v", err)
	}
}
