# proteus-server overview

HTTP server (module github.com/gsoultan/proteus-server, go 1.26) that converts HTML and Markdown
to PDF by importing github.com/gsoultan/proteus-converter (the engine, which uses gpdf). The server
holds transport, auth, limits, jobs and metrics only; no conversion logic. DOCX comes later via
the engine.

Layout:
- cmd/proteus-server: main (env config, wiring, warm-up, graceful shutdown) and subcommands
  keygen, hash, healthcheck (for the distroless HEALTHCHECK).
- internal/config: PROTEUS_* env parsing (sizes accept K/M/G/KiB/MiB/GiB).
- internal/safefetch: SSRF-safe HTTP client (scheme/creds/allowlist pre-check, dial-time IP
  check via net.Dialer.Control, no proxy, redirect/time/size caps). IsPublic is the IP policy.
- internal/loader: proteus.ResourceLoader per request: data: URLs, uploaded assets
  (CleanAssetPath rejects ..), remote only through safefetch; count/total/each limits.
- internal/convert: Request parsing (JSON, multipart, raw text/html|text/markdown), option
  validation and mapping to engine options, Service = bounded slot pool (TryAcquire -> 429,
  Acquire for jobs), per-conversion timeout, panic recovery, output checks.
- internal/jobs: in-memory async Manager (bounded queue, workers, TTL sweeper, owner-scoped
  Get/Result/Delete, HMAC-signed webhooks with retries, drain on Shutdown).
- internal/auth: API keys as "id:sha256hex", constant-time compare, per-key x/time/rate limiter.
- internal/metrics: Prometheus registry (RED per route pattern, conversions, slots, queue, webhooks).
- internal/httpapi: Go 1.22 ServeMux routes, request id, recover, access log + metrics, error JSON.

Endpoints: POST /v1/convert, POST /v1/jobs, GET/DELETE /v1/jobs/{id}, GET /v1/jobs/{id}/result,
GET /healthz, /readyz, /metrics. README documents bodies, statuses and every env variable.
Docker: golang:1.26 build -> distroless static nonroot, fonts-liberation2 + dejavu in
/usr/share/fonts (system font scan runs once per process). CI: vet, gofmt, test -race, docker
build + smoke test.
