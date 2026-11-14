package rpc

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/gsoultan/gpdf/doc"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
)

func (e *env) pdfClient(key string) proteusv1connect.PdfServiceClient {
	return proteusv1connect.NewPdfServiceClient(connect.NewClient(
		connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC()), withKey(key)))
}

// renderPDF converts HTML with one "<h1>title</h1>" per page into a PDF.
func renderPDF(t *testing.T, e *env, titles ...string) []byte {
	t.Helper()
	var src strings.Builder
	for i, title := range titles {
		style := ""
		if i > 0 {
			style = ` style="page-break-before: always"`
		}
		src.WriteString("<h1" + style + ">" + title + "</h1>")
	}
	res, err := e.grpcClient(keyA).Convert(context.Background(), &proteusv1.ConvertRequest{Document: html(src.String())})
	if err != nil {
		t.Fatal(err)
	}
	return res.GetContent()
}

// pdfPages returns the trimmed text of every page of pdf.
func pdfPages(t *testing.T, pdf []byte, password string) []string {
	t.Helper()
	d, err := doc.OpenReaderWithPassword(bytes.NewReader(pdf), int64(len(pdf)), password)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	texts, err := d.ReadContentPerPage()
	if err != nil {
		t.Fatal(err)
	}
	for i := range texts {
		texts[i] = strings.Join(strings.Fields(texts[i]), " ")
	}
	return texts
}

func content(b []byte) *proteusv1.PdfInput {
	return &proteusv1.PdfInput{Source: &proteusv1.PdfInput_Content{Content: b}}
}

