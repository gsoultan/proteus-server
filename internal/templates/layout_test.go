// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

func textBlock(id, text string) *proteusv1.Block {
	return &proteusv1.Block{Id: id, Kind: &proteusv1.Block_Text{Text: &proteusv1.Text{Text: text}}}
}

func TestCompileLayout(t *testing.T) {
	l := &proteusv1.Layout{
		Theme:  &proteusv1.Theme{Locale: "id-ID", Currency: "IDR", AccentColor: "#ff0000", FontFamily: "Georgia, serif"},
		Header: &proteusv1.Region{HeightMm: 30, Blocks: []*proteusv1.Block{textBlock("h", "Header {{.title}}")}},
		Footer: &proteusv1.Region{Blocks: []*proteusv1.Block{textBlock("f", "Footer")}},
		Body: &proteusv1.Region{Blocks: []*proteusv1.Block{
			{Id: "title", Kind: &proteusv1.Block_Heading{Heading: &proteusv1.Heading{Text: "{{.title}}", Level: 1}},
				Style: &proteusv1.BlockStyle{Align: proteusv1.Align_ALIGN_CENTER, Bold: true}},
			{Id: "total", Kind: &proteusv1.Block_Field{Field: &proteusv1.Field{Path: "total", Label: "Total", Format: proteusv1.FieldFormat_FIELD_FORMAT_MONEY}}},
			{Id: "paid", Kind: &proteusv1.Block_Text{Text: &proteusv1.Text{Text: "Paid, thank you"}},
				ShowIf: &proteusv1.Condition{Path: "status", Op: proteusv1.ConditionOp_CONDITION_OP_EQUALS, Value: "paid"}},
			{Id: "items", Kind: &proteusv1.Block_Table{Table: &proteusv1.Table{List: "items", Numbered: true, Columns: []*proteusv1.Column{
				{Header: "Item", Path: "name"},
				{Header: "Qty", Path: "qty", Format: proteusv1.FieldFormat_FIELD_FORMAT_INTEGER},
				{Header: "Amount", Path: "amount", Format: proteusv1.FieldFormat_FIELD_FORMAT_MONEY},
			}}}},
			{Id: "sum", Kind: &proteusv1.Block_Totals{Totals: &proteusv1.Totals{Rows: []*proteusv1.TotalRow{
				{Label: "Subtotal", Value: &proteusv1.TotalRow_Aggregate{Aggregate: &proteusv1.Aggregate{Func: proteusv1.AggregateFunc_AGGREGATE_FUNC_SUM, List: "items", Column: "amount"}}},
				{Label: "Total", Value: &proteusv1.TotalRow_Path{Path: "total"}, Emphasized: true},
			}}}},
			{Id: "when", Kind: &proteusv1.Block_Field{Field: &proteusv1.Field{Path: "date", Format: proteusv1.FieldFormat_FIELD_FORMAT_DATE}}},
			{Id: "pct", Kind: &proteusv1.Block_Field{Field: &proteusv1.Field{Path: "rate", Format: proteusv1.FieldFormat_FIELD_FORMAT_PERCENT}}},
			{Id: "brk", Kind: &proteusv1.Block_PageBreak{PageBreak: &proteusv1.PageBreak{}}},
			{Id: "secs", Kind: &proteusv1.Block_Section{Section: &proteusv1.Section{Repeat: "sections", Content: &proteusv1.Region{Blocks: []*proteusv1.Block{
				{Id: "sh", Kind: &proteusv1.Block_Heading{Heading: &proteusv1.Heading{Text: "{{.heading}}", Level: 2}}},
			}}}}},
		}},
	}
	page := &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_LETTER, Landscape: true, MarginMm: new(float64)}
	*page.MarginMm = 12
	out, err := Compile(l, page)
	if err != nil {
		t.Fatal(err)
	}
	src := string(out)
	for _, want := range []string{
		"size: letter landscape", "margin: 42mm 12mm 27mm 12mm", // 12 + 30 header, 12 + 15 default footer
		".pt-header { position: fixed; top: -30mm", ".pt-footer { position: fixed; bottom: -15mm",
		"--pt-accent: #ff0000", "Georgia, serif",
		`<header class="pt-header">`, `<footer class="pt-footer">`,
		`{{if eq (printf "%v" $.status) "paid"}}`, "{{range $i, $it := $.items}}", "{{money .amount}}",
		`{{money (sum $.items "amount")}}`, "{{fdate $.date}}", "{{percent $.rate}}", `<div class="pt-break">`, "{{range $i, $it := $.sections}}",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("compiled HTML lacks %q\n%s", want, src)
		}
	}

	data := data_(t, map[string]any{
		"title": "Tagihan", "total": 1500000, "status": "paid", "date": "2026-11-02", "rate": 0.11,
		"items":    []any{map[string]any{"name": "A", "qty": 2, "amount": 1000000}, map[string]any{"name": "B", "qty": 1, "amount": 500000}},
		"sections": []any{map[string]any{"heading": "One"}, map[string]any{"heading": "Two"}},
	})
	html, err := ExecuteWith(context.Background(), out, data, Limits{MaxBytes: 1 << 20}, FormatOf(l))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Header Tagihan", "<h1", "Tagihan", "Rp 1.500.000", "Paid, thank you", "Rp 1.000.000", "11%", "2 Nov 2026", "<h2", "One", "Two", ">2<"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("rendered HTML lacks %q\n%s", want, html)
		}
	}
	if strings.Contains(string(html), "{{") {
		t.Fatalf("unrendered action in\n%s", html)
	}

	// The condition hides the block when the data differs.
	data.Fields["status"] = structpb.NewStringValue("sent")
	html, _ = ExecuteWith(context.Background(), out, data, Limits{MaxBytes: 1 << 20}, FormatOf(l))
	if strings.Contains(string(html), "Paid, thank you") {
		t.Fatal("condition did not hide the block")
	}
}

