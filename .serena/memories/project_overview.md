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
returns filled document), ConvertTemplate (stored id or unsaved template draft), SubmitTemplateJob
(both take output_format), ListStarterTemplates (built-in visual templates, filter by category),
ConvertToCode (visual -> code template, optional revision check). Served by
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
- internal/convert: watermark: request.go validateWatermark (text or image asset, opacity 0-1,
  angle, placement/layer/pages enums), engine.go watermarkOption builds core.Watermark via
  proteus.WithWatermark; image assets resolved through loader.Asset(path).
  formats.go (InputFormat/OutputFormat, target table with MIME type, extension,
  Paged flag and output check; Registry keyed by Route{From, To}; Converter interface getting an
  Input with the fetched source and the per-request loader), engine.go (ProteusEngine registers
  html/markdown -> pdf, engineOptions), request.go (FromProto(doc, output, opts) validation,
  options for other formats refused), service.go (slot pool, timeout, URL fetch, loader, registry
  lookup, Check(req) for unsupported routes, panic recovery, size + target check).
- internal/rpc: connect server, interceptors (observe: request id/log/metrics; recoverer;
  authenticate: Bearer or x-api-key, health/reflection exempt), conversionService, error mapping;
  templates.go = templateService (adds NotFound/AlreadyExists/Aborted/ResourceExhausted for
  template errors, reuses conversionService.convert and jobs.Submit).
- internal/templates: template builder support: fields.go (FieldSpec validation, CheckData ->
  *DataError carrying proto DataProblems, mapped to InvalidArgument + TemplateDataError detail
  via connectproto.NewErrorDetail; ApplyDefaults), format.go (Format{Locale, Currency}: Number,
  Money, Date, Percent and the template funcs money/fnumber/fdate/percent/sum/count/avg/
  add/sub/mul/div/round/nl2br/qr (go-qrcode PNG data URL)), layout.go (Compile(layout, page) ->
  HTML with @page size/margins plus header/footer bands as position:fixed .pt-header/.pt-footer
  at top:-H / bottom:-F, pt-* classes, structured show_if conditions, limits: 500 blocks, depth 4,
  20k text, 20 columns, 50 rows, regions <= 150mm; LayoutError names the block), starters.go +
  starters/*.json (protojson Templates embedded: invoice, quotation, receipt, purchase_order,
  delivery_note, letter, certificate (landscape, QR), report, blank; Starter(id) returns a clone,
  Starters(category) in display order; tests render every one and convert to PDF over RPC).
  Service (validate: name 1-200 runes unique per key, description <= 2000, category <= 50 no
  spaces, fields validated, layout compiled into Content when present,
  convert.FromProto checks of format/assets/options, size cap, parse, trial render with
  sample_data to catch html/template context errors; Render/RenderDraft; mergeOptions lays
  request options over template defaults and drops page/pdf defaults the output can't use),
  render applies field defaults + CheckData, finish() merges options and renders a watermark text
  that contains {{ }} (empty result drops the watermark); layoutOptions drops page size/margins
  for compiled layouts because the @page rule carries them, keeping only the watermark),
  render.go (html/template sandbox, see conventions; Execute/ExecuteWith(Format)), store.go
  (Store interface with List(owner, category, after, limit), MemoryStore), storetest
  conformance suite. Body column = proto-marshalled Template with content, assets, options,
  sample_data, layout, fields, starter_id; Record.Category is its own column.
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

## Web builder and SDK (2026-11)
- web/: React 19 + TypeScript 7 + Mantine 9 + TanStack Router/Query + Zustand + Vite 8 + Bun; PWA via
  vite-plugin-pwa. Built into web/dist (gitignored except .gitkeep, restored by the keep-dist plugin) and
  embedded by web/embed.go (`//go:embed all:dist`, SPA fallback, non-GET -> 404 so TestNoRESTSurface holds).
  Mounted last in internal/rpc/server.go. Dockerfile has an oven/bun stage; CI runs bun check/test/build.
- web/src/gen is produced by buf generate (protoc-gen-es from web/node_modules/.bin); CI diff-checks it.
- Gotchas: TanStack Query hashes keys with JSON.stringify -> never put a protobuf message (bigint fields)
  in a queryKey, serialize with toJsonString first. Vite 8 (rolldown) has no manualChunks object.
  Mantine polymorphic `component={Link}` loses typing; use <Link> or renderRoot.
- templates.Create merges a theme-only layout into the starter (hasBlocks) so the web "start from starter"
  flow keeps the starter blocks.
- Client SDKs live in github.com/gsoultan/proteus-server-sdk (go/ and ts/, proto copied by
  scripts/sync-proto.sh).

