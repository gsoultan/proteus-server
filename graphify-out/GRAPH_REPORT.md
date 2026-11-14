# Graph Report - proteus-server  (2026-10-31)

## Corpus Check
- 56 files · ~73,667 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 5 file(s) not represented in the graph (top: (none) 3, .proto 2)

## Summary
- 1961 nodes · 5180 edges · 89 communities (47 shown, 42 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 104 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `77e4c685`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- conversion.pb.go
- Int64Col
- API
- Pred
- rpc_test.go
- Manager
- Service
- newEnv
- steps
- testing.T
- loader.go
- parser
- conventions_and_gaps.md
- project_overview.md
- task_completion_checklist.md
- github.com/gsoultan/proteus-server
- Query
- InputFormat
- github.com/gsoultan/storm/runtime.Executor
- github.com/gsoultan/storm/runtime.Tok
- Mut
- job.gen.go
- Ins
- GetJobResponse
- time.Time
- proteus
- google.golang.org/protobuf/runtime/protoimpl.MessageState
- Job
- context.Context
- google.golang.org/protobuf/runtime/protoimpl.SizeCache
- OutputFormat
- Pred
- Store
- PageOptions
- Sort
- Query
- NullTextCol
- Row
- Template
- FromEnv
- SubmitJobResponse
- Int32Col
- NullTimeCol
- Record
- MemoryStore
- Options
- file_proteus_v1_conversion_proto_rawDescGZIP
- SubmitTemplateJobRequest
- Caller
- Mut
- NullBytesCol
- ListTemplatesResponse
- UUIDCol
- buf.yaml
- Job
- plugins
- Execute
- Observer
- template.gen.go
- file_proteus_v1_template_proto_rawDescGZIP
- time.Duration
- main.go
- google.golang.org/protobuf/reflect/protoreflect.Message
- UpdateTemplateRequest
- DeleteJobRequest
- Ins
- CreateTemplateRequest
- templates/store.go
- render.go
- SubmitTemplateJobResponse
- run
- RenderTemplateRequest
- ConvertTemplateResponse
- checker
- RenderTemplateResponse
- github.com/gsoultan/storm/runtime.Frag
- github.com/gsoultan/storm/runtime.MaskKey

## God Nodes (most connected - your core abstractions)
1. `Query` - 251 edges
2. `Query` - 159 edges
3. `Pred` - 82 edges
4. `Template` - 49 edges
5. `Mut` - 49 edges
6. `Pred` - 44 edges
7. `Sort` - 37 edges
8. `Ins` - 36 edges
9. `Options` - 31 edges
10. `Job` - 30 edges

## Surprising Connections (you probably didn't know these)
- `Requests` --references--> `Document`  [INFERRED]
  README.md → gen/proteus/v1/conversion.pb.go
- `Adding a format` --references--> `Converter`  [INFERRED]
  README.md → internal/convert/formats.go
- `subcommand()` --calls--> `HashKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `subcommand()` --calls--> `NewKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go

## Import Cycles
- 2-file cycle: `internal/templates/store_test.go -> internal/templates/storetest/storetest.go -> internal/templates/store_test.go`
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (89 total, 42 thin omitted)

### Community 0 - "conversion.pb.go"
Cohesion: 0.13
Nodes (10): BadRequestError, file_proteus_v1_conversion_proto_init(), init(), file_proteus_v1_template_proto_init(), init(), ctxKey, logInfo, Document_Content (+2 more)

### Community 2 - "API"
Cohesion: 0.13
Nodes (10): RegisterConversionServiceHandler(), RegisterTemplateServiceHandler(), WithCaller(), isOps(), New(), requestIDFrom(), serverFault(), API (+2 more)

### Community 3 - "Pred"
Cohesion: 0.05
Nodes (7): Int64Col, NullBytesCol, Pred, Sort, TextCol, TimeCol, UUIDCol

### Community 4 - "rpc_test.go"
Cohesion: 0.10
Nodes (28): ConversionServiceClient, NewConversionServiceClient(), NewTemplateServiceClient(), Sign(), TestSign(), expectCode(), expectPDF(), env (+20 more)

### Community 5 - "Manager"
Cohesion: 0.15
Nodes (4): Manager, publicError(), waitStatus(), nopObserver

### Community 6 - "Service"
Cohesion: 0.12
Nodes (12): warmUp(), Limits, nopObserver, Request, Route, badf(), FromProto(), TestFromProtoRejects() (+4 more)

### Community 7 - "newEnv"
Cohesion: 0.14
Nodes (14): ctxKey, Key, Authenticator, HashKey(), New(), NewKey(), ParseKeys(), redact() (+6 more)

### Community 8 - "steps"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "testing.T"
Cohesion: 0.13
Nodes (24): ConverterFunc, NewRegistry(), NewService(), stubRegistry(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown() (+16 more)

### Community 10 - "loader.go"
Cohesion: 0.10
Nodes (11): Settings, checkWebhookURL(), CleanAssetPath(), decodeDataURL(), Limits, Loader, New(), short() (+3 more)

### Community 18 - "InputFormat"
Cohesion: 0.06
Nodes (5): formatName(), InputFormat, JobStatus, MediaType, PageSize

### Community 20 - "github.com/gsoultan/storm/runtime.Executor"
Cohesion: 0.14
Nodes (10): MetaRow, Row, Insert(), InsertAll(), putBinder(), Scan(), scan(), scanMeta() (+2 more)

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.15
Nodes (14): countStmtFor(), existsStmtFor(), stmtFor(), takeStmtFor(), trimStmtFor(), updAllStmtFor(), updAllStmtIn(), updHead() (+6 more)

### Community 22 - "Mut"
Cohesion: 0.05
Nodes (3): Mutate(), stmtForKey(), Mut

### Community 23 - "job.gen.go"
Cohesion: 0.09
Nodes (23): And(), assignable(), assignAdded(), assignExcluded(), delAllStmtFor(), DeleteOp(), distinctStmtFor(), GetBinder() (+15 more)

### Community 25 - "GetJobResponse"
Cohesion: 0.11
Nodes (3): GetJobRequest, GetJobResponse, UnimplementedConversionServiceHandler

### Community 27 - "proteus"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 28 - "google.golang.org/protobuf/runtime/protoimpl.MessageState"
Cohesion: 0.10
Nodes (4): DownloadJobResultRequest, DownloadJobResultResponse, ListFormatsRequest, ConversionServiceDownloadJobResultServerStream

### Community 30 - "context.Context"
Cohesion: 0.09
Nodes (10): Store, Delete(), LockTimeout(), Delete(), InsertAll(), conversionServiceClient, ConversionServiceDownloadJobResultClientStream, conversionServiceHandler (+2 more)

### Community 31 - "google.golang.org/protobuf/runtime/protoimpl.SizeCache"
Cohesion: 0.08
Nodes (4): SubmitJobRequest, Asset, Document, isDocument_Source

### Community 32 - "OutputFormat"
Cohesion: 0.08
Nodes (3): ConvertRequest, ConvertResponse, OutputFormat

### Community 34 - "Store"
Cohesion: 0.14
Nodes (15): Outcome, Record, fromMeta(), fromRow(), idString(), Migrate(), New(), Open() (+7 more)

### Community 39 - "Row"
Cohesion: 0.15
Nodes (9): MetaRow, Insert(), putBinder(), Scan(), scan(), scanMeta(), binder, Row (+1 more)

### Community 40 - "Template"
Cohesion: 0.12
Nodes (9): bad(), document(), Service, NewService(), ToProto(), Rendered, Settings, RenderTemplateRequest_Template (+1 more)

### Community 45 - "FromEnv"
Cohesion: 0.43
Nodes (7): newLogger(), FromEnv(), Config, env(), TestDefaults(), TestErrors(), TestParse()

### Community 49 - "Record"
Cohesion: 0.12
Nodes (10): fromTemplateMeta(), nameErr(), Create(), New(), clone(), Record, Store, newUUID() (+2 more)

### Community 50 - "MemoryStore"
Cohesion: 0.24
Nodes (3): newUUID(), memJob, MemoryStore

### Community 51 - "Options"
Cohesion: 0.07
Nodes (5): mergeOptions(), TestMergeOptions(), Metadata, Options, PdfOptions

### Community 52 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.10
Nodes (3): file_proteus_v1_conversion_proto_rawDescGZIP(), ListFormatsResponse, Conversion

### Community 54 - "Caller"
Cohesion: 0.06
Nodes (35): Converter, Input, InputFormat, OutputFormat, ProteusEngine, Registry, Caller(), engineOptions() (+27 more)

### Community 55 - "Mut"
Cohesion: 0.11
Nodes (3): MutateKey(), stmtForKey(), Mut

### Community 59 - "ListTemplatesResponse"
Cohesion: 0.10
Nodes (3): ListTemplatesRequest, ListTemplatesResponse, UnimplementedTemplateServiceHandler

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 62 - "Job"
Cohesion: 0.21
Nodes (4): _assertJobShape(), _assertTemplateShape(), Job, Template

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 64 - "Execute"
Cohesion: 0.12
Nodes (8): cleanErr(), Execute(), Parse(), TestEscapeErrorIsTemplateError(), toFloat(), Limits, List, run

### Community 66 - "template.gen.go"
Cohesion: 0.14
Nodes (17): And(), assignable(), assignAdded(), assignExcluded(), GetBinder(), InsertOp(), Inserts(), joinAssign() (+9 more)

### Community 67 - "file_proteus_v1_template_proto_rawDescGZIP"
Cohesion: 0.10
Nodes (3): file_proteus_v1_template_proto_rawDescGZIP(), GetTemplateRequest, GetTemplateResponse

### Community 68 - "time.Duration"
Cohesion: 0.09
Nodes (3): Observer, Metrics, Settings

### Community 69 - "main.go"
Cohesion: 0.10
Nodes (6): h2cClient(), healthcheck(), main(), subcommand(), New(), NewUnit()

### Community 72 - "UpdateTemplateRequest"
Cohesion: 0.10
Nodes (3): UpdateTemplateRequest, UpdateTemplateResponse, TemplateServiceHandler

### Community 73 - "DeleteJobRequest"
Cohesion: 0.11
Nodes (3): DeleteJobRequest, DeleteJobResponse, ConversionServiceHandler

### Community 77 - "templates/store.go"
Cohesion: 0.23
Nodes (8): data_(), render(), TestLimits(), TestRefused(), TestRender(), NewMemoryStore(), TestMemoryStore(), Run()

### Community 78 - "render.go"
Cohesion: 0.12
Nodes (3): limitWriter, Number, TemplateError

### Community 80 - "run"
Cohesion: 0.47
Nodes (9): run(), NewManager(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull(), TestShutdownRequeuesRunningJob(), TestValidationAndWebhook() (+1 more)

### Community 87 - "github.com/gsoultan/storm/runtime.Frag"
Cohesion: 0.60
Nodes (4): FragOf(), fragOf(), FragOf(), fragOf()

### Community 88 - "github.com/gsoultan/storm/runtime.MaskKey"
Cohesion: 0.60
Nodes (3): updAllStmtFor(), updAllStmtIn(), updHead()

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 441 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **42 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `Int64Col`, `.Where`, `github.com/gsoultan/storm/runtime.Executor`, `github.com/gsoultan/storm/runtime.Tok`, `job.gen.go`, `time.Time`, `Pred`, `Store`, `Sort`, `.NotIn`, `.Gt`, `.Gte`, `.Lte`, `.IsNull`, `.Like`, `.IsNullOrEmpty`, `.NotEq`, `.HasPrefix`, `.HasSuffix`?**
  _High betweenness centrality (0.181) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `conversion.pb.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1264849755415793 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Query` to `template.gen.go`, `Pred`, `Row`, `.Where`, `Record`, `github.com/gsoultan/storm/runtime.Tok`, `Mut`, `time.Time`?**
  _High betweenness centrality (0.115) - this node is a cross-community bridge._
- **Should `Int64Col` be split into smaller, more focused modules?**
  _Cohesion score 0.125 - nodes in this community are weakly interconnected._
- **Why does `Mut` connect `Mut` to `Store`, `github.com/gsoultan/storm/runtime.Executor`, `github.com/gsoultan/storm/runtime.Tok`, `job.gen.go`?**
  _High betweenness centrality (0.034) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.1341991341991342 - nodes in this community are weakly interconnected._