func TestCompileLayoutErrors(t *testing.T) {
	cases := map[string]struct {
		layout *proteusv1.Layout
		block  string
		msg    string
	}{
		"bad path": {
			layout: &proteusv1.Layout{Body: &proteusv1.Region{Blocks: []*proteusv1.Block{
				{Id: "f", Kind: &proteusv1.Block_Field{Field: &proteusv1.Field{Path: "a..b"}}}}}},
			block: "f", msg: "path",
		},
		"template action in text": {
			layout: &proteusv1.Layout{Body: &proteusv1.Region{Blocks: []*proteusv1.Block{textBlock("t", "{{template \"x\"}}")}}},
			block:  "t", msg: "{{template",
		},
		"bad colour": {
			layout: &proteusv1.Layout{Theme: &proteusv1.Theme{AccentColor: "url(javascript:1)"}, Body: &proteusv1.Region{Blocks: []*proteusv1.Block{textBlock("t", "x")}}},
			msg:    "accent_color",
		},
		"unknown locale": {
			layout: &proteusv1.Layout{Theme: &proteusv1.Theme{Locale: "xx-XX"}, Body: &proteusv1.Region{Blocks: []*proteusv1.Block{textBlock("t", "x")}}},
			msg:    "locale",
		},
		"empty block": {
			layout: &proteusv1.Layout{Body: &proteusv1.Region{Blocks: []*proteusv1.Block{{Id: "e"}}}},
			block:  "e", msg: "kind",
		},
		"too deep": {
			layout: func() *proteusv1.Layout {
				inner := textBlock("t", "x")
				for i := range maxLayoutDepth + 1 {
					inner = &proteusv1.Block{Id: fmt.Sprintf("c%d", i), Kind: &proteusv1.Block_Columns{Columns: &proteusv1.Columns{Columns: []*proteusv1.Region{{Blocks: []*proteusv1.Block{inner}}}}}}
				}
				return &proteusv1.Layout{Body: &proteusv1.Region{Blocks: []*proteusv1.Block{inner}}}
			}(),
			msg: "deep",
		},
		"too many blocks": {
			layout: func() *proteusv1.Layout {
				var blocks []*proteusv1.Block
				for i := range maxBlocks + 1 {
					blocks = append(blocks, textBlock(fmt.Sprintf("b%d", i), "x"))
				}
				return &proteusv1.Layout{Body: &proteusv1.Region{Blocks: blocks}}
			}(),
			msg: "blocks",
		},
		"tall header": {
			layout: &proteusv1.Layout{Header: &proteusv1.Region{HeightMm: 500, Blocks: []*proteusv1.Block{textBlock("t", "x")}}},
			msg:    "height_mm",
		},
		"image traversal": {
			layout: &proteusv1.Layout{Body: &proteusv1.Region{Blocks: []*proteusv1.Block{
				{Id: "i", Kind: &proteusv1.Block_Image{Image: &proteusv1.Image{Source: &proteusv1.Image_Asset{Asset: "../etc/passwd"}}}}}}},
			block: "i", msg: "asset",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Compile(c.layout, nil)
			var le *LayoutError
			if !errors.As(err, &le) {
				t.Fatalf("err = %v, want LayoutError", err)
			}
			if le.BlockID != c.block || !strings.Contains(le.Msg, c.msg) {
				t.Fatalf("err = %v, want block %q with %q", err, c.block, c.msg)
			}
		})
	}
}

