# Conventions, decisions and known gaps

- Every engine call goes through convert.Service.Run with the server loader
  (WithResourceLoader + WithLocalFiles(false)); never let a request reach the default engine loader.
- Output formats: proto OutputFormat + InputFormat enums; Options split into common fields,
  options.page (paged formats) and options.pdf (PDF only). Responses and jobs carry
  output_format, mime_type and file_name. To add a format: proto enum, target in
  convert/formats.go, register a Converter per input; ListFormats picks it up. Converters must
  load resources only through Input.Loader.
- Remote fetch needs both PROTEUS_ALLOW_REMOTE=true and per-request allow_remote (or url input).
- Errors (internal/rpc/service.go toConnect): BadRequestError -> InvalidArgument, ErrBusy /
  ErrQueueFull / rate limit -> ResourceExhausted, ErrTimeout -> DeadlineExceeded, ErrClosed ->
  Unavailable, ErrNotFound -> NotFound, ErrNotFinished or failed job -> FailedPrecondition, else
  Internal with the cause kept server-side only.
- Jobs and templates are scoped to the API key id that created them; others get NotFound (no IDOR).
- Templates: a filled template is an ordinary Document in the template's input format, then the
  normal convert path / jobs.Submit (so every registry route works; jobs store the rendered doc).
  Render sandbox (internal/templates/render.go), because key holders run code on the shared
  server: html/template (auto-escaping, Markdown escaped as HTML), missingkey=zero, no
  define/template/block, max 3 nested range, no range over number literals, data numbers are
  Number (float, prints without exponent, can't be ranged; compare with 3.0), non-empty lists are
  List iterators that spend a per-render step budget (1M) and check ctx each step, empty lists
  are []any (falsy), len/index/join overridden to accept List and also spend steps, two-variable
  ranges are rewritten to pipe through `pairs`, objects max 500 keys, output capped
  (PROTEUS_MAX_REQUEST_BYTES), render timeout PROTEUS_TEMPLATE_RENDER_TIMEOUT. Template is
  re-parsed per render (funcs are bound to the render; html/template can't Clone after Execute).
- Metric labels: procedure, protocol, code; conversions by input, output, outcome (never raw ids).
- Sync and async share the slot pool; job workers default to half the slots. A worker takes a
  slot before claiming so it never holds a lease while waiting.
- Postgres job queue: lease = timeout + 1m, max 3 attempts, Release on shutdown requeues, results
  stored as bytea until TTL, status reads use the Meta projection (no payload/result).
- storm model gotcha: no Size() on columns with a CHECK, or automigrate drifts every run.
- Codegen: `buf generate` after proto edits, `storm generate internal/storage/store` after model
  edits; both outputs are committed and CI fails if stale.
- Tests: go test -race ./...; internal/rpc/rpc_test.go runs the real engine over gRPC (h2c) and
  Connect, incl. SSRF and job lifecycle; pgstore tests need PROTEUS_TEST_DATABASE_URL.

Known gaps / next steps:
- No CORS yet for browser clients (frontend rules from the user: TypeScript 7, Mantine 9.7.4,
  React 19, TanStack Query/Router, zustand, PWA, web + service worker, connect-es v2, buf, bun,
  Vite 8 apply when a web UI is built).
- Results in bytea; plan object storage with presigned URLs.
- Engine checks ctx only between stages; no page-count or memory budget.
- Engine rescans Config.FontDirs on every conversion (prefer system font dirs).
- Not yet: header/footer, PDF/A, batch merge, DOCX, tracing.
- Templates: no versions history (only a revision counter), no sharing between keys, no
  per-template JSON schema for data; Markdown values inside code blocks show HTML entities.
- Docker image build is verified in CI only (no Docker daemon in the dev container).
