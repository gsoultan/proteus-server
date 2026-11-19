# Graph Report - proteus-server  (2026-11-19)

## Corpus Check
- 117 files · ~176,067 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 8 file(s) not represented in the graph (top: (none) 4, .proto 3, .lock 1)

## Summary
- 4122 nodes · 10694 edges · 136 communities (80 shown, 56 thin omitted)
- Extraction: 82% EXTRACTED · 18% INFERRED · 0% AMBIGUOUS · INFERRED: 1913 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `da44db10`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- template.pb.go
- google.golang.org/protobuf/reflect/protoreflect.Message
- API
- Pred
- testing.T
- Manager
- Service
- auth.go
- steps
- service_test.go
- safefetch.go
- Query
- Query
- conventions_and_gaps.md
- proteus-server overview
- task_completion_checklist.md
- github.com/gsoultan/proteus-server
- Query
- google.golang.org/protobuf/reflect/protoreflect.EnumNumber
- layout.ts
- job.gen.go
- github.com/gsoultan/storm/runtime.Tok
- Mut
- serversetting.gen.go
- Ins
- file_proteus_v1_conversion_proto_rawDescGZIP
- time.Time
- proteus
- DownloadJobResultRequest
- Job
- context.Context
- Document
- ConvertResponse
- settings.tsx
- template_pb.ts
- Margins
- Pred
- TimeCol
- package.json
- github.com/gsoultan/storm/runtime.Executor
- Template
- file_proteus_v1_setup_proto_rawDescGZIP
- conversion_pb.ts
- Pred
- CompleteSetupRequest
- apikey.gen.go
- setup_pb.ts
- Canvas.tsx
- Mut
- Store
- Query
- Options
- Conversion
- ConvertTemplateRequest
- ToProto
- Mut
- Sort
- Sort
- settings.ts
- ListTemplatesRequest
- Setting
- buf.yaml
- shape.gen.go
- plugins
- run
- Region
- template.gen.go
- RenderTemplateRequest
- time.Duration
- main.go
- DeleteTemplateRequest
- FieldFormat
- UpdateTemplateRequest
- DeleteJobRequest
- Ins
- FieldSpec
- CreateTemplateRequest
- Layout
- server.go
- Mut
- run
- .isRenderTemplateRequest_Source
- Pred
- checker
- Block
- ConvertToCodeRequest
- Watermark
- github.com/gsoultan/storm/runtime.Frag
- .AllInto
- OutputFormat
- GetSetupStatusResponse
- ListStarterTemplatesResponse
- Ins
- BlockStyle
- proteus-server
- google.golang.org/protobuf/reflect/protoreflect.EnumDescriptor
- Column
- Image
- CheckSetupTokenRequest
- Condition
- TotalRow
- compilerOptions
- Table
- .table
- Theme
- Format
- google.golang.org/protobuf/reflect/protoreflect.EnumType
- FieldType
- ApiKey_Role
- PageOptions
- CompleteSetupResponse
- NewService
- Totals
- JobStatus
- PageSize
- ConditionOp
- FieldFormat
- WatermarkLayer
- WatermarkPages
- WatermarkPlacement
- PageSize
- JobStatus
- .isWatermark_Mark
- Block_Columns
- Block_Divider
- Block_Field
- Block_Image
- Block_KeyValue
- Block_PageBreak
- Block_Qr
- Block_Section
- Block_Signature
- Block_Spacer
- Block_Text
- Block_Totals

## God Nodes (most connected - your core abstractions)
1. `Query` - 248 edges
2. `Query` - 170 edges
3. `Query` - 137 edges
4. `Query` - 103 edges
5. `Pred` - 86 edges
6. `Template` - 68 edges
7. `file_proteus_v1_template_proto_rawDescGZIP()` - 56 edges
8. `Mut` - 49 edges
9. `Pred` - 48 edges
10. `Pred` - 48 edges

## Surprising Connections (you probably didn't know these)
- `Adding a format` --references--> `Converter`  [INFERRED]
  README.md → internal/convert/formats.go
