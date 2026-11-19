# Samples

Sample documents and example clients for proteus-server. Start a dev server
first:

```sh
scripts/dev.sh                 # or: make dev
# dev key: export PROTEUS_API_KEY=pk_dev_local
```

Then, in another terminal:

```sh
export PROTEUS_API_KEY=pk_dev_local
go run ./samples/client samples        # every sample, with Convert and with a job
samples/buf-curl.sh                    # every RPC with buf curl (needs buf and jq)
```

Or do it all in one go, as CI does: `scripts/smoke.sh` (or `make smoke`)
starts the server, runs every sample over gRPC, Connect and gRPC-Web, runs the
buf curl calls, and stops the server. PDFs land in `samples/out/` (ignored by
git).

## Documents

Each directory in `docs/` is one request. The `index.html` or `index.md` file
is the document; every other file is sent as an asset under its relative path
(so `<img src="img/logo.svg">` finds `img/logo.svg`); `options.json`, when
present, is the request's `Options` as protobuf JSON.

| Sample | Shows |
| --- | --- |
| `invoice` | HTML with a linked stylesheet, an SVG logo asset, flexbox, a striped table, `@page` size and margins, PDF metadata and tagging. |
| `report` | Markdown (GitHub flavour): headings, lists, a table, a quote, code, an SVG chart; options set the page size and margins, metadata, bookmarks, and extra CSS. |
| `letter` | Self-contained HTML with an inline `<style>`, A5 landscape from `@page`, text with ligatures (fi, st) and an italic serif signature. |

Extra CSS for Markdown must target `.markdown-body`: the built-in Markdown theme
sets fonts and colours there, so a rule on `body` alone does not override it.

Add a sample by adding a directory; `samples` and the smoke test pick it up.

## Go client

[`client/main.go`](client/main.go) uses the generated ConnectRPC client from
`gen/proteus/v1`. It is a working example of every RPC, including polling a
job and reading the `DownloadJobResult` stream:

```sh
go run ./samples/client formats
go run ./samples/client convert samples/docs/invoice            # writes Invoice-INV-2026-0042.pdf
go run ./samples/client convert -o readme.pdf README.md         # any .html/.md file works too
go run ./samples/client job -o report.pdf samples/docs/report   # SubmitJob, GetJob, DownloadJobResult, DeleteJob
go run ./samples/client -protocol connect samples                # Connect protocol over HTTP/1.1, as a browser would
```

Flags before the command: `-url` (`PROTEUS_URL`, default
`http://127.0.0.1:8080`), `-key` (`PROTEUS_API_KEY`) and `-protocol` (`grpc`,
`connect` or `grpcweb`).

## buf curl

[`buf-curl.sh`](buf-curl.sh) calls health, `ListFormats`, `Convert` (Markdown
with an asset and options, and inline HTML), then `SubmitJob`, `GetJob`,
`DownloadJobResult` and `DeleteJob`. buf reads the schema from the server's
reflection service, so no proto files are needed.
