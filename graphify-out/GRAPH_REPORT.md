# Graph Report - proteus-server  (2026-11-01)

## Corpus Check
- 45 files · ~46,671 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 4 file(s) not represented in the graph (top: (none) 3, .proto 1)

## Summary
- 1296 nodes · 3279 edges · 66 communities (34 shown, 32 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 80 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `dc588ba9`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- jobs.go
- Int64Col
- API
- Client
- rpc_test.go
- Manager
- Service
- auth_test.go
- steps
- service_test.go
- loader.go
- time.Duration
- proteus-server
- conventions_and_gaps.md
- project_overview.md
- task_completion_checklist.md
- github.com/gsoultan/proteus-server
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
- context.Context
- Document
- ConvertResponse
- Pred
- PageOptions
- Sort
- ToProto
- .Op
- .deliver
- Query
- proteus-server/main.go
- SubmitJobRequest
- Int32Col
- time.Time
- conversion.pb.go
- Observer
- Options
- google.golang.org/protobuf/reflect/protoreflect.Message
- putBinder
- NullBytesCol
- buf.yaml
- Job
- plugins
- ProteusEngine
- FromProto

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
- `subcommand()` --calls--> `HashKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `ParseKeys()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go

## Import Cycles
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (66 total, 32 thin omitted)

### Community 0 - "jobs.go"
Cohesion: 0.11
Nodes (4): ctxKey, NewUnit(), ctxKey, logInfo

### Community 2 - "API"
Cohesion: 0.06
Nodes (11): RegisterConversionServiceHandler(), Authenticator, WithCaller(), Metrics, isOps(), New(), requestIDFrom(), serverFault() (+3 more)

### Community 3 - "Client"
Cohesion: 0.15
Nodes (7): checkWebhookURL(), decodeDataURL(), Loader, short(), Client, IsPublic(), TestIsPublic()

### Community 4 - "rpc_test.go"
Cohesion: 0.16
Nodes (27): ConversionServiceClient, NewConversionServiceClient(), Sign(), TestSign(), TestMemoryStore(), Run(), expectCode(), expectPDF() (+19 more)

### Community 5 - "Manager"
Cohesion: 0.23
Nodes (10): Manager, NewManager(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull(), TestShutdownRequeuesRunningJob(), TestValidationAndWebhook() (+2 more)

### Community 7 - "auth_test.go"
Cohesion: 0.36
Nodes (9): Key, HashKey(), New(), ParseKeys(), redact(), TestDisabled(), TestKeys(), TestParseKeysErrors() (+1 more)

### Community 8 - "steps"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "service_test.go"
Cohesion: 0.31
Nodes (11): ConverterFunc, NewRegistry(), NewService(), stubRegistry(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown() (+3 more)

### Community 10 - "loader.go"
Cohesion: 0.09
Nodes (16): BadRequestError, Settings, CleanAssetPath(), Limits, New(), TestCleanAssetPath(), TestLimits(), TestLoad() (+8 more)

### Community 11 - "time.Duration"
Cohesion: 0.11
Nodes (4): parser, Observer, LockTimeout(), Settings

### Community 12 - "proteus-server"
Cohesion: 0.14
Nodes (13): Adding a format, API, Configuration, Development, Errors, Jobs and Postgres, Operations, proteus-server (+5 more)

### Community 18 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.06
Nodes (5): file_proteus_v1_conversion_proto_rawDescGZIP(), InputFormat, JobStatus, MediaType, PageSize

### Community 20 - "Row"
Cohesion: 0.17
Nodes (8): MetaRow, Row, Insert(), InsertAll(), Scan(), scan(), scanMeta(), rowSource

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.17
Nodes (12): countStmtFor(), delAllStmtFor(), distinctStmtFor(), existsStmtFor(), metaStmtFor(), stmtFor(), stmtForKey(), takeStmtFor() (+4 more)

### Community 23 - "job.gen.go"
Cohesion: 0.10
Nodes (19): And(), assignable(), assignAdded(), assignExcluded(), Delete(), FragOf(), fragOf(), GetBinder() (+11 more)

### Community 25 - "GetJobResponse"
Cohesion: 0.10
Nodes (3): GetJobRequest, GetJobResponse, ConversionServiceHandler

### Community 27 - "proteus"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 28 - "google.golang.org/protobuf/runtime/protoimpl.MessageState"
Cohesion: 0.10
Nodes (4): DeleteJobRequest, DeleteJobResponse, DownloadJobResultRequest, DownloadJobResultResponse

### Community 30 - "context.Context"
Cohesion: 0.06
Nodes (25): ConversionServiceDownloadJobResultServerStream, Caller(), newUUID(), Outcome, Record, Store, fromMeta(), fromRow() (+17 more)

### Community 31 - "Document"
Cohesion: 0.11
Nodes (3): Asset, Document, isDocument_Source

### Community 32 - "ConvertResponse"
Cohesion: 0.08
Nodes (3): ConvertRequest, ConvertResponse, OutputFormat

### Community 33 - "Pred"
Cohesion: 0.10
Nodes (3): NullTextCol, Pred, TextCol

### Community 36 - "Sort"
Cohesion: 0.11
Nodes (3): NullTextArrayCol, Sort, UUIDCol

### Community 37 - "ToProto"
Cohesion: 0.19
Nodes (11): Converter, Input, InputFormat, OutputFormat, Registry, Route, Target, InputToProto() (+3 more)

### Community 38 - ".Op"
Cohesion: 0.29
Nodes (3): DeleteOp(), InsertOp(), stmtForInsertNoReturn()

### Community 45 - "proteus-server/main.go"
Cohesion: 0.13
Nodes (15): h2cClient(), healthcheck(), main(), newLogger(), run(), subcommand(), warmUp(), NewKey() (+7 more)

### Community 46 - "SubmitJobRequest"
Cohesion: 0.09
Nodes (3): SubmitJobRequest, SubmitJobResponse, UnimplementedConversionServiceHandler

### Community 49 - "conversion.pb.go"
Cohesion: 0.18
Nodes (4): file_proteus_v1_conversion_proto_init(), init(), Document_Content, Document_Url

### Community 51 - "Options"
Cohesion: 0.07
Nodes (3): Metadata, Options, PdfOptions

### Community 52 - "google.golang.org/protobuf/reflect/protoreflect.Message"
Cohesion: 0.08
Nodes (3): ListFormatsRequest, ListFormatsResponse, Conversion

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 67 - "ProteusEngine"
Cohesion: 0.40
Nodes (3): ProteusEngine, engineOptions(), TestFromProto()

### Community 70 - "FromProto"
Cohesion: 0.17
Nodes (10): Limits, Request, badf(), FileName(), FromProto(), TestFileName(), TestFromProtoRejects(), validateOptions() (+2 more)

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 310 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **32 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `Int64Col`, `.HasPrefix`, `.Where`, `Row`, `github.com/gsoultan/storm/runtime.Tok`, `job.gen.go`, `.Lt`, `context.Context`, `Pred`, `.IsNotNull`, `Sort`, `.Op`, `.HasSuffix`, `.NotIn`, `.Gt`, `.Gte`, `time.Time`, `putBinder`, `.ILike`, `.IsNull`, `.Like`, `.push`, `.Contains`?**
  _High betweenness centrality (0.218) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `jobs.go` be split into smaller, more focused modules?**
  _Cohesion score 0.10708898944193061 - nodes in this community are weakly interconnected._
- **Why does `Pred` connect `Pred` to `Int64Col`, `.HasPrefix`, `.Where`, `job.gen.go`, `.Lt`, `.IsNotNull`, `Sort`, `.HasSuffix`, `.NotIn`, `.Gt`, `.Gte`, `Query`, `Int32Col`, `time.Time`, `.ILike`, `.IsNull`, `.Like`, `NullBytesCol`, `.push`, `.Contains`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._
- **Should `Int64Col` be split into smaller, more focused modules?**
  _Cohesion score 0.125 - nodes in this community are weakly interconnected._
- **Why does `Ins` connect `Ins` to `.Op`, `time.Time`, `Row`, `job.gen.go`, `context.Context`?**
  _High betweenness centrality (0.049) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.06387921022067364 - nodes in this community are weakly interconnected._