// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package templates

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
)

// Every starter must validate as a template, pass its own field checks
// with its sample data, compile and render to HTML without a leftover
// template action.
func TestStartersRender(t *testing.T) {
	all, err := Starters("")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(starterOrder) {
		t.Fatalf("%d starters, want %d", len(all), len(starterOrder))
	}
	for i, want := range starterOrder {
		if all[i].GetStarterId() != want {
			t.Fatalf("starter %d is %q, want %q", i, all[i].GetStarterId(), want)
		}
	}
	svc := NewService(NewMemoryStore(), Settings{})
	for _, st := range all {
		t.Run(st.GetStarterId(), func(t *testing.T) {
			if st.GetName() == "" || st.GetDescription() == "" || st.GetLayout() == nil || st.GetSampleData() == nil || len(st.GetFields()) == 0 {
				t.Fatalf("incomplete starter %+v", st)
			}
			if st.GetStarterId() != "blank" && st.GetCategory() == "" {
				t.Fatal("starter has no category")
			}
			if err := CheckData(st.GetFields(), st.GetSampleData()); err != nil {
				t.Fatalf("sample data: %v", err)
			}
			r, err := svc.RenderDraft(context.Background(), st, nil)
			if err != nil {
				t.Fatal(err)
			}
			out := string(r.Document.GetContent())
			if strings.Contains(out, "{{") || strings.Contains(out, "<no value>") {
				t.Fatalf("unrendered output:\n%s", out)
			}
			if !strings.Contains(out, "pt-body") || !strings.Contains(out, "@page") {
				t.Fatalf("not a compiled layout:\n%s", out[:min(len(out), 400)])
			}
		})
	}
}

func TestStarterSpecifics(t *testing.T) {
	svc := NewService(NewMemoryStore(), Settings{})
	ctx := context.Background()

	inv, ok := Starter("invoice")
	if !ok {
		t.Fatal("no invoice starter")
	}
	pdf := proteusv1.OutputFormat_OUTPUT_FORMAT_PDF
	r, err := svc.RenderConvert(ctx, inv, nil, pdf, nil)
	if err != nil {
		t.Fatal(err)
	}
	out := string(r.Document.GetContent())
	for _, want := range []string{"INV-2026-0042", "Rp", "43.761.750", "11%", "Bank Mandiri", "PT Sinar Lestari"} {
		if !strings.Contains(out, want) {
			t.Errorf("invoice output lacks %q", want)
		}
	}
	// The sample is paid, so the status watermark renders "PAID".
	if wm := r.Options.GetPage().GetWatermark(); wm.GetText() != "PAID" {
		t.Errorf("watermark text = %q, want PAID", wm.GetText())
	}
	// Status "sent" has no watermark at all.
	data := inv.GetSampleData()
	data.Fields["status"] = structpb.NewStringValue("sent")
	r, err = svc.RenderConvert(ctx, inv, data, pdf, nil)
	if err != nil {
		t.Fatal(err)
	}
	if wm := r.Options.GetPage().GetWatermark(); wm != nil {
		t.Errorf("watermark for a sent invoice = %v, want none", wm)
	}

	// Starters are copies: changing one does not change the next.
	inv.Name = "changed"
	again, _ := Starter("invoice")
	if again.GetName() == "changed" {
		t.Fatal("Starter returned shared state")
	}

	cert, _ := Starter("certificate")
	if !cert.GetOptions().GetPage().GetLandscape() {
		t.Fatal("certificate is not landscape")
	}
	r, err = svc.RenderDraft(ctx, cert, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(r.Document.GetContent()), "data:image/png;base64,") {
		t.Fatal("certificate has no QR image")
	}

	if _, ok := Starter("nope"); ok {
		t.Fatal("unknown starter found")
	}
	fin, err := Starters("invoice")
	if err != nil || len(fin) != 1 || fin[0].GetStarterId() != "invoice" {
		t.Fatalf("Starters(invoice) = %v, %v", fin, err)
	}
	if _, err := svc.Create(ctx, "o", &proteusv1.Template{Name: "x"}, "nope"); err == nil || !strings.Contains(err.Error(), "starter") {
		t.Fatalf("create from unknown starter = %v", err)
	}
	created, err := svc.Create(ctx, "o", &proteusv1.Template{Name: "My invoice"}, "invoice")
	if err != nil {
		t.Fatal(err)
	}
	if created.GetName() != "My invoice" || created.GetStarterId() != "invoice" || created.GetCategory() != "invoice" || created.GetLayout() == nil || len(created.GetContent()) == 0 {
		t.Fatalf("created from starter = %+v", created)
	}
}