- `subcommand()` --calls--> `HashKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `subcommand()` --calls--> `NewKey()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `openDatabase()` --references--> `Store`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/storage/pgstore/pgstore.go
- `openDatabase()` --calls--> `Open()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/storage/pgstore/pgstore.go

## Import Cycles
- 2-file cycle: `internal/jobs/memstore_test.go -> internal/jobs/storetest/storetest.go -> internal/jobs/memstore_test.go`
- 2-file cycle: `internal/settings/settings_test.go -> internal/settings/storetest/storetest.go -> internal/settings/settings_test.go`
- 2-file cycle: `internal/templates/store_test.go -> internal/templates/storetest/storetest.go -> internal/templates/store_test.go`

## Communities (136 total, 56 thin omitted)

### Community 0 - "template.pb.go"
Cohesion: 0.16
Nodes (7): BadRequestError, file_proteus_v1_conversion_proto_init(), init(), file_proteus_v1_setup_proto_init(), init(), file_proteus_v1_template_proto_init(), init()

### Community 1 - "google.golang.org/protobuf/reflect/protoreflect.Message"
Cohesion: 0.02
Nodes (17): DeleteApiKeyResponse, GetSettingsRequest, ListApiKeysRequest, file_proteus_v1_template_proto_rawDescGZIP(), DataError, DataProblem, Divider, Heading (+9 more)

### Community 2 - "API"
Cohesion: 0.06
Nodes (27): newLogger(), openDatabase(), RegisterConversionServiceHandler(), SetupServiceClient, SetupServiceHandler, NewSetupServiceClient(), RegisterAdminServiceHandler(), RegisterSetupServiceHandler() (+19 more)

### Community 3 - "Pred"
Cohesion: 0.05
Nodes (7): And(), Grp, Int64Col, NullBytesCol, Pred, Sort, UUIDCol

### Community 4 - "testing.T"
Cohesion: 0.12
Nodes (32): NewConversionServiceClient(), NewTemplateServiceClient(), TestBuilderLifecycle(), TestStartersToPDF(), expectCode(), expectPDF(), h2c(), html() (+24 more)

### Community 5 - "Manager"
Cohesion: 0.12
Nodes (14): Manager, NewManager(), publicError(), Sign(), newSvc(), submitReq(), TestLifecycle(), TestQueueFull() (+6 more)

### Community 6 - "Service"
Cohesion: 0.13
Nodes (8): warmUp(), Request, badf(), FileName(), TestFileName(), dedupe(), Result, Service

### Community 7 - "auth.go"
Cohesion: 0.17
Nodes (14): ctxKey, roleKey, HashKey(), New(), NewKey(), ParseKeys(), redact(), TestDisabled() (+6 more)

### Community 8 - "steps"
Cohesion: 0.08
Nodes (28): runs-on, steps, POSTGRES_PASSWORD, PROTEUS_TEST_DATABASE_URL, jobs, docker, test, name (+20 more)

### Community 9 - "service_test.go"
Cohesion: 0.31
Nodes (11): ConverterFunc, NewRegistry(), NewService(), stubRegistry(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown() (+3 more)

### Community 10 - "safefetch.go"
Cohesion: 0.08
Nodes (21): Settings, checkWebhookURL(), CleanAssetPath(), decodeDataURL(), Limits, Loader, New(), short() (+13 more)

### Community 17 - "Query"
Cohesion: 0.03
Nodes (3): NullTextCol, Query, TextCol

### Community 19 - "layout.ts"
Cohesion: 0.05
Nodes (74): BlockPanel(), ColumnRow(), DataPanel(), FIELD_TYPES, FieldList(), Inspector(), KVRow(), PagePanel() (+66 more)

### Community 20 - "job.gen.go"
Cohesion: 0.05
Nodes (28): assignable(), assignAdded(), assignExcluded(), countStmtFor(), delAllStmtFor(), distinctStmtFor(), existsStmtFor(), GetBinder() (+20 more)

### Community 21 - "github.com/gsoultan/storm/runtime.Tok"
Cohesion: 0.12
Nodes (26): countStmtFor(), delAllStmtFor(), distinctStmtFor(), existsStmtFor(), stmtFor(), updAllStmtFor(), updAllStmtIn(), updHead() (+18 more)

### Community 23 - "serversetting.gen.go"
Cohesion: 0.06
Nodes (22): DeleteOp(), Lowering(), DeleteOp(), Lowering(), assignable(), assignAdded(), assignExcluded(), Delete() (+14 more)

### Community 25 - "file_proteus_v1_conversion_proto_rawDescGZIP"
Cohesion: 0.05
Nodes (13): file_proteus_v1_conversion_proto_rawDescGZIP(), GetJobRequest, GetJobResponse, ListFormatsRequest, ListFormatsResponse, SubmitJobResponse, ConversionServiceClient, NewAdminServiceClient() (+5 more)

### Community 27 - "proteus"
Cohesion: 0.09
Nodes (22): postgres, POSTGRES_DB, POSTGRES_PASSWORD, POSTGRES_USER, PROTEUS_DATABASE_URL, interval, retries, test (+14 more)

### Community 28 - "DownloadJobResultRequest"
Cohesion: 0.11
Nodes (4): DownloadJobResultRequest, DownloadJobResultResponse, ConversionServiceDownloadJobResultServerStream, ConversionServiceDownloadJobResultClientStream

### Community 30 - "context.Context"
Cohesion: 0.08
Nodes (12): ConversionServiceHandler, AdminServiceClient, AdminServiceHandler, adminServiceClient, adminServiceHandler, conversionServiceHandler, setupServiceClient, setupServiceHandler (+4 more)

### Community 31 - "Document"
Cohesion: 0.08
Nodes (9): Limits, FromProto(), TestFromProto(), TestFromProtoRejects(), Asset, Document, Document_Content, Document_Url (+1 more)

### Community 33 - "settings.tsx"
Cohesion: 0.07
Nodes (79): @bufbuild/protobuf, @mantine/core, @mantine/hooks, react, @tabler/icons-react, @tanstack/react-query, @tanstack/react-router, apiKeyInterceptor() (+71 more)

### Community 34 - "template_pb.ts"
Cohesion: 0.02
Nodes (88): RFC-3339, Aggregate, AggregateFuncSchema, AggregateSchema, AlignSchema, BlockStyle, Columns, ColumnSchema (+80 more)

### Community 36 - "Pred"
Cohesion: 0.03
Nodes (10): And(), Grp, Int32Col, Int64Col, NullBytesCol, NullTextArrayCol, Pred, Sort (+2 more)

### Community 38 - "package.json"
Cohesion: 0.03
Nodes (72): @bufbuild/protoc-gen-es, @connectrpc/connect, @connectrpc/connect-web, @mantine/notifications, pdfjs-dist, react-dom, @tanstack/router-plugin, @types/bun (+64 more)

### Community 39 - "github.com/gsoultan/storm/runtime.Executor"
Cohesion: 0.13
Nodes (10): Delete(), MetaRow, Insert(), InsertAll(), putBinder(), Scan(), scan(), scanMeta() (+2 more)

### Community 40 - "Template"
Cohesion: 0.13
Nodes (10): ApplyDefaults(), FormatOf(), Starter(), bad(), document(), Service, layoutOptions(), ToProto() (+2 more)

### Community 41 - "file_proteus_v1_setup_proto_rawDescGZIP"
Cohesion: 0.04
Nodes (13): file_proteus_v1_setup_proto_rawDescGZIP(), CreateApiKeyRequest, CreateApiKeyResponse, DeleteApiKeyRequest, ListApiKeysResponse, UpdateSettingsRequest, UpdateSettingsResponse, Caller() (+5 more)

### Community 42 - "conversion_pb.ts"
Cohesion: 0.03
Nodes (69): Asset, AssetSchema, Conversion, ConversionSchema, ConvertRequest, ConvertRequestSchema, ConvertResponse, ConvertResponseSchema (+61 more)

### Community 43 - "Pred"
Cohesion: 0.05
Nodes (5): And(), WhenSet(), Grp, Pred, TextCol

### Community 44 - "CompleteSetupRequest"
Cohesion: 0.04
Nodes (9): CompleteSetupRequest, GetSettingsResponse, TestDatabaseRequest, TestDatabaseResponse, databaseValues(), control, setupService, Database (+1 more)

### Community 45 - "apikey.gen.go"
Cohesion: 0.05
Nodes (20): Binder, Ins, Row, rowSource, assignable(), assignExcluded(), Create(), Delete() (+12 more)

### Community 46 - "setup_pb.ts"
Cohesion: 0.04
Nodes (53): ApiKey, ApiKey_Role, ADMIN, CLIENT, UNSPECIFIED, ApiKey_RoleSchema, ApiKeySchema, CheckSetupTokenRequest (+45 more)

### Community 47 - "Canvas.tsx"
Cohesion: 0.07
Nodes (45): ALIGN, Band(), BlockView(), Canvas(), Ctx, Kind(), KV(), Label() (+37 more)

### Community 48 - "Mut"
Cohesion: 0.10
Nodes (10): Insert(), Mutate(), MutateKey(), putBinder(), Scan(), scan(), stmtForKey(), binder (+2 more)

### Community 49 - "Store"
Cohesion: 0.05
Nodes (34): newUUID(), TestMemoryStore(), Outcome, Record, Store, Run(), fromMeta(), fromRow() (+26 more)

### Community 51 - "Options"
Cohesion: 0.09
Nodes (8): ProteusEngine, engineOptions(), TestWatermarkOptions(), validateOptions(), mergeOptions(), TestMergeOptions(), Metadata, Options

### Community 52 - "Conversion"
Cohesion: 0.12
Nodes (3): formatName(), Conversion, InputFormat

### Community 53 - "ConvertTemplateRequest"
Cohesion: 0.07
Nodes (4): ConvertTemplateRequest, ConvertTemplateResponse, SubmitTemplateJobRequest, SubmitTemplateJobResponse

### Community 54 - "ToProto"
Cohesion: 0.16
Nodes (14): Converter, Input, InputFormat, OutputFormat, Registry, Route, Target, InputToProto() (+6 more)

### Community 55 - "Mut"
Cohesion: 0.11
Nodes (4): Mutate(), MutateKey(), stmtForKey(), Mut

### Community 56 - "Sort"
Cohesion: 0.08
Nodes (5): Int64Col, NullBytesCol, Sort, TimeCol, UUIDCol

### Community 57 - "Sort"
Cohesion: 0.08
Nodes (4): NullBytesCol, Sort, TimeCol, UUIDCol

### Community 58 - "settings.ts"
Cohesion: 0.10
Nodes (28): Setting, Setting_Kind, BOOL, BYTES, DURATION, FLOAT, INT, LIST (+20 more)

### Community 60 - "Setting"
Cohesion: 0.08
Nodes (3): Setting, Setting_Kind, Setting_Source

### Community 61 - "buf.yaml"
Cohesion: 0.25
Nodes (7): breaking, use, lint, use, modules, path, version

### Community 62 - "shape.gen.go"
Cohesion: 0.16
Nodes (8): _assertAPIKeyShape(), _assertJobShape(), _assertServerSettingShape(), _assertTemplateShape(), APIKey, Job, ServerSetting, Template

### Community 63 - "plugins"
Cohesion: 0.29
Nodes (6): clean, plugins, local, opt, out, version

### Community 64 - "run"
Cohesion: 0.16
Nodes (6): arith(), cleanErr(), toFloat(), List, Number, run

### Community 65 - "Region"
Cohesion: 0.09
Nodes (4): regionHeight(), Columns, Region, Section

### Community 66 - "template.gen.go"
Cohesion: 0.08
Nodes (18): assignable(), assignAdded(), assignExcluded(), delAllStmtFor(), Delete(), distinctStmtFor(), GetBinder(), InsertOp() (+10 more)

### Community 67 - "RenderTemplateRequest"
Cohesion: 0.07
Nodes (7): TemplateServiceClient, GetTemplateRequest, GetTemplateResponse, RenderTemplateRequest, RenderTemplateResponse, templateService, isRenderTemplateRequest_Source

### Community 68 - "time.Duration"
Cohesion: 0.06
Nodes (9): parser, nopObserver, Observer, Metrics, New(), LockTimeout(), nopObserver, Observer (+1 more)

### Community 69 - "main.go"
Cohesion: 0.07
Nodes (27): h2cClient(), healthcheck(), main(), serve(), subcommand(), Field, FromEnv(), env() (+19 more)

### Community 71 - "FieldFormat"
Cohesion: 0.08
Nodes (3): Field, FieldFormat, KeyValueRow

### Community 75 - "FieldSpec"
Cohesion: 0.14
Nodes (7): applyDefaults(), isEmptyValue(), lookup(), ValidateFields(), validateFields(), dataChecker, FieldSpec

### Community 77 - "Layout"
Cohesion: 0.14
Nodes (13): Compile(), TestCompileLayout(), TestCompileLayoutErrors(), TestValidateFields(), textBlock(), Execute(), ExecuteWith(), data_() (+5 more)

### Community 78 - "server.go"
Cohesion: 0.09
Nodes (7): ctxKey, logInfo, LayoutError, limitWriter, TemplateError, Built(), TestHandler()

### Community 79 - "Mut"
Cohesion: 0.14
Nodes (4): Mut, Mutate(), MutateKey(), stmtForKey()

### Community 80 - "run"
Cohesion: 0.06
Nodes (22): newSetupToken(), rpcKeys(), run(), watchSettings(), Authenticator, Key, authKeys(), NewFileStore() (+14 more)

### Community 82 - "Pred"
Cohesion: 0.15
Nodes (3): Grp, Pred, And()

### Community 84 - "Block"
Cohesion: 0.10
Nodes (4): Block, Block_Heading, Block_Table, isBlock_Kind

### Community 85 - "ConvertToCodeRequest"
Cohesion: 0.15
Nodes (3): TemplateServiceHandler, ConvertToCodeRequest, ConvertToCodeResponse

### Community 86 - "Watermark"
Cohesion: 0.20
Nodes (4): watermarkOption(), validateWatermark(), isWatermark_Mark, Watermark

### Community 87 - "github.com/gsoultan/storm/runtime.Frag"
Cohesion: 0.33
Nodes (8): FragOf(), fragOf(), FragOf(), fragOf(), FragOf(), fragOf(), FragOf(), fragOf()

### Community 91 - "ListStarterTemplatesResponse"
Cohesion: 0.16
Nodes (3): ListStarterTemplatesRequest, ListStarterTemplatesResponse, UnimplementedTemplateServiceHandler

### Community 92 - "Ins"
Cohesion: 0.11
Nodes (3): Create(), New(), Ins

### Community 94 - "proteus-server"
Cohesion: 0.11
Nodes (18): Adding a format, API, Configuration, Development, Errors, Fields, Jobs, templates and Postgres, Operations (+10 more)

### Community 96 - "Column"
Cohesion: 0.12
Nodes (3): alignClass(), Align, Column

### Community 97 - "Image"
Cohesion: 0.12
Nodes (4): Image, Image_Asset, Image_Path, isImage_Source

### Community 100 - "TotalRow"
Cohesion: 0.13
Nodes (4): isTotalRow_Value, TotalRow, TotalRow_Aggregate, TotalRow_Path

### Community 101 - "compilerOptions"
Cohesion: 0.12
Nodes (15): compilerOptions, isolatedModules, jsx, lib, module, moduleResolution, noEmit, noUncheckedIndexedAccess (+7 more)

### Community 103 - ".table"
Cohesion: 0.43
Nodes (5): validPath(), checkFormat(), ref(), value(), compiler

### Community 105 - "Format"
Cohesion: 0.24
Nodes (6): KnownCurrency(), KnownLocale(), TestFormat(), currencyInfo, Format, localeInfo

### Community 107 - "FieldType"
Cohesion: 0.18
Nodes (11): FieldType, BOOL, CHOICE, DATE, IMAGE, LIST, MONEY, NUMBER (+3 more)

### Community 111 - "NewService"
Cohesion: 0.28
Nodes (8): CheckData(), Starters(), TestStarterSpecifics(), TestStartersRender(), NewMemoryStore(), TestMemoryStore(), NewService(), Settings

### Community 115 - "ConditionOp"
Cohesion: 0.25
Nodes (8): ConditionOp, EQUALS, GREATER, IS_EMPTY, IS_SET, LESS, NOT_EQUALS, UNSPECIFIED

### Community 116 - "FieldFormat"
Cohesion: 0.25
Nodes (8): FieldFormat, DATE, INTEGER, MONEY, NUMBER, PERCENT, TEXT, UNSPECIFIED

### Community 120 - "PageSize"
Cohesion: 0.29
Nodes (7): PageSize, A3, A4, A5, LEGAL, LETTER, UNSPECIFIED

### Community 121 - "JobStatus"
Cohesion: 0.33
Nodes (6): JobStatus, FAILED, QUEUED, RUNNING, SUCCEEDED, UNSPECIFIED

## Knowledge Gaps
- **88 isolated node(s):** `github.com/gsoultan/proteus-server`, `UNSPECIFIED`, `HTML`, `MARKDOWN`, `UNSPECIFIED` (+83 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1100 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **56 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Query` connect `Query` to `Pred`, `Store`, `job.gen.go`, `github.com/gsoultan/storm/runtime.Tok`, `time.Time`?**
  _High betweenness centrality (0.084) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server`, `UNSPECIFIED`, `HTML` to the rest of the system?**
  _88 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `google.golang.org/protobuf/reflect/protoreflect.Message` be split into smaller, more focused modules?**
  _Cohesion score 0.02385712381713981 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Query` to `template.gen.go`, `Pred`, `TimeCol`, `github.com/gsoultan/storm/runtime.Executor`, `Store`, `github.com/gsoultan/storm/runtime.Tok`, `Mut`, `time.Time`?**
  _High betweenness centrality (0.070) - this node is a cross-community bridge._
- **Should `API` be split into smaller, more focused modules?**
  _Cohesion score 0.06205673758865248 - nodes in this community are weakly interconnected._
- **Why does `Query` connect `Query` to `apikey.gen.go`, `Mut`, `run`, `Pred`, `github.com/gsoultan/storm/runtime.Tok`, `.AllInto`, `Sort`, `time.Time`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._
- **Should `Pred` be split into smaller, more focused modules?**
  _Cohesion score 0.04970914859862507 - nodes in this community are weakly interconnected._