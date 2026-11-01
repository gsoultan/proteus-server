# Task completion checklist

1. Start: read .serena/memories/*.md and graphify-out/GRAPH_REPORT.md (through rtk when installed).
2. Toolchain: GOTOOLCHAIN=auto (go 1.27); tools buf v1.75.0, protoc-gen-go v1.36.12,
   protoc-gen-connect-go v2.0.2, storm v1.11.0.
3. Before done: `buf lint`, `buf generate` and `storm verify -stale internal/storage/store` leave
   no diff; `gofmt -l .` empty, `go vet ./...`, `go test -race ./...` (with
   PROTEUS_TEST_DATABASE_URL for the Postgres store suite).
4. Keep README (RPCs, codes, env table) in sync with code changes.
5. Finish: update these memories, run `graphify update .`, commit both with the code.
6. Commits and PRs: author gsoultan <gembit.shirazi@gmail.com>; no Co-Authored-By or any
   Claude/AI credit lines; check what landed on GitHub afterwards.
