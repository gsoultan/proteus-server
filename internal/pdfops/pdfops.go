// Package pdfops merges and splits PDF files with gpdf. It streams: pages are
// copied as they are stored and written to the output as they are copied, so
// a merge holds the inputs and the output bytes but never a decoded document.
package pdfops

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"runtime/debug"
	"strings"

	"github.com/gsoultan/gpdf/doc"
	"github.com/gsoultan/gpdf/security"

	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/safefetch"
)

// MIMEType is the media type of every output.
const MIMEType = "application/pdf"

// Settings bound what one request may do.
type Settings struct {
	// Remote fetches url inputs; nil refuses them.
	Remote *safefetch.Client
	// MaxOutputBytes caps the output of one request (all parts together);
	// default 100 MiB. Writing stops as soon as it is passed.
	MaxOutputBytes int64
	// MaxInputs caps the files of one merge; default 100.
	MaxInputs int
	// MaxParts caps the parts of one split; default 1000.
	MaxParts int
	// MaxPages caps the output pages of one request; default 10000.
	MaxPages int
	Logger   *slog.Logger
}

// Service runs merges and splits. Callers hold a conversion slot and set the
// deadline; Service checks ctx between writes.
type Service struct{ s Settings }

// New returns a Service with s applied over the defaults.
func New(s Settings) *Service {
	if s.MaxOutputBytes <= 0 {
		s.MaxOutputBytes = 100 << 20
	}
	if s.MaxInputs <= 0 {
		s.MaxInputs = 100
	}
	if s.MaxParts <= 0 {
		s.MaxParts = 1000
	}
	if s.MaxPages <= 0 {
		s.MaxPages = 10000
	}
	if s.Logger == nil {
		s.Logger = slog.Default()
	}
	return &Service{s: s}
}

// Input is one PDF to read.
type Input struct {
	Content  []byte
	URL      string
	Pages    string // "1-3,5,8-"; empty = every page
	Password string
}

// Protection encrypts the output. Generate makes a random password, returned
// in Result.Password.
type Protection struct {
	Password      string
	OwnerPassword string
	Generate      bool
}

// File is one output file.
type File struct {
	Content []byte
	Pages   int
}

// Result is the output of a merge (one file) or split (one file per part).
type Result struct {
	Files []File
	// Password is the generated password, when Protection.Generate was set.
	Password  string
	Protected bool
}

// Merge joins the selected pages of inputs, in order.
func (s *Service) Merge(ctx context.Context, inputs []Input, prot *Protection) (res *Result, err error) {
	defer s.recoverPanic(&err)
	if len(inputs) == 0 {
		return nil, bad("at least one input is required")
	}
	if len(inputs) > s.s.MaxInputs {
		return nil, bad("at most %d inputs may be merged at once", s.s.MaxInputs)
	}
	opts, res, err := s.protection(prot)
	if err != nil {
		return nil, err
	}
	sources := make([]doc.PageSource, len(inputs))
	total := 0
	for i, in := range inputs {
		d, err := s.open(ctx, in, i)
		if err != nil {
			return nil, err
		}
		defer d.Close()
		pages, err := selectPages(d, in.Pages, i)
		if err != nil {
			return nil, err
		}
		total += len(pages)
		if total > s.s.MaxPages {
			return nil, bad("the output may have at most %d pages", s.s.MaxPages)
		}
		sources[i] = doc.PageSource{Doc: d, Pages: pages}
	}
	out := newLimitedBuffer(ctx, s.s.MaxOutputBytes, s.s.MaxOutputBytes)
	n, err := doc.MergeTo(out, opts, sources...)
	if err != nil {
		return nil, out.cause(err)
	}
	res.Files = []File{{Content: out.Bytes(), Pages: n}}
	return res, nil
}

// Split writes one part per selection in ranges, or parts of every pages when
// ranges is empty.
func (s *Service) Split(ctx context.Context, in Input, ranges []string, every int, prot *Protection) (res *Result, err error) {
	defer s.recoverPanic(&err)
	if len(ranges) == 0 && every <= 0 {
		return nil, bad("ranges or a positive every is required")
	}
	if len(ranges) > 0 && every > 0 {
		return nil, bad("give ranges or every, not both")
	}
	if len(ranges) > s.s.MaxParts {
		return nil, bad("at most %d parts may be made at once", s.s.MaxParts)
	}
	opts, res, err := s.protection(prot)
	if err != nil {
		return nil, err
	}
	d, err := s.open(ctx, in, 0)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	all, err := d.Pages()
	if err != nil {
		return nil, bad("input 1 is not a readable PDF: %v", err)
	}
	count := len(all)
	if count == 0 {
		return nil, bad("input 1 has no pages")
	}
	var parts [][]int
	if every > 0 {
		if (count+every-1)/every > s.s.MaxParts {
			return nil, bad("at most %d parts may be made at once", s.s.MaxParts)
		}
		for start := 0; start < count; start += every {
			part := make([]int, 0, min(every, count-start))
			for p := start; p < min(start+every, count); p++ {
				part = append(part, p)
			}
			parts = append(parts, part)
		}
	} else {
		total := 0
		for i, spec := range ranges {
			if strings.TrimSpace(spec) == "" {
				return nil, bad("range %d is empty", i+1)
			}
			pages, err := doc.ParsePageRanges(spec, count)
			if err != nil {
				return nil, bad("range %d: %v", i+1, err)
			}
			total += len(pages)
			if total > s.s.MaxPages {
				return nil, bad("the output may have at most %d pages", s.s.MaxPages)
			}
			parts = append(parts, pages)
		}
	}
	// One budget for all parts: each part's buffer gets what the earlier ones left.
	budget := s.s.MaxOutputBytes
	res.Files = make([]File, 0, len(parts))
	for _, pages := range parts {
		out := newLimitedBuffer(ctx, budget, s.s.MaxOutputBytes)
		n, err := doc.MergeTo(out, opts, doc.PageSource{Doc: d, Pages: pages})
		if err != nil {
			return nil, out.cause(err)
		}
		budget -= int64(out.Len())
		res.Files = append(res.Files, File{Content: out.Bytes(), Pages: n})
	}
	return res, nil
}

