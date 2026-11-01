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
- Jobs are scoped to the API key id that created them; others get NotFound (no IDOR).
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
- Not yet: header/footer, PDF/A, templates, batch merge, DOCX, tracing.
- Docker image build is verified in CI only (no Docker daemon in the dev container).

- Copyright header: every hand-written Go file starts with `// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>` followed by a blank line (so it is not the package doc). CI step "Copyright headers" fails on any tracked .go file without it, except files marked `// Code generated ... DO NOT EDIT.` The .proto file carries the same header and protoc-gen-go / protoc-gen-connect-go copy it into gen/ on buf generate; storm has no header option, so its *.gen.go files stay without it.
