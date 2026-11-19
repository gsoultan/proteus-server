#!/usr/bin/env bash
# Run proteus-server locally with development defaults.
#
#   scripts/dev.sh               auth on with a fixed dev key, jobs in memory
#   scripts/dev.sh --no-auth     no API key (PROTEUS_ALLOW_NO_AUTH=true)
#   scripts/dev.sh --postgres    jobs in Postgres: starts the compose postgres service
#                                (needs Docker) unless PROTEUS_DATABASE_URL is already set
#
# Every PROTEUS_* variable you export wins over the defaults in this script. The dev key
# is PROTEUS_DEV_KEY (default pk_dev_local); clients send it as
# "authorization: Bearer pk_dev_local". Listeners bind to 127.0.0.1 only.
set -euo pipefail
cd "$(dirname "$0")/.."

auth=1
postgres=0
for arg in "$@"; do
  case "$arg" in
    --no-auth) auth=0 ;;
    --postgres) postgres=1 ;;
    -h|--help) sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "dev.sh: unknown option $arg (try --help)" >&2; exit 2 ;;
  esac
done

mkdir -p bin
go build -o bin/proteus-server ./cmd/proteus-server

: "${PROTEUS_ADDR:=127.0.0.1:8080}"
: "${PROTEUS_ADMIN_ADDR:=127.0.0.1:9090}"
: "${PROTEUS_LOG_FORMAT:=text}"
: "${PROTEUS_LOG_LEVEL:=debug}"
: "${PROTEUS_TIMEOUT:=60s}"
: "${PROTEUS_SHUTDOWN_TIMEOUT:=5s}"
: "${PROTEUS_RATE_LIMIT:=0}" # off: the samples send bursts; set it to try the limiter
export PROTEUS_ADDR PROTEUS_ADMIN_ADDR PROTEUS_LOG_FORMAT PROTEUS_LOG_LEVEL PROTEUS_TIMEOUT PROTEUS_SHUTDOWN_TIMEOUT PROTEUS_RATE_LIMIT

if [[ $auth == 1 && -z "${PROTEUS_API_KEYS:-}" && -z "${PROTEUS_API_KEYS_FILE:-}" ]]; then
  key="${PROTEUS_DEV_KEY:-pk_dev_local}"
  export PROTEUS_API_KEYS="dev:$(bin/proteus-server hash "$key")"
  echo "dev key: export PROTEUS_API_KEY=$key" >&2
elif [[ $auth == 0 ]]; then
  export PROTEUS_ALLOW_NO_AUTH=true
fi

if [[ $postgres == 1 && -z "${PROTEUS_DATABASE_URL:-}" ]]; then
  docker compose up -d --wait postgres
  export PROTEUS_DATABASE_URL="postgres://proteus:proteus@127.0.0.1:${PROTEUS_DEV_PG_PORT:-55432}/proteus?sslmode=disable"
fi
if [[ -n "${PROTEUS_DATABASE_URL:-}" ]]; then
  echo "jobs: Postgres" >&2
else
  echo "jobs: in memory" >&2
fi

echo "rpc: http://$PROTEUS_ADDR  metrics: http://$PROTEUS_ADMIN_ADDR/metrics" >&2
exec bin/proteus-server
