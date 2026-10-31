# Graph Report - proteus-server  (2026-10-31)

## Corpus Check
- 43 files · ~46,187 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 4 file(s) not represented in the graph (top: (none) 3, .proto 1)

## Summary
- 1292 nodes · 3273 edges · 72 communities (34 shown, 38 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 80 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `6467167e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- proteus-server/main.go
- Int64Col
- API
- BadRequestError
- rpc_test.go
- Manager
- Service
- Authenticator
- steps
- service_test.go
- loader.go
- parser
- proteus-server
- conventions_and_gaps.md
- project_overview.md
- task_completion_checklist.md
- github.com/gsoultan/proteus-server
- Query
- file_proteus_v1_conversion_proto_rawDescGZIP
- Row
- github.com/gsoultan/storm/runtime.Tok
- Mut
- job.gen.go
- Ins
- GetJobResponse
- proteus
- google.golang.org/protobuf/runtime/protoimpl.MessageState
- Job
- conversionServiceClient
- Document
- ConvertResponse
- Pred
- Store
- google.golang.org/protobuf/reflect/protoreflect.Message
- Sort
- Route
- NullTextCol
- Record
- context.Context
- time.Time
- run
- SubmitJobRequest
- Int32Col
- NullTimeCol
- conversion.pb.go
- MemoryStore
- Options
- file_proteus_v1_conversion_proto_rawDescGZIP
- putBinder
- conversionService
- NullBytesCol
- UUIDCol
- buf.yaml
- Job
- plugins
- .storm-bootstrap-23224/main.go
- FromProto
- Observer
- store.gen.go
- FileName

## God Nodes (most connected - your core abstractions)
1. `Query` - 251 edges
2. `Pred` - 82 edges
3. `Mut` - 49 edges
4. `Sort` - 37 edges
5. `Ins` - 36 edges
6. `Job` - 30 edges
7. `Options` - 28 edges
8. `file_proteus_v1_conversion_proto_rawDescGZIP()` - 27 edges
9. `Document` - 25 edges
10. `newEnv()` - 25 edges

## Surprising Connections (you probably didn't know these)
- `Requests` --references--> `Document`  [INFERRED]
  README.md → gen/proteus/v1/conversion.pb.go
- `Adding a format` --references--> `Converter`  [INFERRED]
  README.md → internal/convert/formats.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `ParseKeys()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `NewService()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/convert/service.go

## Import Cycles
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (72 total, 38 thin omitted)

### Community 2 - "API"
Cohesion: 0.06
Nodes (14): RegisterConversionServiceHandler(), Authenticator, WithCaller(), decodeDataURL(), Loader, short(), Metrics, isOps() (+6 more)

### Community 4 - "rpc_test.go"
Cohesion: 0.17
Nodes (25): ConversionServiceClient, NewConversionServiceClient(), Sign(), TestSign(), expectCode(), expectPDF(), h2c(), html() (+17 more)

### Community 5 - "Manager"
Cohesion: 0.12
Nodes (16): checkWebhookURL(), Manager, NewManager(), publicError(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull() (+8 more)

### Community 6 - "Service"
Cohesion: 0.18
Nodes (6): warmUp(), nopObserver, Request, dedupe(), Result, Service

### Community 7 - "Authenticator"
Cohesion: 0.13
Nodes (15): ctxKey, Key, h2cClient(), healthcheck(), main(), subcommand(), HashKey(), New() (+7 more)

### Community 8 - "steps"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "service_test.go"
Cohesion: 0.31
Nodes (11): ConverterFunc, NewRegistry(), NewService(), stubRegistry(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown() (+3 more)

### Community 10 - "loader.go"
Cohesion: 0.10
Nodes (18): Settings, CleanAssetPath(), Limits, New(), TestCleanAssetPath(), TestLimits(), TestLoad(), TestRemoteGoesThroughSafefetch() (+10 more)

### Community 12 - "proteus-server"
Cohesion: 0.14
Nodes (13): Adding a format, API, Configuration, Development, Errors, Jobs and Postgres, Operations, proteus-server (+5 more)

### Community 18 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.08
Nodes (3): JobStatus, MediaType, PageSize

### Community 20 - "Row"
Cohesion: 0.19
Nodes (8): MetaRow, Row, Insert(), InsertAll(), Scan(), scan(), scanMeta(), rowSource

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.18
Nodes (11): countStmtFor(), delAllStmtFor(), distinctStmtFor(), existsStmtFor(), metaStmtFor(), stmtFor(), takeStmtFor(), trimStmtFor() (+3 more)

### Community 22 - "Mut"
Cohesion: 0.05
Nodes (3): DeleteOp(), stmtForKey(), Mut

### Community 23 - "job.gen.go"
Cohesion: 0.11
Nodes (19): And(), assignable(), assignAdded(), assignExcluded(), FragOf(), fragOf(), GetBinder(), InsertOp() (+11 more)

### Community 25 - "GetJobResponse"
Cohesion: 0.10
Nodes (3): GetJobRequest, GetJobResponse, ConversionServiceHandler

### Community 27 - "proteus"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 28 - "google.golang.org/protobuf/runtime/protoimpl.MessageState"
Cohesion: 0.07
Nodes (6): DeleteJobRequest, DeleteJobResponse, DownloadJobResultResponse, ListFormatsRequest, ListFormatsResponse, UnimplementedConversionServiceHandler

### Community 30 - "conversionServiceClient"
Cohesion: 0.11
Nodes (5): DownloadJobResultRequest, ConversionServiceDownloadJobResultServerStream, conversionServiceClient, ConversionServiceDownloadJobResultClientStream, conversionServiceHandler

### Community 31 - "Document"
Cohesion: 0.10
Nodes (3): Asset, Document, isDocument_Source

### Community 32 - "ConvertResponse"
Cohesion: 0.11
Nodes (3): ConvertRequest, ConvertResponse, OutputFormat

### Community 33 - "Pred"
Cohesion: 0.18
Nodes (3): WhenSet(), Pred, TextCol

### Community 34 - "Store"
Cohesion: 0.24
Nodes (7): Migrate(), New(), Open(), parseID(), TestStore(), New(), Store

### Community 37 - "Route"
Cohesion: 0.24
Nodes (4): Converter, Input, Registry, Route

### Community 39 - "Record"
Cohesion: 0.24
Nodes (6): Record, fromMeta(), fromRow(), idString(), Create(), Status

### Community 40 - "context.Context"
Cohesion: 0.20
Nodes (5): Outcome, Store, Delete(), MutateKey(), LockTimeout()

### Community 45 - "run"
Cohesion: 0.17
Nodes (9): newLogger(), run(), FromEnv(), Config, env(), TestDefaults(), TestErrors(), TestParse() (+1 more)

### Community 49 - "conversion.pb.go"
Cohesion: 0.18
Nodes (4): file_proteus_v1_conversion_proto_init(), init(), Document_Content, Document_Url

### Community 50 - "MemoryStore"
Cohesion: 0.26
Nodes (3): newUUID(), memJob, MemoryStore

### Community 51 - "Options"
Cohesion: 0.07
Nodes (3): Metadata, Options, PdfOptions

### Community 52 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.10
Nodes (3): file_proteus_v1_conversion_proto_rawDescGZIP(), Conversion, InputFormat

### Community 54 - "conversionService"
Cohesion: 0.27
Nodes (9): InputFormat, OutputFormat, Target, Caller(), InputToProto(), OutputToProto(), TargetOf(), ToProto() (+1 more)

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 67 - "FromProto"
Cohesion: 0.22
Nodes (8): Limits, ProteusEngine, engineOptions(), badf(), FromProto(), TestFromProto(), TestFromProtoRejects(), validateOptions()

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 309 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **38 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `Int64Col`, `.Where`, `Row`, `github.com/gsoultan/storm/runtime.Tok`, `Mut`, `job.gen.go`, `time.Time`, `Pred`, `Store`, `Sort`, `Int64Col`, `.Gt`, `time.Time`, `.Lte`, `putBinder`, `.ILike`, `.IsNull`, `.Like`, `.push`, `.Contains`, `.EqLower`, `.IsNullOrEmpty`?**
  _High betweenness centrality (0.214) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `proteus-server/main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.12218045112781954 - nodes in this community are weakly interconnected._
- **Why does `Pred` connect `Pred` to `Int64Col`, `Query`, `.Where`, `job.gen.go`, `time.Time`, `Sort`, `NullTextCol`, `Int64Col`, `.Gt`, `time.Time`, `.Lte`, `Int32Col`, `NullTimeCol`, `.ILike`, `.IsNull`, `.Like`, `NullBytesCol`, `.push`, `UUIDCol`, `.Contains`, `.EqLower`, `.IsNullOrEmpty`?**
  _High betweenness centrality (0.078) - this node is a cross-community bridge._
- **Should `Int64Col` be split into smaller, more focused modules?**
  _Cohesion score 0.125 - nodes in this community are weakly interconnected._
- **Why does `Ins` connect `Ins` to `Row`, `Record`, `job.gen.go`?**
  _High betweenness centrality (0.051) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.05782312925170068 - nodes in this community are weakly interconnected._