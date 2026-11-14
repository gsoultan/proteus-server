package rpc

import (
	"context"
	"fmt"
	"path"
	"strings"
	"unicode"

	"connectrpc.com/connect/v2"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/pdfops"
)

// pdfService merges and splits PDF files. It shares the conversion slots and
// time limit, and the error mapping of conversionService.
type pdfService struct {
	conv *conversionService
	ops  *pdfops.Service
}

var _ proteusv1connect.PdfServiceHandler = (*pdfService)(nil)

func (s *pdfService) MergePdfs(ctx context.Context, req *proteusv1.MergePdfsRequest) (*proteusv1.MergePdfsResponse, error) {
	inputs := make([]pdfops.Input, len(req.GetInputs()))
	for i, in := range req.GetInputs() {
		inputs[i] = inputFromProto(in)
	}
	var res *pdfops.Result
	err := s.run(ctx, func(ctx context.Context) (err error) {
		res, err = s.ops.Merge(ctx, inputs, protectionFromProto(req.GetProtection()))
		return err
	})
	if err != nil {
		return nil, s.conv.toConnect(ctx, err)
	}
	f := res.Files[0]
	return &proteusv1.MergePdfsResponse{
		File:     fileToProto(f, outputName(req.GetFileName(), "merged", 0), res.Protected),
		Password: res.Password,
	}, nil
}

func (s *pdfService) SplitPdf(ctx context.Context, req *proteusv1.SplitPdfRequest) (*proteusv1.SplitPdfResponse, error) {
	if req.GetInput() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, "input is required")
	}
	in := inputFromProto(req.GetInput())
	in.Pages = ""
	var res *pdfops.Result
	err := s.run(ctx, func(ctx context.Context) (err error) {
		res, err = s.ops.Split(ctx, in, req.GetRanges().GetRanges(), int(req.GetEvery()), protectionFromProto(req.GetProtection()))
		return err
	})
	if err != nil {
		return nil, s.conv.toConnect(ctx, err)
	}
	out := &proteusv1.SplitPdfResponse{Parts: make([]*proteusv1.PdfFile, len(res.Files)), Password: res.Password}
	for i, f := range res.Files {
		out.Parts[i] = fileToProto(f, outputName(req.GetFileName(), "part", i+1), res.Protected)
	}
	return out, nil
}

// run takes a conversion slot (or answers RESOURCE_EXHAUSTED at once) and runs
// fn under the conversion time limit.
func (s *pdfService) run(ctx context.Context, fn func(context.Context) error) error {
	release, err := s.conv.api.svc.TryAcquire()
	if err != nil {
		return err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, s.conv.api.svc.Timeout())
	defer cancel()
	return fn(ctx)
}

func inputFromProto(in *proteusv1.PdfInput) pdfops.Input {
	return pdfops.Input{Content: in.GetContent(), URL: in.GetUrl(), Pages: in.GetPages(), Password: in.GetPassword()}
}

func protectionFromProto(p *proteusv1.Protection) *pdfops.Protection {
	if p == nil {
		return nil
	}
	return &pdfops.Protection{Password: p.GetPassword(), OwnerPassword: p.GetOwnerPassword(), Generate: p.GetGenerate()}
}

func fileToProto(f pdfops.File, name string, protected bool) *proteusv1.PdfFile {
	return &proteusv1.PdfFile{
		Content: f.Content, MimeType: pdfops.MIMEType, FileName: name,
		PageCount: int32(f.Pages), PasswordProtected: protected,
	}
}

// outputName makes a safe download name from the caller's name (or def): the
// base name only, printable characters, at most 100 runes, ".pdf" added; part
// > 0 appends "-<part>".
func outputName(name, def string, part int) string {
	name = strings.TrimSpace(path.Base(strings.ReplaceAll(name, `\`, "/")))
	name = strings.TrimSuffix(name, path.Ext(name))
	name = strings.Map(func(r rune) rune {
		if !unicode.IsPrint(r) || strings.ContainsRune(`/\:*?"<>|`, r) {
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > 100 {
		name = string(r[:100])
	}
	if name == "" || name == "." || name == ".." {
		name = def
	}
	if part > 0 {
		name = fmt.Sprintf("%s-%d", name, part)
	}
	return name + ".pdf"
}