// protection turns prot into gpdf options and the result shell carrying a
// generated password.
func (s *Service) protection(prot *Protection) (doc.CopyOptions, *Result, error) {
	res := &Result{}
	if prot == nil {
		return doc.CopyOptions{}, res, nil
	}
	user := prot.Password
	switch {
	case prot.Generate && user != "":
		return doc.CopyOptions{}, nil, bad("protection: give a password or set generate, not both")
	case prot.Generate:
		p, err := GeneratePassword()
		if err != nil {
			return doc.CopyOptions{}, nil, err
		}
		user, res.Password = p, p
	case user == "":
		return doc.CopyOptions{}, nil, bad("protection: a password or generate is required")
	}
	if len(user) > 127 || len(prot.OwnerPassword) > 127 {
		return doc.CopyOptions{}, nil, bad("protection: passwords may be at most 127 bytes")
	}
	res.Protected = true
	return doc.CopyOptions{UserPassword: user, OwnerPassword: prot.OwnerPassword}, res, nil
}

// open fetches (for url inputs) and opens input i.
func (s *Service) open(ctx context.Context, in Input, i int) (doc.Document, error) {
	data := in.Content
	if in.URL != "" {
		if s.s.Remote == nil {
			return nil, bad("input %d: url input is disabled on this server", i+1)
		}
		body, _, err := s.s.Remote.Get(ctx, in.URL)
		if err != nil {
			if errors.Is(err, safefetch.ErrBlocked) || errors.Is(err, safefetch.ErrTooLarge) {
				return nil, bad("input %d: cannot fetch url: %v", i+1, err)
			}
			return nil, fmt.Errorf("input %d: fetch url: %w", i+1, err)
		}
		data = body
	}
	if len(data) == 0 {
		return nil, bad("input %d is empty", i+1)
	}
	d, err := doc.OpenReaderWithPassword(bytes.NewReader(data), int64(len(data)), in.Password)
	switch {
	case errors.Is(err, security.ErrWrongPassword) && in.Password == "":
		return nil, bad("input %d is password protected: give its password", i+1)
	case errors.Is(err, security.ErrWrongPassword):
		return nil, bad("input %d: wrong password", i+1)
	case err != nil:
		return nil, bad("input %d is not a readable PDF", i+1)
	}
	return d, nil
}

func selectPages(d doc.Document, spec string, i int) ([]int, error) {
	all, err := d.Pages()
	if err != nil {
		return nil, bad("input %d is not a readable PDF: %v", i+1, err)
	}
	pages, err := doc.ParsePageRanges(spec, len(all))
	if err != nil {
		return nil, bad("input %d: %v", i+1, err)
	}
	return pages, nil
}

// recoverPanic turns a panic in gpdf (a malformed file it does not guard
// against) into an error so one bad file cannot take the process down.
func (s *Service) recoverPanic(err *error) {
	if r := recover(); r != nil {
		s.s.Logger.Error("pdf operation panic", "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
		*err = bad("an input could not be read as a PDF")
	}
}

// ErrTooLarge is returned when the output passes Settings.MaxOutputBytes.
var ErrTooLarge = errors.New("output too large")

// limitedBuffer collects output, failing a write once ctx is done or the byte
// budget is spent, so a merge stops early instead of finishing a doomed file.
type limitedBuffer struct {
	bytes.Buffer
	ctx   context.Context
	limit int64 // bytes this buffer may take
	max   int64 // the request's limit, for the error message
	err   error
}

func newLimitedBuffer(ctx context.Context, limit, max int64) *limitedBuffer {
	return &limitedBuffer{ctx: ctx, limit: limit, max: max}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.err != nil {
		return 0, b.err
	}
	if err := b.ctx.Err(); err != nil {
		b.err = err
		return 0, err
	}
	if int64(b.Len()+len(p)) > b.limit {
		b.err = ErrTooLarge
		return 0, b.err
	}
	return b.Buffer.Write(p)
}

var _ io.Writer = (*limitedBuffer)(nil)

// cause reports why a merge failed: the buffer's own error (deadline, size)
// wins over the wrapped error gpdf returns.
func (b *limitedBuffer) cause(err error) error {
	switch {
	case errors.Is(b.err, ErrTooLarge):
		return bad("the output is larger than the %d byte limit", b.max)
	case b.err != nil:
		return b.err
	case errors.Is(err, doc.ErrNoPages):
		return bad("no pages selected")
	}
	return err
}

func bad(format string, a ...any) error {
	return &convert.BadRequestError{Msg: fmt.Sprintf(format, a...)}
}

// passwordAlphabet leaves out look-alike characters (0/O, 1/l/I) so a
// generated password can be read out or typed.
const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePassword returns a random 20-character password (about 116 bits).
func GeneratePassword() (string, error) {
	var b strings.Builder
	size := big.NewInt(int64(len(passwordAlphabet)))
	for range 20 {
		n, err := rand.Int(rand.Reader, size)
		if err != nil {
			return "", err
		}
		b.WriteByte(passwordAlphabet[n.Int64()])
	}
	return b.String(), nil
}