func TestValidateFields(t *testing.T) {
	good := []*proteusv1.FieldSpec{
		{Path: "number", Required: true},
		{Path: "client", Type: proteusv1.FieldType_FIELD_TYPE_OBJECT, ItemFields: []*proteusv1.FieldSpec{{Path: "name", Required: true}}},
		{Path: "items", Type: proteusv1.FieldType_FIELD_TYPE_LIST, ItemFields: []*proteusv1.FieldSpec{
			{Path: "qty", Type: proteusv1.FieldType_FIELD_TYPE_NUMBER, Required: true},
			{Path: "when", Type: proteusv1.FieldType_FIELD_TYPE_DATE},
		}},
		{Path: "status", Type: proteusv1.FieldType_FIELD_TYPE_CHOICE, Choices: []string{"draft", "paid"}, DefaultValue: structpb.NewStringValue("draft")},
		{Path: "paid", Type: proteusv1.FieldType_FIELD_TYPE_BOOL},
	}
	if err := ValidateFields(good); err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]*proteusv1.FieldSpec{
		"bad path":        {{Path: "9x"}},
		"duplicate":       {{Path: "a"}, {Path: "a"}},
		"choice without":  {{Path: "s", Type: proteusv1.FieldType_FIELD_TYPE_CHOICE}},
		"items on text":   {{Path: "s", ItemFields: []*proteusv1.FieldSpec{{Path: "x"}}}},
		"nested dotted":   {{Path: "o", Type: proteusv1.FieldType_FIELD_TYPE_OBJECT, ItemFields: []*proteusv1.FieldSpec{{Path: "a.b"}}}},
		"default wrong":   {{Path: "n", Type: proteusv1.FieldType_FIELD_TYPE_NUMBER, DefaultValue: structpb.NewStringValue("x")}},
		"default unknown": {{Path: "s", Type: proteusv1.FieldType_FIELD_TYPE_CHOICE, Choices: []string{"a"}, DefaultValue: structpb.NewStringValue("b")}},
	} {
		if err := ValidateFields(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	data := data_(t, map[string]any{
		"number": " ", "client": map[string]any{}, "paid": "yes",
		"items":  []any{map[string]any{"qty": "x", "when": "yesterday"}, "notanobject", map[string]any{"qty": 2}},
		"status": "sent",
	})
	err := CheckData(good, data)
	var de *DataError
	if !errors.As(err, &de) {
		t.Fatalf("err = %v", err)
	}
	got := map[string]string{}
	for _, p := range de.Problems {
		got[p.GetPath()] = p.GetMessage()
	}
	for path, want := range map[string]string{
		"number": "required", "client.name": "required", "paid": "true or false",
		"items[0].qty": "number", "items[0].when": "date", "items[1]": "object", "status": "one of",
	} {
		if !strings.Contains(got[path], want) {
			t.Errorf("%s: %q, want %q", path, got[path], want)
		}
	}
	if len(got) != 7 {
		t.Errorf("problems = %v", got)
	}
	if de.Detail().GetProblems()[0].GetPath() == "" {
		t.Fatal("detail has no problems")
	}

	ok := data_(t, map[string]any{"number": "1", "client": map[string]any{"name": "x"}, "items": []any{map[string]any{"qty": 1}}})
	if err := CheckData(good, ok); err != nil {
		t.Fatal(err)
	}
	// Defaults fill missing values without touching the input.
	withDefaults := ApplyDefaults(good, ok)
	if withDefaults.Fields["status"].GetStringValue() != "draft" || ok.Fields["status"] != nil {
		t.Fatalf("defaults = %v", withDefaults)
	}
	if CheckData(good, nil) == nil {
		t.Fatal("nil data passed with required fields")
	}
}

func TestFormat(t *testing.T) {
	id := Format{Locale: "id-ID", Currency: "IDR"}
	us := Format{}
	de := Format{Locale: "de-DE", Currency: "EUR"}
	cases := []struct{ got, want string }{
		{id.Money(1234567.891, ""), "Rp 1.234.568"},
		{us.Money(1234567.891, ""), "$1,234,567.89"},
		{de.Money(1234.5, ""), "1.234,50 €"},
		{us.Money(10, "JPY"), "¥10"},
		{us.Money(-5.5, "GBP"), "-£5.50"},
		{id.Number(1234.5, 1), "1.234,5"},
		{us.Number(1234.5, -1), "1,234.5"},
		{us.Percent(0.115, 1), "11.5%"},
		{id.Percent(0.11, 0), "11%"},
		{us.Date("2026-11-02", ""), "Nov 2, 2026"},
		{id.Date("2026-11-02T10:00:00Z", ""), "2 Nov 2026"},
		{us.Date("2026-11-02", "2006/01/02"), "2026/11/02"},
		{us.Date("not a date", ""), "not a date"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("%d: %q, want %q", i, c.got, c.want)
		}
	}
	if !KnownLocale("en-US") || KnownLocale("xx") || !KnownCurrency("IDR") || KnownCurrency("XXX") {
		t.Fatal("known tables")
	}

	out, err := ExecuteWith(context.Background(), []byte(`{{money .a}}|{{money .a "USD"}}|{{fnumber .a 1}}|{{percent .r}}|{{fdate .d}}|{{sum .items "x"}}|{{count .items}}|{{avg .items "x"}}|{{add .a 1}}|{{mul .a 2 | money}}|{{div .a 2}}|{{round .a 0}}|{{nl2br .t}}|{{qr "hi"}}`),
		data_(t, map[string]any{"a": 1500.75, "r": 0.25, "d": "2026-01-31", "t": "a\nb", "items": []any{map[string]any{"x": 1}, map[string]any{"x": 2}}}),
		Limits{MaxBytes: 1 << 20}, id)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"Rp 1.501|", "|$1.500,75|", "|1.500,8|", "|25%|", "|31 Jan 2026|", "|3|", "|2|", "|1.5|", "|1501.75|", "|Rp 3.002|", "|750.375|", "|1501|", "|a<br>b|", "|data:image/png;base64,"} {
		if !strings.Contains(s, want) {
			t.Errorf("output lacks %q: %s", want, s)
		}
	}
}
