package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"

	proteusv1 "github.com/gsoultan/proteus-server/gen/proteus/v1"
	"github.com/gsoultan/proteus-server/gen/proteus/v1/proteusv1connect"
	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
	"github.com/gsoultan/proteus-server/internal/templates"
)

type templateService struct {
	conv *conversionService
}

var _ proteusv1connect.TemplateServiceHandler = (*templateService)(nil)

// toConnect adds the template errors to the conversion error mapping.
func (s *templateService) toConnect(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, templates.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error())
	case errors.Is(err, templates.ErrNameTaken):
		return connect.NewError(connect.CodeAlreadyExists, err.Error())
	case errors.Is(err, templates.ErrRevisionMismatch):
		return connect.NewError(connect.CodeAborted, err.Error())
	case errors.Is(err, templates.ErrLimit):
		return connect.NewError(connect.CodeResourceExhausted, err.Error())
	}
	return s.conv.toConnect(ctx, err)
}

func (s *templateService) tpl() *templates.Service { return s.conv.api.templates }

func (s *templateService) CreateTemplate(ctx context.Context, req *proteusv1.CreateTemplateRequest) (*proteusv1.CreateTemplateResponse, error) {
	t, err := s.tpl().Create(ctx, auth.Caller(ctx), req.GetTemplate())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.CreateTemplateResponse{Template: t}, nil
}

func (s *templateService) GetTemplate(ctx context.Context, req *proteusv1.GetTemplateRequest) (*proteusv1.GetTemplateResponse, error) {
	t, err := s.tpl().Get(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.GetTemplateResponse{Template: t}, nil
}

func (s *templateService) ListTemplates(ctx context.Context, req *proteusv1.ListTemplatesRequest) (*proteusv1.ListTemplatesResponse, error) {
	list, next, err := s.tpl().List(ctx, auth.Caller(ctx), req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.ListTemplatesResponse{Templates: list, NextPageToken: next}, nil
}

func (s *templateService) UpdateTemplate(ctx context.Context, req *proteusv1.UpdateTemplateRequest) (*proteusv1.UpdateTemplateResponse, error) {
	t, err := s.tpl().Update(ctx, auth.Caller(ctx), req.GetTemplate())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.UpdateTemplateResponse{Template: t}, nil
}

func (s *templateService) DeleteTemplate(ctx context.Context, req *proteusv1.DeleteTemplateRequest) (*proteusv1.DeleteTemplateResponse, error) {
	if err := s.tpl().Delete(ctx, auth.Caller(ctx), req.GetId()); err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.DeleteTemplateResponse{}, nil
}

func (s *templateService) RenderTemplate(ctx context.Context, req *proteusv1.RenderTemplateRequest) (*proteusv1.RenderTemplateResponse, error) {
	var (
		r   *templates.Rendered
		err error
	)
	switch src := req.GetSource().(type) {
	case *proteusv1.RenderTemplateRequest_Id:
		r, err = s.tpl().Render(ctx, auth.Caller(ctx), src.Id, req.GetData(), proteusv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED, nil)
	case *proteusv1.RenderTemplateRequest_Template:
		r, err = s.tpl().RenderDraft(ctx, src.Template, req.GetData())
	default:
		err = &convert.BadRequestError{Msg: "id or template is required"}
	}
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.RenderTemplateResponse{Format: r.Document.GetFormat(), Content: r.Document.GetContent()}, nil
}

// ConvertTemplate renders and converts synchronously; like Convert, a full
// pool answers RESOURCE_EXHAUSTED at once.
func (s *templateService) ConvertTemplate(ctx context.Context, req *proteusv1.ConvertTemplateRequest) (*proteusv1.ConvertTemplateResponse, error) {
	r, err := s.tpl().Render(ctx, auth.Caller(ctx), req.GetId(), req.GetData(), req.GetOutputFormat(), req.GetOptions())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	res, err := s.conv.convert(ctx, &proteusv1.ConvertRequest{
		Document: r.Document, OutputFormat: req.GetOutputFormat(), Options: r.Options,
	})
	if err != nil {
		return nil, err
	}
	fileName := res.GetFileName()
	if r.Options.GetMetadata().GetTitle() == "" {
		if f, ok := convert.OutputFromProto(res.GetOutputFormat()); ok {
			fileName = convert.FileName(r.Name, f)
		}
	}
	return &proteusv1.ConvertTemplateResponse{
		Content: res.GetContent(), OutputFormat: res.GetOutputFormat(), MimeType: res.GetMimeType(),
		FileName: fileName, Warnings: res.GetWarnings(), DurationMs: res.GetDurationMs(),
	}, nil
}

// SubmitTemplateJob renders now and queues the conversion of the result,
// so later edits to the template do not change a queued job.
func (s *templateService) SubmitTemplateJob(ctx context.Context, req *proteusv1.SubmitTemplateJobRequest) (*proteusv1.SubmitTemplateJobResponse, error) {
	r, err := s.tpl().Render(ctx, auth.Caller(ctx), req.GetId(), req.GetData(), req.GetOutputFormat(), req.GetOptions())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	rec, err := s.conv.api.jobs.Submit(ctx, auth.Caller(ctx), &proteusv1.SubmitJobRequest{
		Document: r.Document, OutputFormat: req.GetOutputFormat(), Options: r.Options, WebhookUrl: req.GetWebhookUrl(),
	})
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.SubmitTemplateJobResponse{Job: jobs.ToProto(rec)}, nil
}
