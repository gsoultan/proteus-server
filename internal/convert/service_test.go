// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

package convert

import (
	"context"
	"errors"
	"testing"
	"time"
)

func stubService(t *testing.T, timeout time.Duration, fn func(ctx context.Context) ([]byte, error)) *Service {
	t.Helper()
	return NewService(Settings{Workers: 1, Timeout: timeout, Registry: stubRegistry(fn)})
}

func stubRegistry(fn func(ctx context.Context) ([]byte, error)) *Registry {
	r := NewRegistry()
	r.Register(HTML, PDF, ConverterFunc(func(ctx context.Context, _ *Input) ([]byte, error) { return fn(ctx) }))
	return r
}

var htmlReq = &Request{Format: HTML, Output: PDF, Source: []byte("<p>x</p>")}

func TestTimeout(t *testing.T) {
	s := stubService(t, 20*time.Millisecond, func(ctx context.Context) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if _, err := s.Run(context.Background(), htmlReq); !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestPanicBecomesError(t *testing.T) {
	s := stubService(t, time.Second, func(context.Context) ([]byte, error) { panic("boom") })
	if _, err := s.Run(context.Background(), htmlReq); err == nil {
		t.Fatal("panic swallowed without error")
	}
}

func TestOutputChecks(t *testing.T) {
	s := stubService(t, time.Second, func(context.Context) ([]byte, error) { return []byte("not a pdf"), nil })
	if _, err := s.Run(context.Background(), htmlReq); err == nil {
		t.Fatal("non-PDF output accepted")
	}
	s = NewService(Settings{Workers: 1, MaxOutputBytes: 10, Registry: stubRegistry(func(context.Context) ([]byte, error) {
		return []byte("%PDF-1.7 and a lot more"), nil
	})})
	var bre *BadRequestError
	if _, err := s.Run(context.Background(), htmlReq); !errors.As(err, &bre) {
		t.Fatalf("oversized output: %v", err)
	}
}

func TestSlots(t *testing.T) {
	s := NewService(Settings{Workers: 2})
	r1, err1 := s.TryAcquire()
	r2, err2 := s.TryAcquire()
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if _, err := s.TryAcquire(); !errors.Is(err, ErrBusy) {
		t.Fatalf("third acquire = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := s.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire = %v", err)
	}
	r1()
	r2()
	if r, err := s.TryAcquire(); err != nil {
		t.Fatal(err)
	} else {
		r()
	}
}

func TestRealEngineMarkdown(t *testing.T) {
	s := NewService(Settings{Workers: 1, SystemFonts: false})
	res, err := s.Run(context.Background(), &Request{Format: Markdown, Output: PDF, Source: []byte("# Hi\n\n| a | b |\n|---|---|\n| 1 | 2 |\n")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Content) < 500 || res.Target.MIMEType != "application/pdf" {
		t.Fatalf("result: %d bytes, %+v", len(res.Content), res.Target)
	}
}

func TestRegistry(t *testing.T) {
	s := NewService(Settings{Workers: 1})
	if got := s.Routes(); len(got) != 2 || got[0] != (Route{HTML, PDF}) || got[1] != (Route{Markdown, PDF}) {
		t.Fatalf("default routes %v", got)
	}
	var bre *BadRequestError
	if err := s.Check(&Request{Format: HTML, Output: "docx"}); !errors.As(err, &bre) {
		t.Fatalf("unsupported route: %v", err)
	}
	if _, err := s.Run(context.Background(), &Request{Format: "rtf", Output: PDF}); !errors.As(err, &bre) {
		t.Fatalf("Run on unsupported route: %v", err)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("registering an unknown output format should panic")
		}
	}()
	NewRegistry().Register(HTML, "docx", ConverterFunc(nil))
}
