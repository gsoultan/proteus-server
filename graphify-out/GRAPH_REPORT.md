# Graph Report - proteus-server  (2026-11-06)

## Corpus Check
- 71 files · ~126,875 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 6 file(s) not represented in the graph (top: (none) 3, .proto 3)

## Summary
- 3352 nodes · 8366 edges · 129 communities (54 shown, 75 thin omitted)
- Extraction: 95% EXTRACTED · 5% INFERRED · 0% AMBIGUOUS · INFERRED: 394 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `3635b2ce`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- conversion.pb.go
- API
- Pred
- testing.T
- Manager
- Service
- auth.go
- steps
- service_test.go
- safefetch.go
- parser
- conventions_and_gaps.md
- project_overview.md
- task_completion_checklist.md
- github.com/gsoultan/proteus-server
- Query
- InputFormat
- job.gen.go
- github.com/gsoultan/storm/runtime.Tok
- Mut
- webhookendpoint.gen.go
- Ins
- google.golang.org/protobuf/reflect/protoreflect.Message
- proteus
- google.golang.org/protobuf/runtime/protoimpl.UnknownFields
- WebhookEndpoint
- context.Context
- Query
- Query
- github.com/gsoultan/storm/runtime.Executor
- Store
- PageOptions
- Pred
- Query
- template.gen.go
- Template
- WebhookDelivery
- Mut
- Delivery
- WebhookServiceHandler
- main.go
- Caller
- Mut
- UpdateWebhookEndpointRequest
- Record
- MemoryStore
- Options
- Endpoint
- SubmitTemplateJobRequest
- WebhookEvent
- Ins
- Ins
- Sort
- ListTemplatesResponse
- CreateWebhookEndpointRequest
- buf.yaml
- shape.gen.go
- plugins
- run
- Pred
- Sort
- file_proteus_v1_template_proto_rawDescGZIP
- Metrics
- go_pkg_context
- DeleteTemplateRequest
- secret.go
- UpdateTemplateRequest
- Pred
- Ins
- RevealWebhookSecretResponse
- CreateTemplateRequest
- RotateWebhookSecretResponse
- render.go
- SubmitTemplateJobResponse
- TextCol
- RenderTemplateRequest
- ConvertTemplateResponse
- checker
- RenderTemplateResponse
- webhookdelivery.gen.go
- GetWebhookEndpointResponse
- RedeliverWebhookDeliveryResponse
- SendTestWebhookEventResponse
- WebhookServiceClient
- NullUUIDCol
- Registry
- Service
- Int32Col
- Int64Col
- NullTimeCol
- Int32Col
- NullTimeCol
- ToProto
- templateService
- TimeCol
- TimeCol
- FromProto
- UUIDCol
- NullBytesCol
- UUIDCol
- .Check
- binder

## God Nodes (most connected - your core abstractions)
1. `Query` - 251 edges
2. `Query` - 224 edges
3. `Query` - 173 edges
4. `Query` - 159 edges
5. `Pred` - 85 edges
6. `Pred` - 82 edges
7. `Pred` - 57 edges
8. `Template` - 50 edges
9. `Mut` - 49 edges
10. `Mut` - 49 edges

## Surprising Connections (you probably didn't know these)
- `Requests` --references--> `Document`  [INFERRED]
  README.md → gen/proteus/v1/conversion.pb.go
- `Adding a format` --references--> `Converter`  [INFERRED]
  README.md → internal/convert/formats.go
- `Webhooks` --references--> `WebhookEvent`  [INFERRED]
  README.md → gen/proteus/v1/webhook.pb.go
- `subcommand()` --calls--> `HashKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `subcommand()` --calls--> `NewKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go

## Import Cycles
- 2-file cycle: `internal/webhooks/storetest/storetest.go -> internal/webhooks/webhooks_test.go -> internal/webhooks/storetest/storetest.go`
- 2-file cycle: `internal/templates/store_test.go -> internal/templates/storetest/storetest.go -> internal/templates/store_test.go`
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (129 total, 75 thin omitted)

