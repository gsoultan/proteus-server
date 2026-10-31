# Graph Report - proteus-server  (2026-10-31)

## Corpus Check
- 24 files · ~14,417 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 3 file(s) not represented in the graph (top: (none) 3)

## Summary
- 329 nodes · 886 edges · 17 communities (13 shown, 4 thin omitted)
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 58 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- main.go
- Metrics
- API
- request.go
- api_test.go
- Manager
- Service
- Authenticator
- steps
- jobs_test.go
- safefetch_test.go
- parser
- proteus-server
- conventions_and_gaps.md
- project_overview.md
- task_completion_checklist.md
- github.com/gsoultan/proteus-server

## God Nodes (most connected - your core abstractions)
1. `API` - 24 edges
2. `Manager` - 23 edges
3. `newEnv()` - 21 edges
4. `Service` - 20 edges
5. `Metrics` - 16 edges
6. `Request` - 14 edges
7. `Client` - 14 edges
8. `NewService()` - 13 edges
9. `Job` - 13 edges
10. `run()` - 12 edges

## Surprising Connections (you probably didn't know these)
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `ParseKeys()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/auth/auth.go
- `run()` --calls--> `FromEnv()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/config/config.go
- `run()` --calls--> `NewService()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/convert/service.go
- `run()` --calls--> `New()`  [EXTRACTED]
  cmd/proteus-server/main.go → internal/httpapi/server.go

## Import Cycles
- None detected.

## Communities (17 total, 4 thin omitted)

### Community 0 - "main.go"
Cohesion: 0.08
Nodes (14): ctxKey, main(), newLogger(), run(), warmUp(), ctxKey, errorBody, jobResponse (+6 more)

### Community 1 - "Metrics"
Cohesion: 0.06
Nodes (12): Observer, Settings, Config, Config, New(), Limits, Metrics, Client (+4 more)

### Community 2 - "API"
Cohesion: 0.23
Nodes (6): API, statusWriter, Caller(), requestIDFrom(), writeError(), writeJSON()

### Community 3 - "request.go"
Cohesion: 0.16
Nodes (15): BadRequestError, jsonBody, Margins, Metadata, Options, badf(), FileName(), Request (+7 more)

### Community 4 - "api_test.go"
Cohesion: 0.24
Nodes (19): envOpts, testEnv, expectPDF(), mustJSON(), newEnv(), pngPixel(), readBody(), TestAuth() (+11 more)

### Community 5 - "Manager"
Cohesion: 0.14
Nodes (7): checkWebhookURL(), Job, Manager, newID(), publicError(), nopObserver, Status

### Community 6 - "Service"
Cohesion: 0.18
Nodes (6): nopObserver, WithCaller(), Result, Service, short(), Loader

### Community 7 - "Authenticator"
Cohesion: 0.18
Nodes (12): Key, subcommand(), Authenticator, HashKey(), New(), NewKey(), ParseKeys(), redact() (+4 more)

### Community 8 - "steps"
Cohesion: 0.13
Nodes (17): runs-on, steps, jobs, docker, test, name, on, pull_request (+9 more)

### Community 9 - "jobs_test.go"
Cohesion: 0.26
Nodes (13): NewService(), stubService(), TestOutputChecks(), TestPanicBecomesError(), TestRealEngineMarkdown(), TestSlots(), TestTimeout(), NewManager() (+5 more)

### Community 10 - "safefetch_test.go"
Cohesion: 0.19
Nodes (12): CleanAssetPath(), New(), TestCleanAssetPath(), TestLimits(), TestLoad(), TestRemoteGoesThroughSafefetch(), New(), TestAllowlist() (+4 more)

### Community 11 - "parser"
Cohesion: 0.20
Nodes (6): parser, FromEnv(), env(), TestDefaults(), TestErrors(), TestParse()

### Community 12 - "proteus-server"
Cohesion: 0.17
Nodes (11): API, Configuration, Development, Operations, proteus-server, Quick start, Request bodies, Responses (+3 more)

## Knowledge Gaps
- **1 isolated node(s):** `github.com/gsoultan/proteus-server`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 83 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `API` connect `API` to `main.go`, `Metrics`, `Manager`, `Service`, `Authenticator`?**
  _High betweenness centrality (0.078) - this node is a cross-community bridge._
- **What connects `github.com/gsoultan/proteus-server` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.0777323202805377 - nodes in this community are weakly interconnected._
- **Why does `Manager` connect `Manager` to `main.go`, `Metrics`, `API`, `api_test.go`, `Service`, `Authenticator`, `jobs_test.go`?**
  _High betweenness centrality (0.069) - this node is a cross-community bridge._
- **Should `Metrics` be split into smaller, more focused modules?**
  _Cohesion score 0.06282051282051282 - nodes in this community are weakly interconnected._
- **Why does `Metrics` connect `Metrics` to `main.go`, `API`?**
  _High betweenness centrality (0.060) - this node is a cross-community bridge._
- **Should `Manager` be split into smaller, more focused modules?**
  _Cohesion score 0.1422924901185771 - nodes in this community are weakly interconnected._