func TestMergeAndSplitPdfs(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	a := renderPDF(t, e, "Alpha one", "Alpha two", "Alpha three")
	if got := pdfPages(t, a, ""); len(got) != 3 {
		t.Fatalf("source has %d pages (%q), want 3", len(got), got)
	}
	b := renderPDF(t, e, "Bravo one")
	c := e.pdfClient(keyA)

	in := content(a)
	in.Pages = "3,1"
	merged, err := c.MergePdfs(ctx, &proteusv1.MergePdfsRequest{
		Inputs: []*proteusv1.PdfInput{in, content(b)}, FileName: "../../etc/Report Q3.pdf",
	})
	if err != nil {
		t.Fatal(err)
	}
	f := merged.GetFile()
	if f.GetFileName() != "Report Q3.pdf" || f.GetMimeType() != "application/pdf" || f.GetPageCount() != 3 || f.GetPasswordProtected() {
		t.Fatalf("merged file = %q %q %d pages protected=%v", f.GetFileName(), f.GetMimeType(), f.GetPageCount(), f.GetPasswordProtected())
	}
	got := pdfPages(t, f.GetContent(), "")
	want := []string{"Alpha three", "Alpha one", "Bravo one"}
	for i := range want {
		if i >= len(got) || !strings.Contains(got[i], want[i]) {
			t.Fatalf("merged pages = %q, want %q", got, want)
		}
	}

	split, err := c.SplitPdf(ctx, &proteusv1.SplitPdfRequest{
		Input: content(f.GetContent()), Mode: &proteusv1.SplitPdfRequest_Every{Every: 2}, FileName: "out",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(split.GetParts()) != 2 || split.GetParts()[0].GetFileName() != "out-1.pdf" ||
		split.GetParts()[0].GetPageCount() != 2 || split.GetParts()[1].GetPageCount() != 1 {
		t.Fatalf("split parts = %v", split.GetParts())
	}
	if p := pdfPages(t, split.GetParts()[1].GetContent(), ""); !strings.Contains(p[0], "Bravo one") {
		t.Fatalf("second part = %q", p)
	}

	byRange, err := c.SplitPdf(ctx, &proteusv1.SplitPdfRequest{
		Input: content(a),
		Mode:  &proteusv1.SplitPdfRequest_Ranges{Ranges: &proteusv1.PageRanges{Ranges: []string{"2-", "1"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(byRange.GetParts()) != 2 || byRange.GetParts()[0].GetPageCount() != 2 || byRange.GetParts()[1].GetFileName() != "part-2.pdf" {
		t.Fatalf("range parts = %v", byRange.GetParts())
	}
}

func TestPdfPasswords(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	c := e.pdfClient(keyA)
	plain := renderPDF(t, e, "Confidential")

	gen, err := c.MergePdfs(ctx, &proteusv1.MergePdfsRequest{
		Inputs: []*proteusv1.PdfInput{content(plain)}, Protection: &proteusv1.Protection{Generate: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	pw := gen.GetPassword()
	if len(pw) != 20 || !gen.GetFile().GetPasswordProtected() {
		t.Fatalf("generated password %q, protected=%v", pw, gen.GetFile().GetPasswordProtected())
	}
	locked := gen.GetFile().GetContent()
	if p := pdfPages(t, locked, pw); !strings.Contains(p[0], "Confidential") {
		t.Fatalf("locked page = %q", p)
	}

	// A protected input needs its password.
	_, err = c.SplitPdf(ctx, &proteusv1.SplitPdfRequest{Input: content(locked), Mode: &proteusv1.SplitPdfRequest_Every{Every: 1}})
	expectCode(t, err, connect.CodeInvalidArgument)
	if !strings.Contains(err.Error(), "password protected") {
		t.Fatalf("missing password error = %v", err)
	}
	wrong := content(locked)
	wrong.Password = "nope"
	_, err = c.SplitPdf(ctx, &proteusv1.SplitPdfRequest{Input: wrong, Mode: &proteusv1.SplitPdfRequest_Every{Every: 1}})
	expectCode(t, err, connect.CodeInvalidArgument)
	if strings.Contains(err.Error(), "nope") || strings.Contains(err.Error(), pw) {
		t.Fatalf("error echoes a password: %v", err)
	}

	right := content(locked)
	right.Password = pw
	split, err := c.SplitPdf(ctx, &proteusv1.SplitPdfRequest{
		Input: right, Mode: &proteusv1.SplitPdfRequest_Every{Every: 1},
		Protection: &proteusv1.Protection{Password: "new-pass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if split.GetPassword() != "" || !split.GetParts()[0].GetPasswordProtected() {
		t.Fatalf("caller password: response password %q, protected %v", split.GetPassword(), split.GetParts()[0].GetPasswordProtected())
	}
	if p := pdfPages(t, split.GetParts()[0].GetContent(), "new-pass"); !strings.Contains(p[0], "Confidential") {
		t.Fatalf("re-protected page = %q", p)
	}

	// Unprotected output from a protected input.
	unlocked, err := c.MergePdfs(ctx, &proteusv1.MergePdfsRequest{Inputs: []*proteusv1.PdfInput{right}})
	if err != nil {
		t.Fatal(err)
	}
	if p := pdfPages(t, unlocked.GetFile().GetContent(), ""); !strings.Contains(p[0], "Confidential") {
		t.Fatalf("unlocked page = %q", p)
	}

	_, err = c.MergePdfs(ctx, &proteusv1.MergePdfsRequest{
		Inputs: []*proteusv1.PdfInput{content(plain)}, Protection: &proteusv1.Protection{Generate: true, Password: "x"},
	})
	expectCode(t, err, connect.CodeInvalidArgument)
	_, err = c.MergePdfs(ctx, &proteusv1.MergePdfsRequest{
		Inputs: []*proteusv1.PdfInput{content(plain)}, Protection: &proteusv1.Protection{},
	})
	expectCode(t, err, connect.CodeInvalidArgument)
}

func TestPdfValidation(t *testing.T) {
	e := newEnv(t, envOpts{})
	ctx := context.Background()
	c := e.pdfClient(keyA)
	pdf := renderPDF(t, e, "Only page")

	_, err := e.pdfClient("").MergePdfs(ctx, &proteusv1.MergePdfsRequest{Inputs: []*proteusv1.PdfInput{content(pdf)}})
	expectCode(t, err, connect.CodeUnauthenticated)

	cases := map[string]*proteusv1.MergePdfsRequest{
		"no inputs":  {},
		"empty":      {Inputs: []*proteusv1.PdfInput{content(nil)}},
		"not a pdf":  {Inputs: []*proteusv1.PdfInput{content([]byte("hello"))}},
		"bad range":  {Inputs: []*proteusv1.PdfInput{{Source: &proteusv1.PdfInput_Content{Content: pdf}, Pages: "2"}}},
		"url denied": {Inputs: []*proteusv1.PdfInput{{Source: &proteusv1.PdfInput_Url{Url: "https://example.com/a.pdf"}}}},
	}
	for name, req := range cases {
		_, err := c.MergePdfs(ctx, req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: code %v (%v), want InvalidArgument", name, connect.CodeOf(err), err)
		}
	}
	for name, req := range map[string]*proteusv1.SplitPdfRequest{
		"no input": {Mode: &proteusv1.SplitPdfRequest_Every{Every: 1}},
		"no mode":  {Input: content(pdf)},
		"every 0":  {Input: content(pdf), Mode: &proteusv1.SplitPdfRequest_Every{Every: 0}},
		"empty range": {Input: content(pdf),
			Mode: &proteusv1.SplitPdfRequest_Ranges{Ranges: &proteusv1.PageRanges{Ranges: []string{""}}}},
	} {
		_, err := c.SplitPdf(ctx, req)
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: code %v (%v), want InvalidArgument", name, connect.CodeOf(err), err)
		}
	}
}

func TestOutputName(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "merged.pdf",
		"a.pdf":                  "a.pdf",
		"dir/sub/x.PDF":          "x.pdf",
		`C:\tmp\y`:               "y.pdf",
		"..":                     "merged.pdf",
		"bad\x00<name>?":         "badname.pdf",
		strings.Repeat("é", 150): strings.Repeat("é", 100) + ".pdf",
	} {
		if got := outputName(in, "merged", 0); got != want {
			t.Errorf("outputName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := outputName("r", "part", 3); got != "r-3.pdf" {
		t.Errorf("part name = %q", got)
	}
}
