# proteus-server overview

gRPC server (module github.com/gsoultan/proteus-server, go 1.27 because storm v1.11 needs it) for
document conversion. User rule: the design must stay format-agnostic so more output (and input)
formats can be added; today only HTML/Markdown -> PDF exists. Conversions run on github.com/gsoultan/proteus-converter (the engine,
which uses gpdf). The server holds transport, auth, limits, jobs and metrics only; no conversion
logic. DOCX comes later via the engine. User rule: gRPC only, no REST; storm for anything database.

API: proto/proteus/v1/conversion.proto, service proteus.v1.ConversionService with ListFormats,
Convert,
SubmitJob, GetJob, DownloadJobResult (server stream, 1 MiB chunks), DeleteJob; and
proto/proteus/v1/template.proto, service proteus.v1.TemplateService with CreateTemplate,
GetTemplate, ListTemplates (by name, page_token = base64url of last name), UpdateTemplate
(revision = optimistic lock), DeleteTemplate, RenderTemplate (stored id or unsaved template,
returns filled document), ConvertTemplate, SubmitTemplateJob (both take output_format). Served by
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
- internal/convert: formats.go (InputFormat/OutputFormat, target table with MIME type, extension,
  Paged flag and output check; Registry keyed by Route{From, To}; Converter interface getting an
  Input with the fetched source and the per-request loader), engine.go (ProteusEngine registers
  html/markdown -> pdf, engineOptions), request.go (FromProto(doc, output, opts) validation,
  options for other formats refused), service.go (slot pool, timeout, URL fetch, loader, registry
  lookup, Check(req) for unsupported routes, panic recovery, size + target check).
- internal/rpc: connect server, interceptors (observe: request id/log/metrics; recoverer;
  authenticate: Bearer or x-api-key, health/reflection exempt), conversionService, error mapping;
  templates.go = templateService (adds NotFound/AlreadyExists/Aborted/ResourceExhausted for
  template errors, reuses conversionService.convert and jobs.Submit).
- internal/templates: Service (validate: name 1-200 runes unique per key, description <= 2000,
  convert.FromProto checks of format/assets/options, size cap, parse, trial render with
  sample_data to catch html/template context errors; Render/RenderDraft; mergeOptions lays
  request options over template defaults and drops page/pdf defaults the output can't use),
  render.go (html/template sandbox, see conventions), store.go (Store interface, MemoryStore),
  storetest conformance suite. Body column = proto-marshalled Template with only content, assets,
  options, sample_data.
- internal/jobs: Manager over a Store interface (store.go), MemoryStore (memstore.go),
  storetest conformance suite; workers claim with leases, webhooks HMAC-signed.
- internal/storage/model: storm models Job and Template (unique owner+name, Meta projection
  without body); internal/storage/store: storm-generated queries (committed);
  internal/storage/pgstore: jobs.Store on Postgres (claim = Take with ForUpdateSkipLocked,
  automigrate of both models via migrate.AutoPool with Schema + DeclaredOnly) and
  Store.Templates() = templates.Store (update is UpdateAll WHERE revision = read revision).
- internal/auth: API keys "id:sha256hex", per-key rate limiter. internal/metrics: Prometheus.

Docker: golang:1.27 build -> distroless static nonroot with fonts; compose.yaml adds postgres:17.
CI: buf lint + generate, storm verify -stale, git diff on generated code, vet, gofmt,
go test -race with a postgres service (PROTEUS_TEST_DATABASE_URL), docker build + buf curl smoke.
