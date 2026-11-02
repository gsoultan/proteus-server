// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
	"google.golang.org/protobuf/types/known/structpb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// Every starter converts to a PDF through the whole pipeline, from a
// stored copy and as an unsaved draft.
func TestStartersToPDF(t *testing.T) {
	e := newEnv(t, envOpts{})
	a := e.templateConnectClient(keyA)
	ctx := context.Background()

	list, err := a.ListStarterTemplates(ctx, &proteusv1.ListStarterTemplatesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.GetStarters()) < 9 {
		t.Fatalf("%d starters", len(list.GetStarters()))
	}
	fin, err := a.ListStarterTemplates(ctx, &proteusv1.ListStarterTemplatesRequest{Category: "receipt"})
	if err != nil || len(fin.GetStarters()) != 1 || fin.GetStarters()[0].GetStarterId() != "receipt" {
		t.Fatalf("receipt starters = %v, %v", fin, err)
	}
	for _, st := range list.GetStarters() {
		t.Run(st.GetStarterId(), func(t *testing.T) {
			if st.GetId() != "" {
				t.Fatal("starter has an id")
			}
			res, err := a.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Template: st, OutputFormat: proteusv1.OutputFormat_OUTPUT_FORMAT_PDF})
			if err != nil {
				t.Fatal(err)
			}
			expectPDF(t, res.GetContent())
			if res.GetFileName() != "document.pdf" && !strings.HasSuffix(res.GetFileName(), ".pdf") {
				t.Fatalf("file name %q", res.GetFileName())
			}
			for _, w := range res.GetWarnings() {
				// The starters reference a logo the user is expected to upload;
				// anything else is a real problem in the starter.
				if !strings.Contains(w, "assets/logo.png") {
					t.Errorf("warning: %s", w)
				}
			}
		})
	}
}

func TestBuilderLifecycle(t *testing.T) {
	e := newEnv(t, envOpts{})
	a, b := e.templateClient(keyA), e.templateClient(keyB)
	ctx := context.Background()

	// Create from a starter, renaming it; the layout and fields come along.
	created, err := a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{
		StarterId: "invoice", Template: &proteusv1.Template{Name: "Studio invoice", Description: "Ours"},
	})
	if err != nil {
		t.Fatal(err)
	}
	tpl := created.GetTemplate()
	if tpl.GetName() != "Studio invoice" || tpl.GetCategory() != "invoice" || tpl.GetStarterId() != "invoice" ||
		tpl.GetLayout() == nil || len(tpl.GetFields()) == 0 || len(tpl.GetContent()) == 0 || tpl.GetDescription() != "Ours" {
		t.Fatalf("created %+v", tpl)
	}
	_, err = a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{StarterId: "nope"})
	expectCode(t, err, connect.CodeInvalidArgument)

	// A plain code template in another category.
	if _, err := a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
		Name: "note", Category: "letter", Format: proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN, Content: []byte("# {{.title}}"),
	}}); err != nil {
		t.Fatal(err)
	}
	for cat, want := range map[string]int{"": 2, "invoice": 1, "letter": 1, "other": 0} {
		got, err := a.ListTemplates(ctx, &proteusv1.ListTemplatesRequest{Category: cat})
		if err != nil || len(got.GetTemplates()) != want {
			t.Fatalf("list %q = %d, %v; want %d", cat, len(got.GetTemplates()), err, want)
		}
	}
	_, err = a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
		Name: "bad", Category: "has space", Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Content: []byte("x"),
	}})
	expectCode(t, err, connect.CodeInvalidArgument)

	// Rendering and converting with the sample data.
	rendered, err := a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{Source: &proteusv1.RenderTemplateRequest_Id{Id: tpl.GetId()}})
	if err != nil {
		t.Fatal(err)
	}
	if out := string(rendered.GetContent()); !strings.Contains(out, "INV-2026-0042") || !strings.Contains(out, "Rp") {
		t.Fatalf("html output:\n%s", out[:min(len(out), 600)])
	}
	res, err := a.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Id: tpl.GetId()})
	if err != nil {
		t.Fatal(err)
	}
	expectPDF(t, res.GetContent())
	if !strings.HasPrefix(res.GetFileName(), "Studio") || !strings.HasSuffix(res.GetFileName(), ".pdf") {
		t.Fatalf("file name %q", res.GetFileName())
	}

	// Data that does not fit the fields is refused with every problem listed.
	bad, _ := structpb.NewStruct(map[string]any{"number": "X", "issued": "soon", "items": "none"})
	_, err = a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{Source: &proteusv1.RenderTemplateRequest_Id{Id: tpl.GetId()}, Data: bad})
	expectCode(t, err, connect.CodeInvalidArgument)
	var cerr *connect.Error
	if !errors.As(err, &cerr) {
		t.Fatalf("%T", err)
	}
	var detail *proteusv1.TemplateDataError
	for _, d := range cerr.Details() {
		msg, uerr := connectproto.UnmarshalErrorDetail(d)
		if uerr == nil {
			if de, ok := msg.(*proteusv1.TemplateDataError); ok {
				detail = de
			}
		}
	}
	if detail == nil {
		t.Fatalf("no TemplateDataError detail in %v", err)
	}
	paths := map[string]bool{}
	for _, p := range detail.GetProblems() {
		paths[p.GetPath()] = true
	}
	for _, want := range []string{"issued", "seller", "client", "items", "total"} {
		if !paths[want] {
			t.Errorf("no problem for %s in %v", want, detail.GetProblems())
		}
	}

	// Drafts: id and template are exclusive; a broken layout names the block.
	_, err = a.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Id: tpl.GetId(), Template: tpl})
	expectCode(t, err, connect.CodeInvalidArgument)
	broken := tpl
	broken.Layout.Body.Blocks[0].Kind = &proteusv1.Block_Field{Field: &proteusv1.Field{Path: "a..b"}}
	_, err = a.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Template: broken})
	expectCode(t, err, connect.CodeInvalidArgument)
	if !strings.Contains(err.Error(), `block "`+broken.Layout.Body.Blocks[0].GetId()+`"`) {
		t.Fatalf("layout error does not name the block: %v", err)
	}

	// Other keys see nothing.
	_, err = b.ConvertToCode(ctx, &proteusv1.ConvertToCodeRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeNotFound)

	// Convert to code: the compiled HTML stays, the layout goes, the
	// revision moves on and a stale revision is refused.
	_, err = a.ConvertToCode(ctx, &proteusv1.ConvertToCodeRequest{Id: tpl.GetId(), Revision: 7})
	expectCode(t, err, connect.CodeAborted)
	code, err := a.ConvertToCode(ctx, &proteusv1.ConvertToCodeRequest{Id: tpl.GetId(), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	ct := code.GetTemplate()
	if ct.GetLayout() != nil || ct.GetRevision() != 2 || !strings.Contains(string(ct.GetContent()), "pt-body") ||
		!strings.HasPrefix(string(ct.GetContent()), "<!-- proteus: locale=id-ID currency=IDR -->") || len(ct.GetFields()) == 0 {
		t.Fatalf("converted %+v", ct)
	}
	_, err = a.ConvertToCode(ctx, &proteusv1.ConvertToCodeRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeInvalidArgument)
	res, err = a.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Id: tpl.GetId(), OutputFormat: proteusv1.OutputFormat_OUTPUT_FORMAT_PDF})
	if err != nil {
		t.Fatal(err)
	}
	expectPDF(t, res.GetContent())
}
