# proteus-server overview

gRPC server (module github.com/gsoultan/proteus-server, go 1.27 because storm v1.11 needs it) that
converts HTML and Markdown to PDF by importing github.com/gsoultan/proteus-converter (the engine,
which uses gpdf). The server holds transport, auth, limits, jobs and metrics only; no conversion
logic. DOCX comes later via the engine. User rule: gRPC only, no REST; storm for anything database.

API: proto/proteus/v1/conversion.proto, service proteus.v1.ConversionService with Convert,
SubmitJob, GetJob, DownloadJobResult (server stream, 1 MiB chunks), DeleteJob. Served by
connect-go v2 (connectrpc.com/connect/v2) so one h2c listener answers gRPC, gRPC-Web and Connect.
Also grpc.health.v1 (grpchealth/v2) and reflection (grpcreflect/v2). Metrics on a separate admin
listener (PROTEUS_ADMIN_ADDR :9090, GET /metrics).

Layout:
- proto/, buf.yaml, buf.gen.yaml -> gen/proteus/v1 (protoc-gen-go + protoc-gen-connect-go, committed).
- cmd/proteus-server: main (env config, store choice, wiring, warm-up, graceful shutdown) and
  subcommands keygen, hash, healthcheck (gRPC health over h2c, for the distroless HEALTHCHECK).
- internal/config: PROTEUS_* env parsing (sizes accept K/M/G/KiB/MiB/GiB).
- internal/safefetch: SSRF-safe HTTP client (dial-time IP check, no proxy, caps).
- internal/loader: per-request proteus.ResourceLoader: data: URLs, assets, remote via safefetch.
- internal/convert: FromProto validation -> Request, engineOptions mapping, Service = bounded slot
  pool (TryAcquire for sync, Acquire for jobs), timeout, panic recovery, output checks.
- internal/rpc: connect server, interceptors (observe: request id/log/metrics; recoverer;
  authenticate: Bearer or x-api-key, health/reflection exempt), conversionService, error mapping.
- internal/jobs: Manager over a Store interface (store.go), MemoryStore (memstore.go),
  storetest conformance suite; workers claim with leases, webhooks HMAC-signed.
- internal/storage/model: storm model Job; internal/storage/store: storm-generated queries
  (committed); internal/storage/pgstore: jobs.Store on Postgres (claim = Take with
  ForUpdateSkipLocked, automigrate via migrate.AutoPool with Schema + DeclaredOnly).
- internal/auth: API keys "id:sha256hex", per-key rate limiter. internal/metrics: Prometheus.

Docker: golang:1.27 build -> distroless static nonroot with fonts; compose.yaml adds postgres:17.
CI: buf lint + generate, storm verify -stale, git diff on generated code, vet, gofmt,
go test -race with a postgres service (PROTEUS_TEST_DATABASE_URL), docker build + buf curl smoke.
