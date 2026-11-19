#!/usr/bin/env bash
# Call every RPC of proteus.v1.ConversionService with buf curl, the way any
# gRPC client would. Needs a running server (scripts/dev.sh), buf and jq.
# The schema comes from the server's reflection service, so no proto files are
# needed on the client side.
#
#   PROTEUS_URL      server base URL (default http://127.0.0.1:8080)
#   PROTEUS_API_KEY  API key; leave empty when the server runs with --no-auth
#
# Results are written to samples/out/curl.
set -euo pipefail
cd "$(dirname "$0")/.."

url="${PROTEUS_URL:-http://127.0.0.1:8080}"
out=samples/out/curl
mkdir -p "$out"
svc="$url/proteus.v1.ConversionService"
args=(--protocol grpc --http2-prior-knowledge)
if [[ -n "${PROTEUS_API_KEY:-}" ]]; then
  args+=(-H "authorization: Bearer $PROTEUS_API_KEY")
fi
call() { buf curl "${args[@]}" --data "$2" "$svc/$1"; }
b64() { base64 < "$1" | tr -d '\n'; }
is_pdf() { head -c5 "$1" | grep -q '^%PDF-' || { echo "$1 is not a PDF" >&2; exit 1; }; }

echo "-- grpc.health.v1.Health/Check"
buf curl "${args[@]}" "$url/grpc.health.v1.Health/Check"

echo "-- ListFormats"
call ListFormats '{}'

echo "-- Convert: Markdown with page options"
call Convert "$(jq -n --arg c "$(b64 samples/docs/report/index.md)" --arg chart "$(b64 samples/docs/report/chart.svg)" '{
  document: {format: "INPUT_FORMAT_MARKDOWN", content: $c, assets: [{path: "chart.svg", data: $chart}]},
  options: {page: {size: "PAGE_SIZE_A4", marginMm: 15}, metadata: {title: "Q3 report (buf curl)"}}
}')" > "$out/convert.json"
jq -r .content "$out/convert.json" | base64 -d > "$out/report.pdf"
is_pdf "$out/report.pdf"
echo "wrote $out/report.pdf ($(jq -r .fileName "$out/convert.json"), $(jq -r .durationMs "$out/convert.json") ms)"

echo "-- Convert: inline HTML"
call Convert "$(jq -n --arg c "$(printf '<h1 style="color:#1d4e89">Hello from buf curl</h1><p>Inline HTML, no assets.</p>' | base64 | tr -d '\n')" \
  '{document: {format: "INPUT_FORMAT_HTML", content: $c}}')" | jq -r .content | base64 -d > "$out/hello.pdf"
is_pdf "$out/hello.pdf"
echo "wrote $out/hello.pdf"

echo "-- SubmitJob"
id="$(call SubmitJob "$(jq -n --arg c "$(b64 samples/docs/letter/index.html)" '{document: {format: "INPUT_FORMAT_HTML", content: $c}}')" | jq -r .job.id)"
echo "job $id"

echo "-- GetJob (poll)"
status=""
for _ in $(seq 1 120); do
  status="$(call GetJob "{\"id\":\"$id\"}" | jq -r .job.status)"
  [[ $status == JOB_STATUS_SUCCEEDED || $status == JOB_STATUS_FAILED ]] && break
  sleep 0.25
done
echo "status $status"
[[ $status == JOB_STATUS_SUCCEEDED ]] || { echo "job did not succeed" >&2; exit 1; }

echo "-- DownloadJobResult (server stream)"
call DownloadJobResult "{\"id\":\"$id\"}" | jq -r .chunk | while read -r chunk; do printf '%s' "$chunk" | base64 -d; done > "$out/letter.pdf"
is_pdf "$out/letter.pdf"
echo "wrote $out/letter.pdf"

echo "-- DeleteJob"
call DeleteJob "{\"id\":\"$id\"}"

echo "buf curl: ok"
