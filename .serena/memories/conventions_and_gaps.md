# Conventions, decisions and known gaps

- Every engine call goes through convert.Service.Run with the server loader
  (WithResourceLoader + WithLocalFiles(false)); never let a request reach the default engine
  loader, which reads local disk.
- Remote fetch needs both PROTEUS_ALLOW_REMOTE=true and per-request allow_remote (or url input).
- Errors: convert.BadRequestError -> 422, ErrBusy/ErrQueueFull/rate limit -> 429 + Retry-After,
  ErrTimeout -> 504, body limit -> 413. Internal errors are logged, clients get a generic message.
- Jobs are scoped to the API key id that created them; others get 404 (no IDOR).
- Metrics labels use the route pattern, never raw paths.
- Sync and async share the slot pool; job workers default to half the slots so sync keeps capacity.
- multipart: use the raw Content-Disposition filename (Part.FileName strips directories).
- Tests: go test -race ./...; httpapi/api_test.go runs the real engine end to end, including an
  SSRF test that asserts a loopback server is never hit.

Known gaps / next steps:
- Engine checks ctx only between stages; no page-count or memory budget in the engine.
- Engine rescans Config.FontDirs on every conversion (prefer system font dirs, cached once).
- Jobs are in-memory per instance; plan is Postgres queue (FOR UPDATE SKIP LOCKED) + object
  storage with presigned URLs.
- Not yet: header/footer, PDF/A, templates, batch merge, DOCX, stage timing metrics/tracing.
- Docker image build is verified in CI only (no Docker daemon in the dev container).
