// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package convert

import (
	"errors"
	"strings"
	"testing"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

func TestFromProto(t *testing.T) {
	doc := &proteusv1.Document{
		Format: proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN,
		Source: &proteusv1.Document_Content{Content: []byte("# x")},
		Assets: []*proteusv1.Asset{{Path: "./img/a.png", Data: []byte("a")}},
	}
	req, err := FromProto(doc, 0, nil, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if req.Format != Markdown || req.Output != PDF || string(req.Assets["img/a.png"]) != "a" || req.Options == nil {
		t.Fatalf("request %+v", req)
	}
	if len(engineOptions(nil, nil)) != 0 || len(engineOptions(req.Options, nil)) != 0 {
		t.Fatal("default options should add nothing")
	}

	url, err := FromProto(&proteusv1.Document{Source: &proteusv1.Document_Url{Url: " https://example.com "}}, proteusv1.OutputFormat_OUTPUT_FORMAT_PDF, nil, Limits{})
	if err != nil || url.Format != "html" || url.URL != "https://example.com" {
		t.Fatalf("url request %+v, %v", url, err)
	}

	m := 10.0
	opts := &proteusv1.Options{Page: &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_A3, Landscape: true, MarginMm: &m},
		MediaType: proteusv1.MediaType_MEDIA_TYPE_SCREEN, Pdf: &proteusv1.PdfOptions{Tagged: true}, Metadata: &proteusv1.Metadata{Title: "t"}}
	req, err = FromProto(doc, 0, opts, Limits{})
	if err != nil || len(engineOptions(req.Options, nil)) != 5 {
		t.Fatalf("options: %v, %d engine options", err, len(engineOptions(req.Options, nil)))
	}
}

func TestFromProtoRejects(t *testing.T) {
	big := 101.0
	content := &proteusv1.Document_Content{Content: []byte("x")}
	cases := map[string]struct {
		doc  *proteusv1.Document
		out  proteusv1.OutputFormat
		opts *proteusv1.Options
		lim  Limits
	}{
		"bad output":      {doc: &proteusv1.Document{Format: 1, Source: content}, out: 99},
		"nil document":    {doc: nil},
		"no source":       {doc: &proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_HTML}},
		"no format":       {doc: &proteusv1.Document{Source: content}},
		"bad format":      {doc: &proteusv1.Document{Format: 99, Source: content}},
		"traversal":       {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a/../../b"}}}},
		"duplicate asset": {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a"}, {Path: "./a"}}}},
		"too many assets": {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a"}, {Path: "b"}}}, lim: Limits{MaxAssets: 1}},
		"margin":          {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{Page: &proteusv1.PageOptions{MarginMm: &big}}},
		"margins":         {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{Page: &proteusv1.PageOptions{Margins: &proteusv1.Margins{Top: -1}}}},
		"page size":       {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{Page: &proteusv1.PageOptions{Size: 42}}},
		"media type":      {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{MediaType: 9}},
	}
	for name, c := range cases {
		var bre *BadRequestError
		if _, err := FromProto(c.doc, c.out, c.opts, c.lim); !errors.As(err, &bre) {
			t.Errorf("%s: %v, want BadRequestError", name, err)
		}
	}
}

func TestFileName(t *testing.T) {
	for title, want := range map[string]string{"Q3 report": "Q3-report.pdf", "": "document.pdf", "a/b": "ab.pdf"} {
		if got := FileName(title, PDF); got != want {
			t.Errorf("FileName(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestWatermarkOptions(t *testing.T) {
	doc := &proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Source: &proteusv1.Document_Content{Content: []byte("<p>x</p>")},
		Assets: []*proteusv1.Asset{{Path: "logo.png", Data: []byte("png")}}}
	ok := &proteusv1.Options{Page: &proteusv1.PageOptions{Watermark: &proteusv1.Watermark{
		Mark: &proteusv1.Watermark_Text{Text: "DRAFT"}, Opacity: 0.2,
		Placement: proteusv1.WatermarkPlacement_WATERMARK_PLACEMENT_TILED,
		Layer:     proteusv1.WatermarkLayer_WATERMARK_LAYER_OVER,
		Pages:     proteusv1.WatermarkPages_WATERMARK_PAGES_FIRST,
	}}}
	if _, err := FromProto(doc, proteusv1.OutputFormat_OUTPUT_FORMAT_PDF, ok, Limits{}); err != nil {
		t.Fatalf("text watermark refused: %v", err)
	}
	if n := len(engineOptions(ok, nil)); n != 1 {
		t.Fatalf("engine options = %d, want 1 (the watermark)", n)
	}
	img := &proteusv1.Options{Page: &proteusv1.PageOptions{Watermark: &proteusv1.Watermark{
		Mark: &proteusv1.Watermark_ImageAsset{ImageAsset: "logo.png"}}}}
	if _, err := FromProto(doc, proteusv1.OutputFormat_OUTPUT_FORMAT_PDF, img, Limits{}); err != nil {
		t.Fatalf("image watermark refused: %v", err)
	}
	assets := func(p string) ([]byte, bool) { return []byte("png"), p == "logo.png" }
	if n := len(engineOptions(img, assets)); n != 1 {
		t.Fatalf("image watermark engine options = %d, want 1", n)
	}
	if n := len(engineOptions(img, nil)); n != 0 {
		t.Fatalf("image watermark without assets = %d options, want 0", n)
	}
	for name, w := range map[string]*proteusv1.Watermark{
		"missing asset": {Mark: &proteusv1.Watermark_ImageAsset{ImageAsset: "other.png"}},
		"opacity":       {Mark: &proteusv1.Watermark_Text{Text: "x"}, Opacity: 1.5},
		"angle":         {Mark: &proteusv1.Watermark_Text{Text: "x"}, AngleDeg: new(float64(400))},
		"long text":     {Mark: &proteusv1.Watermark_Text{Text: strings.Repeat("x", 201)}},
		"placement":     {Mark: &proteusv1.Watermark_Text{Text: "x"}, Placement: 9},
	} {
		o := &proteusv1.Options{Page: &proteusv1.PageOptions{Watermark: w}}
		if _, err := FromProto(doc, proteusv1.OutputFormat_OUTPUT_FORMAT_PDF, o, Limits{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// Empty watermark: nothing to stamp, no option.
	empty := &proteusv1.Options{Page: &proteusv1.PageOptions{Watermark: &proteusv1.Watermark{}}}
	if n := len(engineOptions(empty, nil)); n != 0 {
		t.Fatalf("empty watermark = %d options", n)
	}
}
