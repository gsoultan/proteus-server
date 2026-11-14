# Graph Report - proteus-server  (2026-11-14)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 2135 nodes · 5656 edges · 95 communities (56 shown, 39 thin omitted)
- Extraction: 97% EXTRACTED · 3% INFERRED · 0% AMBIGUOUS · INFERRED: 166 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `5a0ebe16`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- Community 0
- Community 1
- Community 2
- Community 3
- Community 4
- Community 5
- Community 6
- Community 7
- Community 8
- Community 9
- Community 10
- Community 11
- Community 13
- Community 14
- Community 15
- Community 16
- Community 17
- Community 18
- Community 20
- Community 21
- Community 22
- Community 23
- Community 24
- Community 25
- Community 26
- Community 27
- Community 28
- Community 29
- Community 30
- Community 31
- Community 32
- Community 33
- Community 34
- Community 35
- Community 36
- Community 37
- Community 38
- Community 39
- Community 40
- Community 41
- Community 42
- Community 43
- Community 44
- Community 45
- Community 46
- Community 47
- Community 48
- Community 49
- Community 50
- Community 51
- Community 52
- Community 53
- Community 54
- Community 55
- Community 56
- Community 57
- Community 58
- Community 59
- Community 60
- Community 61
- Community 62
- Community 63
- Community 64
- Community 65
- Community 66
- Community 67
- Community 68
- Community 69
- Community 70
- Community 71
- Community 72
- Community 73
- Community 74
- Community 78
- Community 79
- Community 80
- Community 81
- Community 83
- Community 84
- Community 89
- Community 90
- Community 91
- Community 94

## God Nodes (most connected - your core abstractions)
1. `Query` - 251 edges
2. `Query` - 159 edges
3. `Pred` - 82 edges
4. `Mut` - 49 edges
5. `Template` - 49 edges
6. `Pred` - 44 edges
7. `Sort` - 37 edges
8. `Ins` - 36 edges
9. `newEnv()` - 32 edges
10. `Options` - 31 edges

## Surprising Connections (you probably didn't know these)
- `Requests` --references--> `Document`  [INFERRED]
  README.md → gen/proteus/v1/conversion.pb.go
- `Merging and splitting PDFs` --references--> `PdfInput`  [INFERRED]
  README.md → gen/proteus/v1/pdf.pb.go
- `Merging and splitting PDFs` --references--> `PdfFile`  [INFERRED]
  README.md → gen/proteus/v1/pdf.pb.go
- `Adding a format` --references--> `Converter`  [INFERRED]
  README.md → internal/convert/formats.go
- `content()` --references--> `PdfInput`  [EXTRACTED]
  internal/rpc/pdf_test.go → gen/proteus/v1/pdf.pb.go

