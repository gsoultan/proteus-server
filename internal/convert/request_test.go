package convert

import (
	"errors"
	"testing"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

func TestFromProto(t *testing.T) {
	doc := &proteusv1.Document{
		Format: proteusv1.Format_FORMAT_MARKDOWN,
		Source: &proteusv1.Document_Content{Content: []byte("# x")},
		Assets: []*proteusv1.Asset{{Path: "./img/a.png", Data: []byte("a")}},
	}
	req, err := FromProto(doc, nil, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if req.Format != "markdown" || string(req.Assets["img/a.png"]) != "a" || req.Options == nil {
		t.Fatalf("request %+v", req)
	}
	if len(engineOptions(nil)) != 0 || len(engineOptions(req.Options)) != 0 {
		t.Fatal("default options should add nothing")
	}

	url, err := FromProto(&proteusv1.Document{Source: &proteusv1.Document_Url{Url: " https://example.com "}}, nil, Limits{})
	if err != nil || url.Format != "html" || url.URL != "https://example.com" {
		t.Fatalf("url request %+v, %v", url, err)
	}

	m := 10.0
	opts := &proteusv1.Options{PageSize: proteusv1.PageSize_PAGE_SIZE_A3, Landscape: true, MarginMm: &m,
		MediaType: proteusv1.MediaType_MEDIA_TYPE_SCREEN, Tagged: true, Metadata: &proteusv1.Metadata{Title: "t"}}
	req, err = FromProto(doc, opts, Limits{})
	if err != nil || len(engineOptions(req.Options)) != 5 {
		t.Fatalf("options: %v, %d engine options", err, len(engineOptions(req.Options)))
	}
}

func TestFromProtoRejects(t *testing.T) {
	big := 101.0
	content := &proteusv1.Document_Content{Content: []byte("x")}
	cases := map[string]struct {
		doc  *proteusv1.Document
		opts *proteusv1.Options
		lim  Limits
	}{
		"nil document":    {doc: nil},
		"no source":       {doc: &proteusv1.Document{Format: proteusv1.Format_FORMAT_HTML}},
		"no format":       {doc: &proteusv1.Document{Source: content}},
		"bad format":      {doc: &proteusv1.Document{Format: 99, Source: content}},
		"traversal":       {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a/../../b"}}}},
		"duplicate asset": {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a"}, {Path: "./a"}}}},
		"too many assets": {doc: &proteusv1.Document{Format: 1, Source: content, Assets: []*proteusv1.Asset{{Path: "a"}, {Path: "b"}}}, lim: Limits{MaxAssets: 1}},
		"margin":          {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{MarginMm: &big}},
		"margins":         {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{Margins: &proteusv1.Margins{Top: -1}}},
		"page size":       {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{PageSize: 42}},
		"media type":      {doc: &proteusv1.Document{Format: 1, Source: content}, opts: &proteusv1.Options{MediaType: 9}},
	}
	for name, c := range cases {
		var bre *BadRequestError
		if _, err := FromProto(c.doc, c.opts, c.lim); !errors.As(err, &bre) {
			t.Errorf("%s: %v, want BadRequestError", name, err)
		}
	}
}
