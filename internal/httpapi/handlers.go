package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gsoultan/proteus-server/internal/auth"
	"github.com/gsoultan/proteus-server/internal/convert"
	"github.com/gsoultan/proteus-server/internal/jobs"
)

// parse reads and validates a conversion request, writing the error response
// itself when it fails.
func (a *API) parse(w http.ResponseWriter, r *http.Request) (*convert.Request, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxBodyBytes)
	req, err := convert.ParseHTTP(r, a.cfg.MaxAssets)
	if err != nil {
		a.writeConvertError(w, r, err)
		return nil, false
	}
	return req, true
}

func (a *API) writeConvertError(w http.ResponseWriter, r *http.Request, err error) {
	var bre *convert.BadRequestError
	switch {
	case errors.As(err, &bre):
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_request", bre.Msg)
	case errors.Is(err, convert.ErrBodyTooLarge):
		writeError(w, r, http.StatusRequestEntityTooLarge, "too_large",
			"request body exceeds "+strconv.FormatInt(a.cfg.MaxBodyBytes, 10)+" bytes")
	case errors.Is(err, convert.ErrBusy), errors.Is(err, jobs.ErrQueueFull):
		w.Header().Set("Retry-After", "2")
		writeError(w, r, http.StatusTooManyRequests, "busy", err.Error())
	case errors.Is(err, convert.ErrTimeout):
		writeError(w, r, http.StatusGatewayTimeout, "timeout", err.Error())
	case errors.Is(err, jobs.ErrClosed):
		writeError(w, r, http.StatusServiceUnavailable, "shutting_down", err.Error())
	case errors.Is(err, jobs.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", err.Error())
	case r.Context().Err() != nil:
		// Client went away; nothing useful to send.
		w.WriteHeader(499)
	default:
		a.log.Error("conversion failed", "request_id", requestIDFrom(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "conversion_failed", "conversion failed")
	}
}

// handleConvert is POST /v1/convert: convert synchronously and return the PDF.
// A full pool answers 429 at once instead of queueing.
func (a *API) handleConvert(w http.ResponseWriter, r *http.Request) {
	release, err := a.svc.TryAcquire()
	if err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	defer release()
	req, ok := a.parse(w, r)
	if !ok {
		return
	}
	if req.WebhookURL != "" {
		writeError(w, r, http.StatusUnprocessableEntity, "invalid_request", "webhook_url is only for /v1/jobs")
		return
	}
	res, err := a.svc.Run(r.Context(), req)
	if err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	name := "document.pdf"
	if m := req.Options.Metadata; m != nil && m.Title != "" {
		name = convert.FileName(m.Title)
	}
	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Length", strconv.Itoa(len(res.PDF)))
	h.Set("Content-Disposition", `inline; filename="`+name+`"`)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Proteus-Duration-Ms", strconv.FormatInt(res.Duration.Milliseconds(), 10))
	if n := len(res.Warnings); n > 0 {
		h.Set("X-Proteus-Warnings", strconv.Itoa(n))
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(res.PDF)
}

// handleSubmitJob is POST /v1/jobs: queue a conversion and return 202 with the job.
func (a *API) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	req, ok := a.parse(w, r)
	if !ok {
		return
	}
	job, err := a.jobs.Submit(auth.Caller(r.Context()), req)
	if err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	w.Header().Set("Location", "/v1/jobs/"+job.ID)
	writeJSON(w, http.StatusAccepted, jobView(job))
}

func (a *API) handleGetJob(w http.ResponseWriter, r *http.Request) {
	job, err := a.jobs.Get(auth.Caller(r.Context()), r.PathValue("id"))
	if err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, jobView(job))
}

func (a *API) handleJobResult(w http.ResponseWriter, r *http.Request) {
	pdf, job, err := a.jobs.Result(auth.Caller(r.Context()), r.PathValue("id"))
	if err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	switch job.Status {
	case jobs.Succeeded:
	case jobs.Failed:
		writeError(w, r, http.StatusConflict, "job_failed", job.Error)
		return
	default:
		w.Header().Set("Retry-After", "1")
		writeError(w, r, http.StatusConflict, "not_ready", "job is "+string(job.Status))
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/pdf")
	h.Set("Content-Length", strconv.Itoa(len(pdf)))
	h.Set("Content-Disposition", `attachment; filename="`+job.ID+`.pdf"`)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func (a *API) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	if err := a.jobs.Delete(auth.Caller(r.Context()), r.PathValue("id")); err != nil {
		a.writeConvertError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// jobResponse adds the result link to a job snapshot.
type jobResponse struct {
	jobs.Job
	ResultURL string `json:"result_url,omitempty"`
}

func jobView(j jobs.Job) jobResponse {
	v := jobResponse{Job: j}
	if j.Status == jobs.Succeeded {
		v.ResultURL = "/v1/jobs/" + j.ID + "/result"
	}
	return v
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version})
}

func (a *API) handleReady(w http.ResponseWriter, r *http.Request) {
	if !a.ready.Load() {
		writeError(w, r, http.StatusServiceUnavailable, "not_ready", "starting or shutting down")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "slots": a.svc.Capacity()})
}
