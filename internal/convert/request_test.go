package convert

import (
	"errors"
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
	if len(engineOptions(nil)) != 0 || len(engineOptions(req.Options)) != 0 {
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
	if err != nil || len(engineOptions(req.Options)) != 5 {
		t.Fatalf("options: %v, %d engine options", err, len(engineOptions(req.Options)))
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
		"pdf opts xlsx":   {doc: &proteusv1.Document{Format: 1, Source: content}, out: proteusv1.OutputFormat_OUTPUT_FORMAT_XLSX, opts: &proteusv1.Options{Pdf: &proteusv1.PdfOptions{Tagged: true}}},
		"page opts xlsx":  {doc: &proteusv1.Document{Format: 1, Source: content}, out: proteusv1.OutputFormat_OUTPUT_FORMAT_XLSX, opts: &proteusv1.Options{Page: &proteusv1.PageOptions{Landscape: true}}},
		"sheet opts pdf":  {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{Spreadsheet: &proteusv1.SpreadsheetOptions{}}},
	}
	for name, c := range cases {
		var bre *BadRequestError
		if _, err := FromProto(c.doc, c.out, c.opts, c.lim); !errors.As(err, &bre) {
			t.Errorf("%s: %v, want BadRequestError", name, err)
		}
	}
}

func TestFromProtoXLSX(t *testing.T) {
	req, err := FromProto(&proteusv1.Document{Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Source: &proteusv1.Document_Content{Content: []byte("x")}},
		proteusv1.OutputFormat_OUTPUT_FORMAT_XLSX, &proteusv1.Options{Spreadsheet: &proteusv1.SpreadsheetOptions{}}, Limits{})
	if err != nil || req.Output != XLSX || req.Route() != (Route{HTML, XLSX}) {
		t.Fatalf("xlsx request: %v %+v", err, req)
	}
	if got := FileName("Sales 2026", XLSX); got != "Sales-2026.xlsx" {
		t.Errorf("FileName = %q", got)
	}
	tg, ok := TargetOf(XLSX)
	if !ok || tg.Paged || tg.MIMEType != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || tg.Check([]byte("PK\x03\x04...")) != nil || tg.Check([]byte("%PDF-")) == nil {
		t.Errorf("target = %+v", tg)
	}
}

func TestFileName(t *testing.T) {
	for title, want := range map[string]string{"Q3 report": "Q3-report.pdf", "": "document.pdf", "a/b": "ab.pdf"} {
		if got := FileName(title, PDF); got != want {
			t.Errorf("FileName(%q) = %q, want %q", title, got, want)
		}
	}
}
