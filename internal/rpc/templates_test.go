package rpc

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
)

func (e *env) templateClient(key string) proteusv1connect.TemplateServiceClient {
	return proteusv1connect.NewTemplateServiceClient(connect.NewClient(
		connecthttp.NewTransport(h2c(), e.url, connecthttp.WithGRPC()), withKey(key)))
}

func (e *env) templateConnectClient(key string) proteusv1connect.TemplateServiceClient {
	return proteusv1connect.NewTemplateServiceClient(connect.NewClient(
		connecthttp.NewTransport(http.DefaultClient, e.url), withKey(key)))
}

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const invoiceHTML = `<html><head><title>Invoice {{.number}}</title></head><body>
<h1>Invoice {{.number}}</h1><p>{{.customer.name}}</p>
<table>{{range $i, $it := .items}}<tr><td>{{$i}}</td><td>{{$it.sku}}</td><td>{{number 2 $it.price}}</td></tr>{{end}}</table>
</body></html>`

func TestTemplatesLifecycle(t *testing.T) {
	e := newEnv(t, envOpts{})
	a, b := e.templateClient(keyA), e.templateClient(keyB)
	ctx := context.Background()
	sample := mustStruct(t, map[string]any{
		"number": "INV-1", "customer": map[string]any{"name": "Sample"},
		"items": []any{map[string]any{"sku": "S", "price": 1}},
	})

	created, err := a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
		Name: " invoice ", Description: "Monthly invoice", Format: proteusv1.InputFormat_INPUT_FORMAT_HTML,
		Content: []byte(invoiceHTML), SampleData: sample,
		Options: &proteusv1.Options{Page: &proteusv1.PageOptions{Size: proteusv1.PageSize_PAGE_SIZE_A4}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	tpl := created.GetTemplate()
	if tpl.GetId() == "" || tpl.GetName() != "invoice" || tpl.GetRevision() != 1 || tpl.GetSizeBytes() != int64(len(invoiceHTML)) ||
		tpl.GetCreateTime() == nil || !bytes.Equal(tpl.GetContent(), []byte(invoiceHTML)) {
		t.Fatalf("created %v", tpl)
	}

	// Names are unique per key, and other keys cannot see the template.
	_, err = a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
		Name: "invoice", Format: proteusv1.InputFormat_INPUT_FORMAT_HTML, Content: []byte("x")}})
	expectCode(t, err, connect.CodeAlreadyExists)
	_, err = b.GetTemplate(ctx, &proteusv1.GetTemplateRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	_, err = b.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeNotFound)

	// Render with the sample data, then with real data.
	r, err := a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{Source: &proteusv1.RenderTemplateRequest_Id{Id: tpl.GetId()}})
	if err != nil || !strings.Contains(string(r.GetContent()), "<p>Sample</p>") {
		t.Fatalf("render with sample = %s, %v", r.GetContent(), err)
	}
	data := mustStruct(t, map[string]any{
		"number": "INV-7", "customer": map[string]any{"name": "<b>Acme</b>"},
		"items": []any{map[string]any{"sku": "A", "price": 1234.5}, map[string]any{"sku": "B", "price": 2}},
	})
	r, err = a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{Source: &proteusv1.RenderTemplateRequest_Id{Id: tpl.GetId()}, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	out := string(r.GetContent())
	for _, want := range []string{"&lt;b&gt;Acme&lt;/b&gt;", "<td>0</td><td>A</td><td>1,234.50</td>", "<td>1</td><td>B</td>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render missing %q:\n%s", want, out)
		}
	}

	// Convert synchronously, over gRPC and the Connect protocol.
	for name, c := range map[string]proteusv1connect.TemplateServiceClient{"grpc": a, "connect": e.templateConnectClient(keyA)} {
		res, err := c.ConvertTemplate(ctx, &proteusv1.ConvertTemplateRequest{Id: tpl.GetId(), Data: data})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		expectPDF(t, res.GetContent())
		if res.GetMimeType() != "application/pdf" || res.GetFileName() != "invoice.pdf" ||
			res.GetOutputFormat() != proteusv1.OutputFormat_OUTPUT_FORMAT_PDF {
			t.Fatalf("%s: response %v", name, res)
		}
	}

	// Queue a job; it is an ordinary job.
	sub, err := a.SubmitTemplateJob(ctx, &proteusv1.SubmitTemplateJobRequest{Id: tpl.GetId(), Data: data})
	if err != nil {
		t.Fatal(err)
	}
	job := waitJob(t, e.grpcClient(keyA), sub.GetJob().GetId())
	if job.GetStatus() != proteusv1.JobStatus_JOB_STATUS_SUCCEEDED || job.GetInputFormat() != proteusv1.InputFormat_INPUT_FORMAT_HTML {
		t.Fatalf("job %v", job)
	}

	// Update with optimistic locking.
	tpl.Description = "changed"
	up, err := a.UpdateTemplate(ctx, &proteusv1.UpdateTemplateRequest{Template: tpl})
	if err != nil || up.GetTemplate().GetRevision() != 2 || up.GetTemplate().GetDescription() != "changed" {
		t.Fatalf("update = %v, %v", up, err)
	}
	_, err = a.UpdateTemplate(ctx, &proteusv1.UpdateTemplateRequest{Template: tpl}) // still revision 1
	expectCode(t, err, connect.CodeAborted)
	_, err = b.UpdateTemplate(ctx, &proteusv1.UpdateTemplateRequest{Template: up.GetTemplate()})
	expectCode(t, err, connect.CodeNotFound)

	// List pages by name without bodies.
	for _, n := range []string{"b-letter", "a-receipt"} {
		if _, err := a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
			Name: n, Format: proteusv1.InputFormat_INPUT_FORMAT_MARKDOWN, Content: []byte("# {{.title}}")}}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := a.ListTemplates(ctx, &proteusv1.ListTemplatesRequest{PageSize: 2})
	if err != nil || len(page.GetTemplates()) != 2 || page.GetTemplates()[0].GetName() != "a-receipt" ||
		page.GetTemplates()[1].GetName() != "b-letter" || page.GetNextPageToken() == "" || page.GetTemplates()[0].GetContent() != nil {
		t.Fatalf("page 1 = %v, %v", page, err)
	}
	page, err = a.ListTemplates(ctx, &proteusv1.ListTemplatesRequest{PageSize: 2, PageToken: page.GetNextPageToken()})
	if err != nil || len(page.GetTemplates()) != 1 || page.GetTemplates()[0].GetName() != "invoice" || page.GetNextPageToken() != "" {
		t.Fatalf("page 2 = %v, %v", page, err)
	}
	if page, err := b.ListTemplates(ctx, &proteusv1.ListTemplatesRequest{}); err != nil || len(page.GetTemplates()) != 0 {
		t.Fatalf("b list = %v, %v", page, err)
	}

	// Delete.
	_, err = b.DeleteTemplate(ctx, &proteusv1.DeleteTemplateRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	if _, err := a.DeleteTemplate(ctx, &proteusv1.DeleteTemplateRequest{Id: tpl.GetId()}); err != nil {
		t.Fatal(err)
	}
	_, err = a.GetTemplate(ctx, &proteusv1.GetTemplateRequest{Id: tpl.GetId()})
	expectCode(t, err, connect.CodeNotFound)
	// The job made from it is unaffected.
	if _, err := e.grpcClient(keyA).GetJob(ctx, &proteusv1.GetJobRequest{Id: job.GetId()}); err != nil {
		t.Fatal(err)
	}
}

