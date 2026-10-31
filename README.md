# proteus-server

A gRPC server that converts HTML and Markdown to PDF. It is a thin layer over
[proteus-converter](https://github.com/gsoultan/proteus-converter), the pure-Go
conversion engine: this repo holds transport, authentication, limits, jobs and
metrics, and no conversion logic. There is no browser or LibreOffice at runtime.

```
gpdf               PDF object model and writer
proteus-converter  the engine: HTML / Markdown in, PDF out
proteus-server     this repo: gRPC API, auth, limits, jobs, metrics
```

The API is defined in Protocol Buffers ([`proto/proteus/v1/conversion.proto`](proto/proteus/v1/conversion.proto))
and served with [ConnectRPC](https://connectrpc.com) (connect-go v2). One
listener answers native **gRPC** (HTTP/2, h2c), **gRPC-Web** and the
**Connect** protocol, so the same service works from Go/Java/Python gRPC
clients and from browsers through `@connectrpc/connect-web`. There is no REST
API. Jobs are queued in **PostgreSQL** through [storm](https://github.com/gsoultan/storm)
when `PROTEUS_DATABASE_URL` is set, and in memory otherwise.

## Quick start

```sh
go run ./cmd/proteus-server keygen ci
# API key (give to the client, shown once): pk_...
# PROTEUS_API_KEYS entry:                    ci:3f9a...

PROTEUS_API_KEYS='ci:3f9a...' go run ./cmd/proteus-server

buf curl --protocol grpc --http2-prior-knowledge \
  -H "authorization: Bearer pk_..." \
  --data "{\"document\":{\"format\":\"FORMAT_MARKDOWN\",\"content\":\"$(base64 -w0 README.md)\"}}" \
  http://localhost:8080/proteus.v1.ConversionService/Convert
```

`grpcurl -plaintext localhost:8080 list` works too, through server reflection.

With Docker and Postgres (fonts are bundled, so output is the same on every host):

```sh
docker compose up --build
```

## API

`proteus.v1.ConversionService`:

| RPC | Use |
| --- | --- |
| `Convert` | Convert synchronously; the response carries the PDF. |
| `SubmitJob` | Queue a conversion; returns the job at once. |
| `GetJob` | A job's status, timestamps, error and result size. |
| `DownloadJobResult` | Server stream of the PDF in chunks of at most 1 MiB. |
| `DeleteJob` | Drop a finished job and its result. |

Also served: `grpc.health.v1.Health` (Check and Watch; works with Kubernetes
gRPC probes and `grpc-health-probe`) and `grpc.reflection.v1`/`v1alpha`.
Neither needs a key. Prometheus metrics are on a separate admin listener
(`PROTEUS_ADMIN_ADDR`, `GET /metrics`); keep it off the public network.

Send the key as `authorization: Bearer <key>` or `x-api-key: <key>`. A job is
visible only to the key that created it; any other key gets `NOT_FOUND`.

### Requests

A `Document` has a `format` (`FORMAT_HTML` or `FORMAT_MARKDOWN`) and either
`content` (the document bytes) or `url` (an HTML page to fetch; needs remote
resources enabled). `assets` carries files the document references by relative
path: an asset with path `img/logo.png` serves `<img src="img/logo.png">`.
`Options` sets page size, orientation, margins in millimetres, extra CSS,
media type, bookmarks, tagging, metadata and `allow_remote`. Every field is
documented in the proto file.

### Errors

| Code | When |
| --- | --- |
| `UNAUTHENTICATED` | Missing or unknown API key. |
| `INVALID_ARGUMENT` | Invalid input, a blocked resource, or output over the size limit. |
| `RESOURCE_EXHAUSTED` | Rate limit hit, every conversion slot busy, the job queue full, or a message over the size limit. Retry with backoff. |
| `DEADLINE_EXCEEDED` | Conversion exceeded `PROTEUS_TIMEOUT`. |
| `NOT_FOUND` | Unknown, expired or someone else's job. |
| `FAILED_PRECONDITION` | Result or delete asked for before the job finished, or the job failed. |
| `UNAVAILABLE` | The server is shutting down. |

Error messages are written for callers; internal causes stay in the server log.
A full pool answers `RESOURCE_EXHAUSTED` at once rather than queueing; use
`SubmitJob` for work that can wait. gRPC clients cap received messages at
4 MiB by default, so raise that limit for large synchronous PDFs, or use jobs
and `DownloadJobResult`.

### Webhooks

When `PROTEUS_WEBHOOK_SECRET` is set, a job with `webhook_url` gets a `POST`
of the finished `Job` as protobuf JSON. Verify it with:

```
X-Proteus-Timestamp: 1793444400
X-Proteus-Signature: sha256=hex(HMAC_SHA256(secret, timestamp + "." + body))
```

Reject stale timestamps on your side. Delivery makes up to three attempts,
retrying on network errors, `429` and `5xx`. Webhook URLs pass the same SSRF
checks as resources.

## Jobs and Postgres

Without a database, jobs live in memory on the instance that accepted them and
are lost on restart. With `PROTEUS_DATABASE_URL`, every instance shares one
`jobs` table (in `PROTEUS_DATABASE_SCHEMA`, default `public`), so any instance
can answer any job:

- A worker takes a conversion slot, then claims the oldest queued job with one
  `UPDATE … WHERE id IN (SELECT … FOR UPDATE SKIP LOCKED)`. Two workers never
  take the same job and neither waits.
- A claim is a lease (timeout plus a minute). If an instance dies mid-job, the
  job is claimed again once the lease runs out, up to three attempts, then it
  fails with a clear error.
- On `SIGTERM`, running jobs get the shutdown timeout to finish; ones still
  running are put back in the queue.
- Results are stored in the row (`bytea`) until `PROTEUS_JOB_TTL`, then a
  sweeper deletes them in batches. Status reads use a projection that never
  loads the PDF.

The schema is a storm model ([`internal/storage/model`](internal/storage/model)),
with generated, typed queries in `internal/storage/store`. At startup the
server applies it with storm's automigrate (advisory lock, one transaction,
destructive steps refused). Set `PROTEUS_AUTO_MIGRATE=false` to manage the
schema yourself, for example with `storm diff`.

## Security

Documents are untrusted input, so the engine never touches the server's disk
or network directly. The server replaces the engine's resource loader with one
that serves only:

- `data:` URLs;
- assets sent with the request (paths with `..` are refused);
- when `PROTEUS_ALLOW_REMOTE=true` and the request sets `allow_remote` (or
  uses `url`): `http` and `https` URLs fetched through `internal/safefetch`.

`safefetch` refuses other schemes and URL credentials, applies the optional
`PROTEUS_ALLOWED_HOSTS` allowlist, and checks the IP address of every
connection after DNS resolution, so DNS rebinding and redirects cannot reach
loopback, private (RFC 1918, ULA), link-local and cloud metadata
(`169.254.169.254`, `fd00:ec2::254`), CGNAT, NAT64/6to4 or reserved ranges.
It ignores proxy environment variables, caps redirects, time and size, and
the loader also caps the number and total size of resources per document.

Other limits: request message size, asset count, a per-conversion timeout
passed to the engine through `context.Context`, output PDF size, a bounded
conversion pool, and per-key rate limits. API keys are stored only as SHA-256
hashes. The server refuses to start without keys unless
`PROTEUS_ALLOW_NO_AUTH=true`.

The listener speaks h2c (HTTP/2 without TLS) for gRPC; terminate TLS at your
load balancer or ingress. Browser clients on another origin need CORS, which
the server does not add yet; put them behind the same origin or a proxy.

Known gaps: the engine checks the deadline between its stages (parse, style,
layout, paginate, write), so a single huge stage can overrun the timeout; it
has no page-count or memory budget yet.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `PROTEUS_ADDR` | `:8080` | RPC listener (gRPC, gRPC-Web, Connect). |
| `PROTEUS_ADMIN_ADDR` | `:9090` | Metrics listener; `off` disables it. |
| `PROTEUS_API_KEYS` | | `id:sha256hex` entries, comma or newline separated. |
| `PROTEUS_API_KEYS_FILE` | | File with more entries (`#` comments allowed). |
| `PROTEUS_ALLOW_NO_AUTH` | `false` | Run without keys (local development only). |
| `PROTEUS_RATE_LIMIT` / `PROTEUS_RATE_BURST` | `10` / `20` | Requests per second and burst, per key. `0` disables. |
| `PROTEUS_WORKERS` | CPU count | Concurrent conversions (sync and async share the pool). |
| `PROTEUS_TIMEOUT` | `30s` | Per conversion. |
| `PROTEUS_MAX_REQUEST_BYTES` | `20MiB` | Largest request message. |
| `PROTEUS_MAX_ASSETS` | `64` | Assets per document. |
| `PROTEUS_MAX_OUTPUT_BYTES` | `100MiB` | Largest PDF produced. |
| `PROTEUS_ALLOW_REMOTE` | `false` | Allow `url` input and remote resources. |
| `PROTEUS_ALLOWED_HOSTS` | | Comma list; `.example.com` matches subdomains. Empty means any public host. |
| `PROTEUS_FETCH_TIMEOUT` | `10s` | Per remote fetch. |
| `PROTEUS_MAX_RESOURCE_BYTES` | `10MiB` | Per resource. |
| `PROTEUS_MAX_RESOURCES` | `256` | Resources per document. |
| `PROTEUS_MAX_TOTAL_RESOURCE_BYTES` | `50MiB` | All resources of one document. |
| `PROTEUS_SYSTEM_FONTS` | `true` | Use the OS font directories (scanned once per process). |
| `PROTEUS_FONT_DIRS` | | Extra font directories. The engine rescans these on every conversion, so prefer a system directory. |
| `PROTEUS_DATABASE_URL` | | Postgres URL for the shared job queue; empty keeps jobs in memory. |
| `PROTEUS_DATABASE_SCHEMA` | `public` | Schema holding the `jobs` table. |
| `PROTEUS_AUTO_MIGRATE` | `true` | Create or update the `jobs` table at startup. |
| `PROTEUS_JOB_WORKERS` | half of workers | Async workers per instance. |
| `PROTEUS_JOB_QUEUE` | `100` | Queued jobs before `SubmitJob` answers `RESOURCE_EXHAUSTED`. |
| `PROTEUS_JOB_TTL` | `1h` | How long finished jobs and results are kept. |
| `PROTEUS_MAX_JOBS` | `1000` | Jobs held by the memory store. |
| `PROTEUS_WEBHOOK_SECRET` | | HMAC key (32+ characters); enables webhooks. |
| `PROTEUS_SHUTDOWN_TIMEOUT` | `30s` | Drain time on `SIGTERM`. |
| `PROTEUS_LOG_LEVEL` / `PROTEUS_LOG_FORMAT` | `info` / `json` | `debug`…`error`; `json` or `text`. |

Sizes accept `K`, `M`, `G`, `KiB`, `MiB`, `GiB`.

## Operations

- Logs are structured (JSON by default) with `request_id` (taken from a sane
  `x-request-id` header or generated, and returned as a response header),
  procedure, protocol, code, duration and the caller's key id. Keys and
  documents are never logged.
- Metrics: `proteus_rpc_requests_total{procedure,protocol,code}` and
  `proteus_rpc_duration_seconds{procedure}` (RED),
  `proteus_conversions_total{format,outcome}`,
  `proteus_conversion_duration_seconds`, `proteus_output_bytes`,
  `proteus_resource_blocked_total{reason}`, `proteus_conversion_slots` and
  `proteus_conversion_slots_in_use` (saturation), `proteus_job_queue_depth`,
  `proteus_webhook_deliveries_total`, plus Go and process metrics.
- Suggested alerts: p99 of `proteus_rpc_duration_seconds` for `Convert`, the
  rate of `internal`/`unknown` codes, slots in use near capacity, and a
  growing job queue.
- Health turns `SERVING` after a warm-up conversion (which loads system
  fonts), checks the database when one is configured, and turns
  `NOT_SERVING` on `SIGTERM` before the listener drains. The image's
  `HEALTHCHECK` runs `proteus-server healthcheck`, a gRPC health call.

## Development

```sh
go install github.com/bufbuild/buf/cmd/buf@v1.75.0
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install connectrpc.com/connect/v2/cmd/protoc-gen-connect-go@v2.0.2
go install github.com/gsoultan/storm/cmd/storm@v1.11.0

buf lint && buf generate                    # after editing proto/
storm generate internal/storage/store       # after editing internal/storage/model
go vet ./... && test -z "$(gofmt -l .)" && go test -race ./...
PROTEUS_TEST_DATABASE_URL=postgres://... go test ./internal/storage/pgstore/   # store suite on real Postgres
```

Generated code (`gen/`, `internal/storage/store/`) is committed; CI fails if
it is stale.

## Roadmap

1. Done: gRPC API (ConnectRPC v2), safe resource loader, limits, API keys
   with rate limits, metrics, Docker, Postgres job queue on storm.
2. Results in object storage behind presigned URLs instead of `bytea`, and
   CORS for browser clients.
3. Header and footer templates, PDF/A, templates (HTML plus JSON data),
   batch merging.
4. DOCX input, once the engine supports it.
