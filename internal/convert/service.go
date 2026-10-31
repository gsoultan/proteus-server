package convert

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	proteus "github.com/gsoultan/proteus-converter"

	"github.com/gsoultan/proteus-server/internal/loader"
	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// ErrBusy is returned when every conversion slot is taken. Callers map it to 429.
var ErrBusy = errors.New("server is at capacity, retry later")

// ErrTimeout is returned when a conversion runs past its deadline.
var ErrTimeout = errors.New("conversion timed out")

// Settings are the server-wide conversion limits and resources.
type Settings struct {
	Workers        int           // concurrent conversions; default 4
	Timeout        time.Duration // per conversion; default 30s
	MaxOutputBytes int64         // reject PDFs larger than this; default 100 MiB
	AllowRemote    bool          // whether callers may enable remote resources at all
	Remote         *safefetch.Client
	LoaderLimits   loader.Limits
	FontDirs       []string
	SystemFonts    bool
	Logger         *slog.Logger
	Metrics        Observer
}

// Observer receives conversion telemetry. Implementations must be safe for concurrent use.
type Observer interface {
	ConversionDone(format string, outcome string, d time.Duration, bytes int)
	ResourceBlocked(reason string)
	SlotsInUse(n int)
}

type nopObserver struct{}

func (nopObserver) ConversionDone(string, string, time.Duration, int) {}
func (nopObserver) ResourceBlocked(string)                            {}
func (nopObserver) SlotsInUse(int)                                    {}

// Service runs conversions on a bounded pool of slots.
type Service struct {
	s     Settings
	slots chan struct{}
	conv  func(ctx context.Context, f proteus.Format, src []byte, opts ...proteus.Option) ([]byte, error)
}

// NewService returns a Service with s applied over the defaults.
func NewService(s Settings) *Service {
	if s.Workers <= 0 {
		s.Workers = 4
	}
	if s.Timeout <= 0 {
		s.Timeout = 30 * time.Second
	}
	if s.MaxOutputBytes <= 0 {
		s.MaxOutputBytes = 100 << 20
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	if s.Metrics == nil {
		s.Metrics = nopObserver{}
	}
	return &Service{s: s, slots: make(chan struct{}, s.Workers), conv: proteus.ConvertBytes}
}

// Capacity returns the number of conversion slots.
func (s *Service) Capacity() int { return cap(s.slots) }

// TryAcquire takes a slot without waiting. It returns ErrBusy when none is free.
// The returned release must be called exactly once.
func (s *Service) TryAcquire() (release func(), err error) {
	select {
	case s.slots <- struct{}{}:
		s.s.Metrics.SlotsInUse(len(s.slots))
		return s.release, nil
	default:
		return nil, ErrBusy
	}
}

// Acquire waits for a slot until ctx is done.
func (s *Service) Acquire(ctx context.Context) (release func(), err error) {
	select {
	case s.slots <- struct{}{}:
		s.s.Metrics.SlotsInUse(len(s.slots))
		return s.release, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Service) release() {
	<-s.slots
	s.s.Metrics.SlotsInUse(len(s.slots))
}

// Result is a finished PDF.
type Result struct {
	PDF      []byte
	Warnings []string
	Duration time.Duration
}

// Run converts req. The caller must hold a slot (TryAcquire or Acquire) and
// release it after Run returns. The engine checks ctx between its stages, so
// a timeout is noticed at the next stage boundary; Run never returns while
// the engine is still working, which keeps the slot count honest.
func (s *Service) Run(ctx context.Context, req *Request) (*Result, error) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, s.s.Timeout)
	defer cancel()

	res, err := s.run(ctx, req)
	d := time.Since(start)
	outcome, size := "ok", 0
	switch {
	case err == nil:
		size = len(res.PDF)
		res.Duration = d
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrTimeout):
		outcome, err = "timeout", ErrTimeout
	case errors.Is(err, context.Canceled):
		outcome = "canceled"
	default:
		var bre *BadRequestError
		if errors.As(err, &bre) {
			outcome = "rejected"
		} else {
			outcome = "error"
		}
	}
	s.s.Metrics.ConversionDone(string(req.Format), outcome, d, size)
	return res, err
}

func (s *Service) run(ctx context.Context, req *Request) (*Result, error) {
	allowRemote := s.s.AllowRemote && s.s.Remote != nil && (req.Options.GetAllowRemote() || req.URL != "")
	var remote *safefetch.Client
	if allowRemote {
		remote = s.s.Remote
	}
	src, base := req.Source, ""
	if req.URL != "" {
		if remote == nil {
			return nil, badf("url input is disabled on this server")
		}
		data, _, err := remote.Get(ctx, req.URL)
		if err != nil {
			if errors.Is(err, safefetch.ErrBlocked) || errors.Is(err, safefetch.ErrTooLarge) {
				return nil, badf("cannot fetch url: %v", err)
			}
			return nil, fmt.Errorf("fetch url: %w", err)
		}
		src, base = data, req.URL
	}

	ld := loader.New(req.Assets, remote, s.s.LoaderLimits)
	ld.OnBlocked = s.s.Metrics.ResourceBlocked

	var (
		mu       sync.Mutex
		warnings []string
	)
	opts := []proteus.Option{
		proteus.WithResourceLoader(ld),
		proteus.WithLocalFiles(false),
		proteus.WithHTTP(allowRemote),
		proteus.WithMaxResourceSize(s.s.LoaderLimits.MaxEach),
		proteus.WithSystemFonts(s.s.SystemFonts),
		proteus.WithWarningHandler(func(msg string) {
			mu.Lock()
			defer mu.Unlock()
			if len(warnings) < 50 {
				warnings = append(warnings, msg)
			}
		}),
	}
	if base != "" {
		opts = append(opts, proteus.WithBaseURL(base))
	}
	for _, d := range s.s.FontDirs {
		opts = append(opts, proteus.WithFontDir(d))
	}
	opts = append(opts, engineOptions(req.Options)...)

	pdf, err := s.safeConvert(ctx, req.Format, src, opts)
	if err != nil {
		if errors.Is(err, proteus.ErrResourceDenied) || errors.Is(err, proteus.ErrResourceTooLarge) {
			return nil, badf("%v", err)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, err
	}
	if int64(len(pdf)) > s.s.MaxOutputBytes {
		return nil, badf("output PDF is larger than the %d byte limit", s.s.MaxOutputBytes)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return nil, errors.New("engine returned invalid PDF output")
	}
	mu.Lock()
	defer mu.Unlock()
	return &Result{PDF: pdf, Warnings: dedupe(warnings)}, nil
}

// safeConvert calls the engine and turns a panic into an error so one bad
// document cannot take the process down.
func (s *Service) safeConvert(ctx context.Context, f proteus.Format, src []byte, opts []proteus.Option) (pdf []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.s.Logger.Error("engine panic", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
			pdf, err = nil, fmt.Errorf("engine panic: %v", r)
		}
	}()
	return s.conv(ctx, f, src, opts...)
}
func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, w := range in {
		w = strings.TrimSpace(w)
		if w == "" || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// Timeout returns the per-conversion time limit.
func (s *Service) Timeout() time.Duration { return s.s.Timeout }