## Import Cycles
- 2-file cycle: `internal/templates/store_test.go -> internal/templates/storetest/storetest.go -> internal/templates/store_test.go`
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`

## Communities (95 total, 39 thin omitted)

### Community 0 - "Community 0"
Cohesion: 0.12
Nodes (15): file_proteus_v1_conversion_proto_init(), init(), file_proteus_v1_pdf_proto_init(), init(), file_proteus_v1_protection_proto_init(), init(), PdfServiceHandler, file_proteus_v1_template_proto_init() (+7 more)

### Community 1 - "Community 1"
Cohesion: 0.17
Nodes (3): BadRequestError, GeneratePassword(), TestGeneratePassword()

### Community 2 - "Community 2"
Cohesion: 0.08
Nodes (10): ctxKey, RegisterConversionServiceHandler(), RegisterPdfServiceHandler(), RegisterTemplateServiceHandler(), WithCaller(), isOps(), requestIDFrom(), serverFault() (+2 more)

### Community 3 - "Community 3"
Cohesion: 0.06
Nodes (8): WhenSet(), Int64Col, NullBytesCol, Pred, Sort, TextCol, TimeCol, UUIDCol

### Community 4 - "Community 4"
Cohesion: 0.17
Nodes (29): NewConversionServiceClient(), content(), pdfPages(), renderPDF(), TestMergeAndSplitPdfs(), TestPdfPasswords(), TestPdfValidation(), expectCode() (+21 more)

### Community 5 - "Community 5"
Cohesion: 0.14
Nodes (13): Manager, NewManager(), publicError(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull(), TestShutdownRequeuesRunningJob() (+5 more)

### Community 6 - "Community 6"
Cohesion: 0.14
Nodes (10): warmUp(), Limits, Request, Result, badf(), FromProto(), TestFromProtoRejects(), validateOptions() (+2 more)

### Community 7 - "Community 7"
Cohesion: 0.19
Nodes (10): Key, Authenticator, HashKey(), New(), ParseKeys(), redact(), TestDisabled(), TestKeys() (+2 more)

### Community 8 - "Community 8"
Cohesion: 0.09
Nodes (26): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+18 more)

### Community 9 - "Community 9"
Cohesion: 0.13
Nodes (15): ConverterFunc, Input, ProteusEngine, engineOptions(), NewRegistry(), TestFromProto(), NewService(), stubRegistry() (+7 more)

### Community 10 - "Community 10"
Cohesion: 0.08
Nodes (21): Settings, checkWebhookURL(), CleanAssetPath(), decodeDataURL(), Limits, Loader, New(), short() (+13 more)

### Community 11 - "Community 11"
Cohesion: 0.15
Nodes (10): newLogger(), run(), parser, FromEnv(), Config, env(), TestDefaults(), TestErrors() (+2 more)

### Community 13 - "Community 13"
Cohesion: 0.12
Nodes (5): SplitPdfRequest, SplitPdfResponse, pdfServiceClient, UnimplementedPdfServiceHandler, isSplitPdfRequest_Mode

### Community 14 - "Community 14"
Cohesion: 0.16
Nodes (3): MergePdfsRequest, MergePdfsResponse, pdfService

### Community 16 - "Community 16"
Cohesion: 0.13
Nodes (8): TestMemoryStore(), Outcome, Record, Store, Run(), fromMeta(), fromRow(), Status

### Community 20 - "Community 20"
Cohesion: 0.18
Nodes (4): Insert(), putBinder(), scan(), binder

### Community 21 - "Community 21"
Cohesion: 0.13
Nodes (21): countStmtFor(), delAllStmtFor(), distinctStmtFor(), existsStmtFor(), metaStmtFor(), stmtFor(), takeStmtFor(), trimStmtFor() (+13 more)

### Community 23 - "Community 23"
Cohesion: 0.15
Nodes (16): And(), assignable(), assignAdded(), assignExcluded(), Create(), GetBinder(), InsertOp(), Inserts() (+8 more)

### Community 25 - "Community 25"
Cohesion: 0.04
Nodes (10): ConvertResponse, DeleteJobRequest, DeleteJobResponse, GetJobRequest, GetJobResponse, ListFormatsRequest, ListFormatsResponse, ConversionServiceClient (+2 more)

### Community 27 - "Community 27"
Cohesion: 0.10
Nodes (20): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_ALLOW_NO_AUTH, PROTEUS_DATABASE_URL, interval, retries (+12 more)

### Community 30 - "Community 30"
Cohesion: 0.10
Nodes (5): conversionServiceClient, ConversionServiceDownloadJobResultClientStream, conversionServiceHandler, templateServiceClient, templateServiceHandler

### Community 31 - "Community 31"
Cohesion: 0.07
Nodes (4): SubmitJobRequest, Asset, Document, isDocument_Source

### Community 32 - "Community 32"
Cohesion: 0.22
Nodes (3): mergeOptions(), TestMergeOptions(), OutputFormat

### Community 33 - "Community 33"
Cohesion: 0.17
Nodes (9): bad(), Input, Protection, Service, newLimitedBuffer(), selectPages(), limitedBuffer, Result (+1 more)

### Community 34 - "Community 34"
Cohesion: 0.21
Nodes (8): Migrate(), New(), Open(), parseID(), TestStore(), MutateKey(), New(), Store

### Community 36 - "Community 36"
Cohesion: 0.03
Nodes (11): WhenSet(), Int32Col, Int64Col, NullBytesCol, NullTextArrayCol, NullTextCol, NullTimeCol, Pred (+3 more)

### Community 38 - "Community 38"
Cohesion: 0.14
Nodes (6): MetaRow, Row, Mutate(), Scan(), scanMeta(), rowSource

### Community 39 - "Community 39"
Cohesion: 0.13
Nodes (14): Delete(), InsertAll(), LockTimeout(), Delete(), MetaRow, Insert(), InsertAll(), putBinder() (+6 more)

### Community 40 - "Community 40"
Cohesion: 0.20
Nodes (8): bad(), document(), formatName(), Service, NewService(), ToProto(), Rendered, Settings

### Community 41 - "Community 41"
Cohesion: 0.15
Nodes (5): inputFromProto(), isPdfInput_Source, PdfInput, PdfInput_Content, PdfInput_Url

### Community 42 - "Community 42"
Cohesion: 0.17
Nodes (11): New(), isBad(), samplePDF(), TestCanceledContextStopsMerge(), TestLimits(), TestOutputLimitStopsEarly(), New(), Settings (+3 more)

### Community 43 - "Community 43"
Cohesion: 0.13
Nodes (14): API, Configuration, Development, Errors, Jobs, templates and Postgres, Merging and splitting PDFs, Operations, proteus-server (+6 more)

### Community 44 - "Community 44"
Cohesion: 0.21
Nodes (5): h2cClient(), healthcheck(), main(), subcommand(), NewKey()

### Community 45 - "Community 45"
Cohesion: 0.23
Nodes (4): Converter, Registry, Route, Adding a format

### Community 47 - "Community 47"
Cohesion: 0.15
Nodes (3): File, fileToProto(), PdfFile

### Community 48 - "Community 48"
Cohesion: 0.24
Nodes (6): idString(), fromTemplateMeta(), nameErr(), Create(), New(), Templates

### Community 49 - "Community 49"
Cohesion: 0.17
Nodes (8): clone(), Record, Store, NewMemoryStore(), newUUID(), TestMemoryStore(), Run(), MemoryStore

### Community 50 - "Community 50"
Cohesion: 0.24
Nodes (3): newUUID(), memJob, MemoryStore

### Community 51 - "Community 51"
Cohesion: 0.05
Nodes (5): file_proteus_v1_conversion_proto_rawDescGZIP(), ConvertRequest, Metadata, Options, PdfOptions

### Community 53 - "Community 53"
Cohesion: 0.06
Nodes (4): TemplateServiceClient, ConvertTemplateRequest, ConvertTemplateResponse, SubmitTemplateJobRequest

### Community 54 - "Community 54"
Cohesion: 0.24
Nodes (5): Caller(), FileName(), TestFileName(), conversionService, templateService

### Community 55 - "Community 55"
Cohesion: 0.11
Nodes (3): Mutate(), MutateKey(), Mut

### Community 56 - "Community 56"
Cohesion: 0.23
Nodes (7): PdfServiceClient, NewPdfServiceClient(), NewTemplateServiceClient(), h2c(), withKey(), env, env

### Community 57 - "Community 57"
Cohesion: 0.23
Nodes (10): cleanErr(), Execute(), Parse(), data_(), render(), TestEscapeErrorIsTemplateError(), TestLimits(), TestRefused() (+2 more)

### Community 58 - "Community 58"
Cohesion: 0.35
Nodes (10): InputFormat, OutputFormat, Target, InputToProto(), OutputFromProto(), OutputToProto(), TargetOf(), ToProto() (+2 more)

### Community 60 - "Community 60"
Cohesion: 0.24
Nodes (3): file_proteus_v1_protection_proto_rawDescGZIP(), protectionFromProto(), Protection

### Community 61 - "Community 61"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 62 - "Community 62"
Cohesion: 0.21
Nodes (4): _assertJobShape(), _assertTemplateShape(), Job, Template

### Community 63 - "Community 63"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 64 - "Community 64"
Cohesion: 0.16
Nodes (3): toFloat(), List, run

### Community 66 - "Community 66"
Cohesion: 0.09
Nodes (23): DeleteOp(), FragOf(), fragOf(), Lowering(), And(), assignable(), assignAdded(), assignExcluded() (+15 more)

### Community 67 - "Community 67"
Cohesion: 0.09
Nodes (3): GetTemplateRequest, GetTemplateResponse, UnimplementedTemplateServiceHandler

### Community 68 - "Community 68"
Cohesion: 0.11
Nodes (4): nopObserver, Observer, Metrics, Options

### Community 69 - "Community 69"
Cohesion: 0.19
Nodes (3): Sign(), TestSign(), NewUnit()

### Community 72 - "Community 72"
Cohesion: 0.04
Nodes (9): file_proteus_v1_template_proto_rawDescGZIP(), CreateTemplateRequest, CreateTemplateResponse, ListTemplatesResponse, UpdateTemplateRequest, UpdateTemplateResponse, TemplateServiceHandler, RenderTemplateRequest_Template (+1 more)

### Community 79 - "Community 79"
Cohesion: 0.12
Nodes (3): DownloadJobResultResponse, SubmitTemplateJobResponse, PageRanges

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 473 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **39 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Community 17` to `Community 34`, `Community 36`, `Community 38`, `Community 75`, `Community 76`, `Community 82`, `Community 19`, `Community 20`, `Community 21`, `Community 85`, `Community 23`, `Community 86`, `Community 26`?**
  _High betweenness centrality (0.153) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.11585365853658537 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Community 37` to `Community 66`, `Community 3`, `Community 39`, `Community 12`, `Community 48`, `Community 21`, `Community 87`, `Community 88`, `Community 26`?**
  _High betweenness centrality (0.123) - this node is a cross-community bridge._
- **Should `Community 2` be split into smaller, more focused modules?**
  _Cohesion score 0.07954545454545454 - nodes in this community are weakly interconnected._
- **Why does `Pred` connect `Community 36` to `Community 3`, `Community 75`, `Community 76`, `Community 17`, `Community 82`, `Community 19`, `Community 85`, `Community 86`, `Community 23`, `Community 26`?**
  _High betweenness centrality (0.063) - this node is a cross-community bridge._
- **Should `Community 3` be split into smaller, more focused modules?**
  _Cohesion score 0.06219426974143955 - nodes in this community are weakly interconnected._