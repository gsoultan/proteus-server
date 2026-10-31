# Task completion checklist

1. Start: read .serena/memories/*.md and graphify-out/GRAPH_REPORT.md (through rtk when installed).
2. Before done: `gofmt -l .` empty, `go vet ./...`, `go test -race ./...`.
3. Keep README (API, statuses, env table) in sync with code changes.
4. Finish: update these memories, run `graphify update .`, commit both with the code.
5. Commits and PRs: author gsoultan <gembit.shirazi@gmail.com>; no Co-Authored-By or any
   Claude/AI credit lines; check what landed on GitHub afterwards.
