package pdfops

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gsoultan/gpdf/doc"

	"github.com/gsoultan/proteus-server/internal/convert"
)

func samplePDF(t *testing.T, pages int) []byte {
	t.Helper()
	b := doc.New()
	for p := range pages {
		b.AddPage()
		b.DrawText(strings.Repeat("x", 50)+string(rune('A'+p)), 72, 700, "Helvetica", 12)
	}
	d, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func isBad(err error) bool {
	var bre *convert.BadRequestError
	return errors.As(err, &bre)
}

func TestOutputLimitStopsEarly(t *testing.T) {
	pdf := samplePDF(t, 3)
	s := New(Settings{MaxOutputBytes: 300})
	if _, err := s.Merge(context.Background(), []Input{{Content: pdf}}, nil); !isBad(err) || !strings.Contains(err.Error(), "300 byte limit") {
		t.Fatalf("merge over the limit: %v", err)
	}
	if _, err := s.Split(context.Background(), Input{Content: pdf}, nil, 1, nil); !isBad(err) {
		t.Fatalf("split over the limit: %v", err)
	}
}

func TestCanceledContextStopsMerge(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New(Settings{}).Merge(ctx, []Input{{Content: samplePDF(t, 2)}}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestLimits(t *testing.T) {
	pdf := samplePDF(t, 2)
	s := New(Settings{MaxInputs: 2, MaxPages: 3, MaxParts: 1})
	if _, err := s.Merge(context.Background(), []Input{{Content: pdf}, {Content: pdf}, {Content: pdf}}, nil); !isBad(err) {
		t.Errorf("too many inputs: %v", err)
	}
	if _, err := s.Merge(context.Background(), []Input{{Content: pdf}, {Content: pdf}}, nil); !isBad(err) {
		t.Errorf("too many pages: %v", err)
	}
	if _, err := s.Split(context.Background(), Input{Content: pdf}, nil, 1, nil); !isBad(err) {
		t.Errorf("too many parts: %v", err)
	}
	res, err := s.Split(context.Background(), Input{Content: pdf}, []string{"2-1"}, 0, nil)
	if err != nil || len(res.Files) != 1 || res.Files[0].Pages != 2 {
		t.Fatalf("split = %+v, %v", res, err)
	}
}

func TestGeneratePassword(t *testing.T) {
	a, err := GeneratePassword()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := GeneratePassword()
	if len(a) != 20 || a == b || strings.ContainsAny(a, "0O1lI") {
		t.Fatalf("passwords %q %q", a, b)
	}
}
