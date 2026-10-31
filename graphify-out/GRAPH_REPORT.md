# Graph Report - proteus-server  (2026-10-31)

## Corpus Check
- 41 files · ~42,329 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 4 file(s) not represented in the graph (top: (none) 3, .proto 1)

## Summary
- 1169 nodes · 2938 edges · 70 communities (31 shown, 39 thin omitted)
- Extraction: 98% EXTRACTED · 2% INFERRED · 0% AMBIGUOUS · INFERRED: 70 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `1d9b5154`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- proteus-server/main.go
- Client
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
- DeleteJobRequest
- time.Time
- proteus
- google.golang.org/protobuf/runtime/protoimpl.MessageState
- Job
- conversionServiceClient
- Document
- Options
- Pred
- Store
- google.golang.org/protobuf/reflect/protoreflect.Message
- Sort
- NullTextCol
- Record
- context.Context
- Int64Col
- run
- SubmitJobResponse
- Int32Col
- NullTimeCol
- conversion.pb.go
- MemoryStore
- Metadata
- ConvertResponse
- Metrics
- conversionService
- NewManager
- SubmitJobRequest
- NullBytesCol
- UUIDCol
- buf.yaml
- Job
- plugins
- .storm-bootstrap-23224/main.go
- FromProto
- Observer
- NewUnit

## God Nodes (most connected - your core abstractions)
1. `Query` - 223 edges
2. `Pred` - 82 edges
3. `Mut` - 47 edges
4. `Sort` - 37 edges
5. `Ins` - 34 edges
6. `Options` - 31 edges
7. `Job` - 26 edges
8. `Document` - 25 edges
9. `Row` - 25 edges
10. `newEnv()` - 24 edges

## Surprising Connections (you probably didn't know these)
- `Requests` --references--> `Document`  [INFERRED]
  README.md → gen/proteus/v1/conversion.pb.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `ParseKeys()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `NewService()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/convert/service.go