func TestTemplateValidation(t *testing.T) {
	e := newEnv(t, envOpts{})
	a := e.templateClient(keyA)
	ctx := context.Background()
	htmlFmt := proteusv1.InputFormat_INPUT_FORMAT_HTML
	bad := map[string]*proteusv1.Template{
		"no name":        {Format: htmlFmt, Content: []byte("x")},
		"no format":      {Name: "n", Content: []byte("x")},
		"no content":     {Name: "n", Format: htmlFmt},
		"syntax":         {Name: "n", Format: htmlFmt, Content: []byte("{{.x")},
		"define":         {Name: "n", Format: htmlFmt, Content: []byte(`{{define "a"}}{{end}}`)},
		"range number":   {Name: "n", Format: htmlFmt, Content: []byte(`{{range 100000000}}{{end}}`)},
		"html context":   {Name: "n", Format: htmlFmt, Content: []byte(`<a href="{{.x}}`)},
		"bad asset path": {Name: "n", Format: htmlFmt, Content: []byte("x"), Assets: []*proteusv1.Asset{{Path: "../etc/passwd"}}},
		"bad options":    {Name: "n", Format: htmlFmt, Content: []byte("x"), Options: &proteusv1.Options{Page: &proteusv1.PageOptions{MarginMm: proto.Float64(500)}}},
		"long name":      {Name: strings.Repeat("n", 201), Format: htmlFmt, Content: []byte("x")},
	}
	for name, tpl := range bad {
		_, err := a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: tpl})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v, want InvalidArgument", name, err)
		}
	}

	// A draft renders without being saved; data that breaks it is InvalidArgument.
	draft := &proteusv1.Template{Name: "draft", Format: htmlFmt, Content: []byte(`<p>{{.a.b}}</p>`),
		SampleData: mustStruct(t, map[string]any{"a": map[string]any{"b": "ok"}})}
	r, err := a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{Source: &proteusv1.RenderTemplateRequest_Template{Template: draft}})
	if err != nil || string(r.GetContent()) != "<p>ok</p>" {
		t.Fatalf("draft = %q, %v", r.GetContent(), err)
	}
	_, err = a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{
		Source: &proteusv1.RenderTemplateRequest_Template{Template: draft}, Data: mustStruct(t, map[string]any{"x": 1})})
	expectCode(t, err, connect.CodeInvalidArgument)
	_, err = a.RenderTemplate(ctx, &proteusv1.RenderTemplateRequest{})
	expectCode(t, err, connect.CodeInvalidArgument)

	// The per-key limit (5 in tests).
	for i := range 5 {
		_, err = a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{
			Name: string(rune('a' + i)), Format: htmlFmt, Content: []byte("x")}})
	}
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.CreateTemplate(ctx, &proteusv1.CreateTemplateRequest{Template: &proteusv1.Template{Name: "f", Format: htmlFmt, Content: []byte("x")}})
	expectCode(t, err, connect.CodeResourceExhausted)
}
