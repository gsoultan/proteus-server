#!/usr/bin/env bash
# End-to-end check: start the dev server, run every sample through the Go
# client over gRPC, Connect and gRPC-Web, run the buf curl samples when buf is
# installed, then stop the server. Extra arguments go to scripts/dev.sh
# (for example --postgres). PDFs land in samples/out.
set -euo pipefail
cd "$(dirname "$0")/.."

export PROTEUS_DEV_KEY="${PROTEUS_DEV_KEY:-pk_dev_local}"
export PROTEUS_API_KEY="$PROTEUS_DEV_KEY"
export PROTEUS_ADDR="${PROTEUS_ADDR:-127.0.0.1:8080}"
export PROTEUS_URL="http://$PROTEUS_ADDR"
log="$(mktemp)"

go build -o bin/proteus-client ./samples/client
scripts/dev.sh "$@" >"$log" 2>&1 &
server=$!
cleanup() {
  kill "$server" 2>/dev/null || true
  wait "$server" 2>/dev/null || true
}
trap cleanup EXIT

ready=0
for _ in $(seq 1 120); do
  if ! kill -0 "$server" 2>/dev/null; then break; fi
  if [[ -x bin/proteus-server ]] && bin/proteus-server healthcheck >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.5
done
if [[ $ready != 1 ]]; then
  echo "smoke: server did not become healthy" >&2
  cat "$log" >&2
  exit 1
fi

status=0
for protocol in grpc connect grpcweb; do
  echo "== Go client over $protocol"
  bin/proteus-client -protocol "$protocol" samples -out "samples/out/$protocol" || status=1
done
if command -v buf >/dev/null; then
  echo "== buf curl"
  samples/buf-curl.sh || status=1
else
  echo "== buf curl skipped (buf not installed)"
fi

if [[ $status != 0 ]]; then
  echo "smoke: FAILED; server log:" >&2
  cat "$log" >&2
  exit 1
fi
echo "smoke: ok"