- `run()` --calls--> `NewManager()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/jobs/jobs.go

## Import Cycles
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (70 total, 39 thin omitted)

### Community 1 - "Client"
Cohesion: 0.14
Nodes (7): checkWebhookURL(), decodeDataURL(), short(), Client, IsPublic(), TestIsPublic(), Loader

### Community 2 - "API"
Cohesion: 0.13
Nodes (7): RegisterConversionServiceHandler(), WithCaller(), isOps(), requestIDFrom(), serverFault(), API, healthChecker

### Community 4 - "rpc_test.go"
Cohesion: 0.15
Nodes (25): NewConversionServiceClient(), Sign(), TestSign(), TestMemoryStore(), Run(), expectCode(), expectPDF(), h2c() (+17 more)

### Community 5 - "Manager"
Cohesion: 0.16
Nodes (4): Manager, publicError(), waitStatus(), nopObserver

### Community 6 - "Service"
Cohesion: 0.15
Nodes (8): warmUp(), nopObserver, Request, engineOptions(), TestFromProto(), dedupe(), Result, Service

### Community 7 - "Authenticator"
Cohesion: 0.11
Nodes (16): ctxKey, Key, h2cClient(), healthcheck(), main(), subcommand(), Authenticator, HashKey() (+8 more)

### Community 8 - "steps"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "service_test.go"
Cohesion: 0.46
Nodes (7): NewService(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown(), TestSlots(), TestTimeout()

### Community 10 - "loader.go"
Cohesion: 0.10
Nodes (15): Settings, CleanAssetPath(), Limits, New(), TestCleanAssetPath(), TestLimits(), TestLoad(), TestRemoteGoesThroughSafefetch() (+7 more)

### Community 12 - "proteus-server"
Cohesion: 0.15
Nodes (12): API, Configuration, Development, Errors, Jobs and Postgres, Operations, proteus-server, Quick start (+4 more)

### Community 18 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.07
Nodes (5): file_proteus_v1_conversion_proto_rawDescGZIP(), Format, JobStatus, MediaType, PageSize

### Community 20 - "Row"
Cohesion: 0.17
Nodes (10): MetaRow, Row, Insert(), InsertAll(), putBinder(), Scan(), scan(), scanMeta() (+2 more)

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.13
Nodes (11): countStmtFor(), delAllStmtFor(), existsStmtFor(), metaStmtFor(), stmtFor(), stmtForKey(), takeStmtFor(), trimStmtFor() (+3 more)

### Community 23 - "job.gen.go"
Cohesion: 0.09
Nodes (22): And(), assignable(), assignAdded(), assignExcluded(), DeleteOp(), distinctStmtFor(), FragOf(), fragOf() (+14 more)

### Community 25 - "DeleteJobRequest"
Cohesion: 0.09
Nodes (4): DeleteJobRequest, GetJobRequest, ConversionServiceClient, UnimplementedConversionServiceHandler

### Community 27 - "proteus"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 28 - "google.golang.org/protobuf/runtime/protoimpl.MessageState"
Cohesion: 0.12
Nodes (4): DeleteJobResponse, DownloadJobResultRequest, DownloadJobResultResponse, ConversionServiceDownloadJobResultServerStream

### Community 29 - "Job"
Cohesion: 0.12
Nodes (3): GetJobResponse, ts(), Job

### Community 30 - "conversionServiceClient"
Cohesion: 0.19
Nodes (3): conversionServiceClient, ConversionServiceDownloadJobResultClientStream, conversionServiceHandler

### Community 31 - "Document"
Cohesion: 0.12
Nodes (3): Asset, Document, isDocument_Source

### Community 34 - "Store"
Cohesion: 0.21
Nodes (8): Migrate(), New(), Open(), parseID(), TestStore(), MutateKey(), New(), Store

### Community 39 - "Record"
Cohesion: 0.20
Nodes (7): Record, fromMeta(), fromRow(), idString(), Create(), Settings, Status

### Community 40 - "context.Context"
Cohesion: 0.23
Nodes (4): Outcome, Store, Delete(), LockTimeout()

### Community 45 - "run"
Cohesion: 0.24
Nodes (11): newLogger(), run(), FromEnv(), Config, env(), TestDefaults(), TestErrors(), TestParse() (+3 more)

### Community 49 - "conversion.pb.go"
Cohesion: 0.18
Nodes (4): file_proteus_v1_conversion_proto_init(), init(), Document_Content, Document_Url

### Community 50 - "MemoryStore"
Cohesion: 0.26
Nodes (3): newUUID(), memJob, MemoryStore

### Community 54 - "conversionService"
Cohesion: 0.40
Nodes (4): Caller(), FileName(), ToProto(), conversionService

### Community 55 - "NewManager"
Cohesion: 0.53
Nodes (8): NewManager(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull(), TestShutdownRequeuesRunningJob(), TestValidationAndWebhook(), NewMemoryStore()

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 67 - "FromProto"
Cohesion: 0.50
Nodes (5): Limits, badf(), FromProto(), TestFromProtoRejects(), validateOptions()

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 280 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **39 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `.Contains`, `Pred`, `Store`, `.EqLower`, `Sort`, `.NotEq`, `Int64Col`, `.Gt`, `.Gte`, `.Lte`, `.Where`, `Row`, `github.com/gsoultan/storm/runtime.Tok`, `job.gen.go`, `.IsNull`, `time.Time`, `.push`?**
  _High betweenness centrality (0.238) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `proteus-server/main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.13876040703052728 - nodes in this community are weakly interconnected._
- **Why does `Pred` connect `Pred` to `Query`, `.Where`, `job.gen.go`, `time.Time`, `Sort`, `.NotEq`, `NullTextCol`, `Int64Col`, `.Gt`, `.Gte`, `.Lte`, `Int32Col`, `NullTimeCol`, `.IsNull`, `NullBytesCol`, `.push`, `UUIDCol`, `.Contains`, `.EqLower`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Should `Client` be split into smaller, more focused modules?**
  _Cohesion score 0.13852813852813853 - nodes in this community are weakly interconnected._
- **Why does `Mut` connect `Mut` to `Store`, `Row`, `github.com/gsoultan/storm/runtime.Tok`, `job.gen.go`?**
  _High betweenness centrality (0.052) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.12857142857142856 - nodes in this community are weakly interconnected._