### Community 0 - "conversion.pb.go"
Cohesion: 0.11
Nodes (12): BadRequestError, file_proteus_v1_conversion_proto_init(), init(), file_proteus_v1_template_proto_init(), init(), file_proteus_v1_webhook_proto_init(), init(), ctxKey (+4 more)

### Community 2 - "API"
Cohesion: 0.13
Nodes (11): RegisterConversionServiceHandler(), RegisterTemplateServiceHandler(), RegisterWebhookServiceHandler(), Authenticator, WithCaller(), isOps(), New(), requestIDFrom() (+3 more)

### Community 3 - "Pred"
Cohesion: 0.06
Nodes (8): WhenSet(), Int64Col, NullBytesCol, Pred, Sort, TextCol, TimeCol, UUIDCol

### Community 4 - "testing.T"
Cohesion: 0.12
Nodes (37): Sign(), TestSign(), TestMemoryStore(), Run(), expectCode(), expectPDF(), html(), md() (+29 more)

### Community 5 - "Manager"
Cohesion: 0.17
Nodes (12): Manager, NewManager(), publicError(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull(), TestShutdownRequeuesRunningJob() (+4 more)

### Community 6 - "Service"
Cohesion: 0.14
Nodes (6): nopObserver, Request, Route, dedupe(), Result, Service

### Community 7 - "auth.go"
Cohesion: 0.25
Nodes (11): ctxKey, Key, HashKey(), New(), NewKey(), ParseKeys(), redact(), TestDisabled() (+3 more)

### Community 8 - "steps"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "service_test.go"
Cohesion: 0.23
Nodes (12): ConverterFunc, Settings, NewRegistry(), NewService(), stubRegistry(), stubService(), TestOutputChecks(), TestPanicBecomesError() (+4 more)

### Community 10 - "safefetch.go"
Cohesion: 0.07
Nodes (23): checkWebhookURL(), CleanAssetPath(), decodeDataURL(), Limits, Loader, New(), short(), TestCleanAssetPath() (+15 more)

### Community 18 - "InputFormat"
Cohesion: 0.06
Nodes (5): formatName(), InputFormat, JobStatus, MediaType, PageSize

### Community 20 - "job.gen.go"
Cohesion: 0.07
Nodes (31): And(), assignable(), assignAdded(), assignExcluded(), delAllStmtFor(), Delete(), distinctStmtFor(), GetBinder() (+23 more)

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.09
Nodes (28): countStmtFor(), existsStmtFor(), metaStmtFor(), takeStmtFor(), updAllStmtFor(), updAllStmtIn(), updHead(), countStmtFor() (+20 more)

### Community 23 - "webhookendpoint.gen.go"
Cohesion: 0.06
Nodes (22): DeleteOp(), DeleteOp(), DeleteOp(), And(), assignable(), assignAdded(), assignExcluded(), Delete() (+14 more)

### Community 25 - "google.golang.org/protobuf/reflect/protoreflect.Message"
Cohesion: 0.03
Nodes (12): ConvertResponse, DeleteJobRequest, DeleteJobResponse, DownloadJobResultResponse, GetJobRequest, GetJobResponse, ListFormatsRequest, ListFormatsResponse (+4 more)

### Community 27 - "proteus"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 28 - "google.golang.org/protobuf/runtime/protoimpl.UnknownFields"
Cohesion: 0.14
Nodes (3): DownloadJobResultRequest, EnableWebhookEndpointRequest, EnableWebhookEndpointResponse

### Community 29 - "WebhookEndpoint"
Cohesion: 0.06
Nodes (7): TestWebhookRetriesAndAutoPause(), DeliveryToProto(), EndpointToProto(), EventType(), ts(), Job, WebhookEndpoint

### Community 30 - "context.Context"
Cohesion: 0.10
Nodes (7): ConversionServiceDownloadJobResultServerStream, conversionServiceClient, ConversionServiceDownloadJobResultClientStream, conversionServiceHandler, templateServiceClient, templateServiceHandler, webhookServiceHandler

### Community 33 - "github.com/gsoultan/storm/runtime.Executor"
Cohesion: 0.07
Nodes (20): delAllStmtFor(), distinctStmtFor(), MetaRow, Row, Insert(), InsertAll(), metaStmtFor(), Mutate() (+12 more)

### Community 34 - "Store"
Cohesion: 0.09
Nodes (15): Outcome, Record, Store, fromMeta(), fromRow(), Migrate(), New(), Open() (+7 more)

### Community 36 - "Pred"
Cohesion: 0.03
Nodes (12): WhenSet(), Int32Col, Int64Col, NullBytesCol, NullTextArrayCol, NullTextCol, NullTimeCol, Pred (+4 more)

### Community 39 - "template.gen.go"
Cohesion: 0.05
Nodes (34): And(), assignable(), assignAdded(), assignExcluded(), delAllStmtFor(), Delete(), distinctStmtFor(), GetBinder() (+26 more)

### Community 40 - "Template"
Cohesion: 0.11
Nodes (9): bad(), document(), Service, NewService(), ToProto(), Rendered, Settings, RenderTemplateRequest_Template (+1 more)

### Community 41 - "WebhookDelivery"
Cohesion: 0.05
Nodes (7): file_proteus_v1_webhook_proto_rawDescGZIP(), ListWebhookDeliveriesRequest, ListWebhookDeliveriesResponse, decodeCursor(), encodeCursor(), WebhookDelivery, WebhookDeliveryStatus

### Community 42 - "Mut"
Cohesion: 0.07
Nodes (6): fromEndpoint(), Mut, MutateKey(), New(), Result, Webhooks

### Community 43 - "Delivery"
Cohesion: 0.09
Nodes (15): Observer, idString(), fromDelivery(), fromDeliveryMeta(), nullID(), LockTimeout(), Create(), New() (+7 more)

### Community 44 - "WebhookServiceHandler"
Cohesion: 0.08
Nodes (7): WebhookServiceHandler, DisableWebhookEndpointRequest, DisableWebhookEndpointResponse, ListWebhookEndpointsRequest, ListWebhookEndpointsResponse, UnimplementedWebhookServiceHandler, webhookServiceClient

### Community 45 - "main.go"
Cohesion: 0.14
Nodes (14): h2cClient(), healthcheck(), main(), newLogger(), run(), subcommand(), warmUp(), FromEnv() (+6 more)

### Community 46 - "Caller"
Cohesion: 0.13
Nodes (5): DeleteWebhookEndpointRequest, DeleteWebhookEndpointResponse, Caller(), conversionService, webhookService

### Community 48 - "UpdateWebhookEndpointRequest"
Cohesion: 0.09
Nodes (7): UpdateWebhookEndpointRequest, UpdateWebhookEndpointResponse, badRequest(), checkDescription(), checkEvents(), EventName(), WebhookEventType

### Community 49 - "Record"
Cohesion: 0.12
Nodes (10): fromTemplateMeta(), nameErr(), Create(), New(), clone(), Record, Store, newUUID() (+2 more)

### Community 50 - "MemoryStore"
Cohesion: 0.24
Nodes (3): newUUID(), memJob, MemoryStore

### Community 51 - "Options"
Cohesion: 0.03
Nodes (11): file_proteus_v1_conversion_proto_rawDescGZIP(), ConvertRequest, SubmitJobRequest, Asset, Conversion, Document, isDocument_Source, Metadata (+3 more)

### Community 52 - "Endpoint"
Cohesion: 0.13
Nodes (5): copyEndpoint(), Endpoint, Store, Run(), MemoryStore

### Community 54 - "WebhookEvent"
Cohesion: 0.06
Nodes (18): Adding a format, API, Configuration, Development, Errors, Jobs, templates and Postgres, Operations, proteus-server (+10 more)

### Community 58 - "Sort"
Cohesion: 0.11
Nodes (3): BoolCol, Sort, TextArrayCol

### Community 59 - "ListTemplatesResponse"
Cohesion: 0.10
Nodes (3): TemplateServiceClient, ListTemplatesRequest, ListTemplatesResponse

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 62 - "shape.gen.go"
Cohesion: 0.13
Nodes (8): _assertJobShape(), _assertTemplateShape(), _assertWebhookDeliveryShape(), _assertWebhookEndpointShape(), Job, Template, WebhookDelivery, WebhookEndpoint

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 64 - "run"
Cohesion: 0.16
Nodes (3): toFloat(), List, run

### Community 66 - "Sort"
Cohesion: 0.12
Nodes (3): BoolCol, NullBytesCol, Sort

### Community 67 - "file_proteus_v1_template_proto_rawDescGZIP"
Cohesion: 0.10
Nodes (3): file_proteus_v1_template_proto_rawDescGZIP(), GetTemplateRequest, GetTemplateResponse

### Community 68 - "Metrics"
Cohesion: 0.10
Nodes (4): Metrics, nopObserver, Observer, nopObserver

### Community 69 - "go_pkg_context"
Cohesion: 0.14
Nodes (4): NewUnit(), NewMemoryStore(), TestMemoryStore(), Run()

### Community 70 - "DeleteTemplateRequest"
Cohesion: 0.11
Nodes (3): DeleteTemplateRequest, DeleteTemplateResponse, UnimplementedTemplateServiceHandler

### Community 71 - "secret.go"
Cohesion: 0.11
Nodes (7): NewSealer(), NewSecret(), New(), TestSealer(), Observer, Sealer, Settings

### Community 78 - "render.go"
Cohesion: 0.09
Nodes (13): cleanErr(), Execute(), Parse(), data_(), render(), TestEscapeErrorIsTemplateError(), TestLimits(), TestRefused() (+5 more)

### Community 87 - "webhookdelivery.gen.go"
Cohesion: 0.05
Nodes (27): FragOf(), fragOf(), Lowering(), FragOf(), fragOf(), Lowering(), And(), assignable() (+19 more)

### Community 91 - "WebhookServiceClient"
Cohesion: 0.18
Nodes (9): NewConversionServiceClient(), NewTemplateServiceClient(), WebhookServiceClient, NewWebhookServiceClient(), env, h2c(), withKey(), env (+1 more)

### Community 93 - "Registry"
Cohesion: 0.22
Nodes (6): Converter, Input, ProteusEngine, Registry, engineOptions(), TestFromProto()

### Community 100 - "ToProto"
Cohesion: 0.30
Nodes (11): InputFormat, OutputFormat, Target, InputToProto(), OutputFromProto(), OutputToProto(), TargetOf(), ToProto() (+3 more)

### Community 104 - "FromProto"
Cohesion: 0.22
Nodes (7): Limits, badf(), FileName(), FromProto(), TestFileName(), TestFromProtoRejects(), validateOptions()

## Knowledge Gaps
- **2 isolated node(s):** `env`, `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 945 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **75 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `github.com/gsoultan/storm/runtime.Executor`, `Pred`, `time.Time`, `Delivery`, `github.com/gsoultan/storm/runtime.Tok`, `webhookdelivery.gen.go`, `Binder`, `binder`, `NullUUIDCol`?**
  _High betweenness centrality (0.092) - this node is a cross-community bridge._
- **What connects `env`, `github.com/gsoultan/proteus-server` to the rest of the system?**
  _2 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `conversion.pb.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10650281618023553 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Query` to `Pred`, `time.Time`, `template.gen.go`, `.Gte`, `.Lt`, `.Where`, `.Lte`, `Record`, `.NotIn`, `.push`, `.Contains`, `github.com/gsoultan/storm/runtime.Tok`, `.EqLower`, `.Like`?**
  _High betweenness centrality (0.074) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.12666666666666668 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Query` to `.In`, `Store`, `.IsNull`, `Pred`, `time.Time`, `.Contains`, `.ILike`, `.push`, `.Where`, `job.gen.go`, `github.com/gsoultan/storm/runtime.Tok`, `Mut`, `.HasPrefix`, `.HasSuffix`, `.Lt`?**
  _High betweenness centrality (0.071) - this node is a cross-community bridge._
- **Should `Pred` be split into smaller, more focused modules?**
  _Cohesion score 0.056866303690260134 - nodes in this community are weakly interconnected._