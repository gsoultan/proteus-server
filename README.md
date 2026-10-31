# proteus-server

An HTTP server that converts HTML and Markdown to PDF. It is a thin layer over
[proteus-converter](https://github.com/gsoultan/proteus-converter), the pure-Go
conversion engine: this repo holds transport, authentication, limits, jobs and
metrics, and no conversion logic. There is no browser or LibreOffice at runtime.

```
gpdf               PDF object model and writer
proteus-converter  the engine: HTML / Markdown in, PDF out
proteus-server     this repo: HTTP API, auth, limits, jobs, metrics
```

## Quick start

```sh
go run ./cmd/proteus-server keygen ci
# API key (give to the client, shown once): pk_...
# PROTEUS_API_KEYS entry:                    ci:3f9a...

PROTEUS_API_KEYS='ci:3f9a...' go run ./cmd/proteus-server

curl -H "Authorization: Bearer pk_..." -H 'Content-Type: text/markdown' \
     --data-binary @README.md -o README.pdf http://localhost:8080/v1/convert
```

With Docker (fonts are bundled, so output is the same on every host):

```sh
docker build -t proteus-server .
docker run -p 8080:8080 -e PROTEUS_API_KEYS='ci:3f9a...' proteus-server
```

## API

| Endpoint | Use |
| --- | --- |
| `POST /v1/convert` | Convert synchronously; the response body is the PDF. |
| `POST /v1/jobs` | Queue a conversion; returns `202` with the job. |
| `GET /v1/jobs/{id}` | Job status, and `result_url` once it has succeeded. |
| `GET /v1/jobs/{id}/result` | The PDF of a succeeded job. |
| `DELETE /v1/jobs/{id}` | Drop a finished job and its result. |
| `GET /healthz`, `GET /readyz` | Liveness and readiness probes (no key needed). |
| `GET /metrics` | Prometheus metrics (no key needed; keep it off the public network). |

Send the key as `Authorization: Bearer <key>` or `X-API-Key: <key>`. A job is
visible only to the key that created it.

### Request bodies

The same bodies work for `/v1/convert` and `/v1/jobs`.

**Raw document**: `Content-Type: text/html` or `text/markdown`, body is the document.

**JSON** (`application/json`): exactly one of `html`, `markdown` or `url`.

```json
{
  "html": "<h1>Invoice</h1>",
  "options": {
    "page_size": "A4",
    "landscape": false,
    "margin_mm": 15,
    "margins_mm": {"top": 20, "right": 15, "bottom": 20, "left": 15},
    "stylesheet": "h1 { color: #c00 }",
    "media_type": "print",
    "outline": true,
    "tagged": false,
    "metadata": {"title": "Invoice 42", "author": "ACME", "language": "en"},
    "allow_remote": false
  },
  "webhook_url": "https://example.com/hooks/pdf"
}
```

`webhook_url` is for `/v1/jobs` only. `url` and `allow_remote` need
`PROTEUS_ALLOW_REMOTE=true` on the server.

**Multipart** (`multipart/form-data`) for documents with local assets:

- `file`: the document. The format comes from its file name (`.html`, `.md`)
  or from a `format` field (`html` or `markdown`).
- any other file part: an asset. Its file name, directories included, is the
  path the document uses, so `img/logo.png` serves `<img src="img/logo.png">`.
- `options`: the options object above, as JSON.
- `webhook_url`: as above.

```sh
curl -H "Authorization: Bearer $KEY" \
  -F file=@invoice.html -F asset=@css/site.css\;filename=css/site.css \
  -F 'options={"page_size":"Letter"}' -o invoice.pdf http://localhost:8080/v1/convert
```

### Responses

A successful conversion returns `200` with `Content-Type: application/pdf`,
`X-Proteus-Duration-Ms`, and `X-Proteus-Warnings` (count of non-fatal engine
warnings such as a missing image). Errors are JSON:

```json
{"error": "invalid_request", "message": "unknown page_size \"B9\"", "request_id": "5f0c..."}
```

| Status | When |
| --- | --- |
| `401` | Missing or unknown API key. |
| `413` | Body larger than `PROTEUS_MAX_BODY_BYTES`. |
| `422` | Invalid input, a blocked resource, or output over the size limit. |
| `429` | Rate limit hit, every conversion slot busy, or the job queue full. Honour `Retry-After`. |
| `504` | Conversion exceeded `PROTEUS_TIMEOUT`. |

A full pool answers `429` at once rather than queueing; use `/v1/jobs` for
work that can wait.

### Webhooks

When `PROTEUS_WEBHOOK_SECRET` is set, a job with `webhook_url` gets a `POST`
of its final status. Verify it with:

```
X-Proteus-Timestamp: 1793444400
X-Proteus-Signature: sha256=hex(HMAC_SHA256(secret, timestamp + "." + body))
```

Reject stale timestamps on your side. Delivery makes up to three attempts, retrying on
network errors, `429` and `5xx`. Webhook URLs pass the same SSRF checks as resources.

## Security

Documents are untrusted input, so the engine never touches the server's disk
or network directly. The server replaces the engine's resource loader with one
that serves only:

- `data:` URLs;
- files uploaded with the request (paths with `..` are refused);
- when `PROTEUS_ALLOW_REMOTE=true` and the request sets `allow_remote` (or
  uses `url`): `http` and `https` URLs fetched through `internal/safefetch`.

`safefetch` refuses other schemes and URL credentials, applies the optional
`PROTEUS_ALLOWED_HOSTS` allowlist, and checks the IP address of every
connection after DNS resolution, so DNS rebinding and redirects cannot reach
loopback, private (RFC 1918, ULA), link-local and cloud metadata
(`169.254.169.254`, `fd00:ec2::254`), CGNAT, NAT64/6to4 or reserved ranges.
It ignores proxy environment variables, caps redirects, time and size, and
the loader also caps the number and total size of resources per document.

Other limits: request body size, uploaded asset count, a per-conversion
timeout passed to the engine through `context.Context`, output PDF size, a
bounded conversion pool, and per-key rate limits. API keys are stored only as
SHA-256 hashes. The server refuses to start without keys unless
`PROTEUS_ALLOW_NO_AUTH=true`.

Known gaps: the engine checks the deadline between its stages (parse, style,
layout, paginate, write), so a single huge stage can overrun the timeout; it
has no page-count or memory budget yet.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `PROTEUS_ADDR` | `:8080` | Listen address. |
| `PROTEUS_API_KEYS` | | `id:sha256hex` entries, comma or newline separated. |
| `PROTEUS_API_KEYS_FILE` | | File with more entries (`#` comments allowed). |
| `PROTEUS_ALLOW_NO_AUTH` | `false` | Run without keys (local development only). |
| `PROTEUS_RATE_LIMIT` / `PROTEUS_RATE_BURST` | `10` / `20` | Requests per second and burst, per key. `0` disables. |
| `PROTEUS_WORKERS` | CPU count | Concurrent conversions (sync and async share the pool). |
| `PROTEUS_TIMEOUT` | `30s` | Per conversion. |
| `PROTEUS_MAX_BODY_BYTES` | `20MiB` | Request body limit. |
| `PROTEUS_MAX_ASSETS` | `64` | Uploaded files per request. |
| `PROTEUS_MAX_OUTPUT_BYTES` | `100MiB` | Largest PDF returned. |
| `PROTEUS_ALLOW_REMOTE` | `false` | Allow `url` input and remote resources. |
| `PROTEUS_ALLOWED_HOSTS` | | Comma list; `.example.com` matches subdomains. Empty means any public host. |
| `PROTEUS_FETCH_TIMEOUT` | `10s` | Per remote fetch. |
| `PROTEUS_MAX_RESOURCE_BYTES` | `10MiB` | Per resource. |
| `PROTEUS_MAX_RESOURCES` | `256` | Resources per document. |
| `PROTEUS_MAX_TOTAL_RESOURCE_BYTES` | `50MiB` | All resources of one document. |
| `PROTEUS_SYSTEM_FONTS` | `true` | Use the OS font directories (scanned once per process). |
| `PROTEUS_FONT_DIRS` | | Extra font directories. The engine rescans these on every conversion, so prefer installing fonts in a system directory. |
| `PROTEUS_JOB_WORKERS` | half of workers | Async workers. |
| `PROTEUS_JOB_QUEUE` | `100` | Queued jobs before `429`. |
| `PROTEUS_JOB_TTL` | `1h` | How long finished jobs and results are kept. |
| `PROTEUS_MAX_JOBS` | `1000` | Jobs held in memory. |
| `PROTEUS_WEBHOOK_SECRET` | | HMAC key (32+ characters); enables webhooks. |
| `PROTEUS_SHUTDOWN_TIMEOUT` | `30s` | Drain time on `SIGTERM`. |
| `PROTEUS_LOG_LEVEL` / `PROTEUS_LOG_FORMAT` | `info` / `json` | `debug`…`error`; `json` or `text`. |

Sizes accept `K`, `M`, `G`, `KiB`, `MiB`, `GiB`.

## Operations

- Logs are structured (JSON by default) with `request_id` (taken from a sane
  `X-Request-ID` header or generated, and echoed back), route, status,
  duration and the caller's key id. Keys and documents are never logged.
- Metrics: `proteus_http_requests_total` and
  `proteus_http_request_duration_seconds` per route (RED),
  `proteus_conversions_total{format,outcome}`,
  `proteus_conversion_duration_seconds`, `proteus_output_bytes`,
  `proteus_resource_blocked_total{reason}`, `proteus_conversion_slots` and
  `proteus_conversion_slots_in_use` (saturation), `proteus_job_queue_depth`,
  `proteus_webhook_deliveries_total`, plus Go and process metrics.
- Suggested alerts: p99 of `proteus_http_request_duration_seconds` on
  `POST /v1/convert`, the 5xx rate, slots in use near capacity, and a growing
  job queue.
- `/readyz` turns ready after a warm-up conversion (which loads system fonts)
  and turns unready on `SIGTERM`, before the listener drains.
- Instances are stateless for sync conversion; scale them behind a load
  balancer. Async jobs live in memory on the instance that accepted them, so
  route `/v1/jobs/{id}` back to it (or run one instance) until a shared queue
  lands.

## Roadmap

1. Done here: sync and async endpoints, safe resource loader, limits, API keys
   with rate limits, metrics, Docker.
2. Shared job queue on Postgres (`FOR UPDATE SKIP LOCKED`) and results in
   object storage behind presigned URLs, so any instance can serve any job.
3. Header and footer templates, PDF/A, templates (HTML plus JSON data),
   batch merging.
4. DOCX input, once the engine supports it.

## Development

```sh
go vet ./... && test -z "$(gofmt -l .)" && go test -race ./...
```
