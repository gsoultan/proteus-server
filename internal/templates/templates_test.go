package templates

import (
	"testing"

	"google.golang.org/protobuf/proto"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

func TestMergeOptions(t *testing.T) {
	defaults := &proteusv1.Options{
		Stylesheet: "body{}",
		Metadata:   &proteusv1.Metadata{Author: "Template", Title: "T"},
		Page:       &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_A4, Landscape: true},
		Pdf:        &proteusv1.PdfOptions{Tagged: true},
	}
	req := &proteusv1.Options{
		Metadata: &proteusv1.Metadata{Title: "Request"},
		Page:     &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_LETTER},
	}
	got := mergeOptions(defaults, proteusv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED, req)
	want := &proteusv1.Options{
		Stylesheet: "body{}",
		Metadata:   &proteusv1.Metadata{Author: "Template", Title: "Request"},
		Page:       &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_LETTER, Landscape: true},
		Pdf:        &proteusv1.PdfOptions{Tagged: true},
	}
	if !proto.Equal(got, want) {
		t.Fatalf("merged %v\nwant %v", got, want)
	}
	if defaults.GetMetadata().GetTitle() != "T" {
		t.Fatal("defaults were modified")
	}
	if got := mergeOptions(nil, proteusv1.OutputFormat_OUTPUT_FORMAT_PDF, nil); got == nil || proto.Size(got) != 0 {
		t.Fatalf("empty merge = %v", got)
	}
}
