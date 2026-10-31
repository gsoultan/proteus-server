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
)

// chunkSize is the DownloadJobResult message size.
const chunkSize = 1 << 20

type conversionService struct {
	api *API
}

var _ proteusv1connect.ConversionServiceHandler = (*conversionService)(nil)

// toConnect maps internal errors to RPC errors. Only messages written here
// reach the caller; causes stay on the server.
func (s *conversionService) toConnect(ctx context.Context, err error) error {
	var bre *convert.BadRequestError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &bre):
		return connect.NewError(connect.CodeInvalidArgument, bre.Msg)
	case errors.Is(err, convert.ErrBusy), errors.Is(err, jobs.ErrQueueFull):
		return connect.NewError(connect.CodeResourceExhausted, err.Error())
	case errors.Is(err, convert.ErrTimeout):
		return connect.NewError(connect.CodeDeadlineExceeded, err.Error())
	case errors.Is(err, jobs.ErrClosed):
		return connect.NewError(connect.CodeUnavailable, err.Error())
	case errors.Is(err, jobs.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err.Error())
	case errors.Is(err, jobs.ErrNotFinished):
		return connect.NewError(connect.CodeFailedPrecondition, err.Error())
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, "canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, "deadline exceeded")
	}
	s.api.log.Error("request failed", "request_id", requestIDFrom(ctx), "error", err)
	return connect.NewError(connect.CodeInternal, "internal error").WithCause(err)
}

// Convert implements the synchronous conversion. A full pool answers
// RESOURCE_EXHAUSTED at once instead of queueing.
func (s *conversionService) Convert(ctx context.Context, req *proteusv1.ConvertRequest) (*proteusv1.ConvertResponse, error) {
	return s.convert(ctx, req)
}

// convert runs a synchronous conversion and returns RPC errors.
func (s *conversionService) convert(ctx context.Context, req *proteusv1.ConvertRequest) (*proteusv1.ConvertResponse, error) {
	parsed, err := convert.FromProto(req.GetDocument(), req.GetOutputFormat(), req.GetOptions(), convert.Limits{MaxAssets: s.api.cfg.MaxAssets})
	if err == nil {
		err = s.api.svc.Check(parsed)
	}
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	release, err := s.api.svc.TryAcquire()
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	defer release()
	res, err := s.api.svc.Run(ctx, parsed)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.ConvertResponse{
		Content:      res.Content,
		OutputFormat: convert.OutputToProto(res.Target.Format),
		MimeType:     res.Target.MIMEType,
		FileName:     convert.FileName(req.GetOptions().GetMetadata().GetTitle(), res.Target.Format),
		Warnings:     res.Warnings,
		DurationMs:   res.Duration.Milliseconds(),
	}, nil
}

// ListFormats returns the conversions the server supports.
func (s *conversionService) ListFormats(context.Context, *proteusv1.ListFormatsRequest) (*proteusv1.ListFormatsResponse, error) {
	res := &proteusv1.ListFormatsResponse{}
	for _, rt := range s.api.svc.Routes() {
		t, _ := convert.TargetOf(rt.To)
		res.Conversions = append(res.Conversions, &proteusv1.Conversion{
			InputFormat: convert.InputToProto(rt.From), OutputFormat: convert.OutputToProto(rt.To),
			MimeType: t.MIMEType, FileExtension: t.Extension,
		})
	}
	return res, nil
}

// SubmitJob queues a conversion.
func (s *conversionService) SubmitJob(ctx context.Context, req *proteusv1.SubmitJobRequest) (*proteusv1.SubmitJobResponse, error) {
	rec, err := s.api.jobs.Submit(ctx, auth.Caller(ctx), req)
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.SubmitJobResponse{Job: jobs.ToProto(rec)}, nil
}

// GetJob returns a job of the caller.
func (s *conversionService) GetJob(ctx context.Context, req *proteusv1.GetJobRequest) (*proteusv1.GetJobResponse, error) {
	rec, err := s.api.jobs.Get(ctx, auth.Caller(ctx), req.GetId())
	if err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.GetJobResponse{Job: jobs.ToProto(rec)}, nil
}

// DownloadJobResult streams the output in chunks of at most 1 MiB.
func (s *conversionService) DownloadJobResult(ctx context.Context, req *proteusv1.DownloadJobResultRequest,
	stream proteusv1connect.ConversionServiceDownloadJobResultServerStream) error {
	rec, out, err := s.api.jobs.Result(ctx, auth.Caller(ctx), req.GetId())
	if errors.Is(err, jobs.ErrNotFinished) && rec != nil && rec.Status == jobs.Failed {
		return connect.Errorf(connect.CodeFailedPrecondition, "job failed: %s", rec.Error)
	}
	if err != nil {
		return s.toConnect(ctx, err)
	}
	for len(out) > 0 {
		n := min(len(out), chunkSize)
		if err := stream.Send(&proteusv1.DownloadJobResultResponse{Chunk: out[:n]}); err != nil {
			return err
		}
		out = out[n:]
	}
	return nil
}

// DeleteJob removes a finished job.
func (s *conversionService) DeleteJob(ctx context.Context, req *proteusv1.DeleteJobRequest) (*proteusv1.DeleteJobResponse, error) {
	if err := s.api.jobs.Delete(ctx, auth.Caller(ctx), req.GetId()); err != nil {
		return nil, s.toConnect(ctx, err)
	}
	return &proteusv1.DeleteJobResponse{}, nil
}
