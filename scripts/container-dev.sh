#!/usr/bin/env bash
# Run proteus-server and Postgres for local development in Apple containers
# (https://github.com/apple/container), on a Mac with Apple silicon.
#
#   scripts/container-dev.sh [up]     build the image, start Postgres and the server, wait until healthy
#   scripts/container-dev.sh logs     follow the server log
#   scripts/container-dev.sh status   show the containers
#   scripts/container-dev.sh smoke    run every sample against the running server (needs Go)
#   scripts/container-dev.sh down     stop and delete the containers (the database volume is kept)
#   scripts/container-dev.sh reset    down, then delete the database volume too
#
# Options for up:
#   --no-db      keep jobs in memory; no Postgres container
#   --no-auth    no API key (PROTEUS_ALLOW_NO_AUTH=true)
#   --no-build   reuse the existing proteus-server:dev image
#
# The server listens on 127.0.0.1:8080 (gRPC, gRPC-Web, Connect) and
# 127.0.0.1:9090 (metrics); Postgres on 127.0.0.1:55432 (user, password and
# database "proteus"). The dev key is PROTEUS_DEV_KEY (default pk_dev_local).
# Set PROTEUS_HTTP_PORT, PROTEUS_ADMIN_PORT or PROTEUS_PG_PORT to move a port.
# The server reaches Postgres by its container IP, so this works on the
# default network (macOS 15 and later) without `container network create`.
set -euo pipefail
cd "$(dirname "$0")/.."

IMAGE="${PROTEUS_IMAGE:-proteus-server:dev}"
APP=proteus-dev
PG=proteus-dev-postgres
VOLUME=proteus-dev-pgdata
HTTP_PORT="${PROTEUS_HTTP_PORT:-8080}"
ADMIN_PORT="${PROTEUS_ADMIN_PORT:-9090}"
PG_PORT="${PROTEUS_PG_PORT:-55432}"
DEV_KEY="${PROTEUS_DEV_KEY:-pk_dev_local}"

say() { printf '\033[1m==> %s\033[0m\n' "$*" >&2; }
die() { printf 'container-dev: %s\n' "$*" >&2; exit 1; }

need_container() {
  command -v container >/dev/null ||
    die "the 'container' CLI is not installed: get the signed installer from https://github.com/apple/container/releases"
  if ! container system status >/dev/null 2>&1; then
    say "starting the container system service"
    container system start --enable-kernel-install
  fi
}

# exists NAME: the container exists, running or not.
exists() { container ls --all --quiet | grep -qx "$1"; }
running() { container ls --quiet | grep -qx "$1"; }

remove() {
  if exists "$1"; then
    container stop "$1" >/dev/null 2>&1 || true
    container delete --force "$1" >/dev/null
  fi
}

# ip NAME: the container's IPv4 address on its first network, without the prefix length.
ip() {
  container inspect "$1" |
    grep -oE '"(ipv4Address|address)"[[:space:]]*:[[:space:]]*"[0-9.]+' |
    head -n1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+'
}

# sha256 of the dev key, the form PROTEUS_API_KEYS stores (same as `proteus-server hash`).
key_hash() {
  if command -v shasum >/dev/null; then
    printf '%s' "$1" | shasum -a 256 | cut -d' ' -f1
  else
    printf '%s' "$1" | sha256sum | cut -d' ' -f1
  fi
}

wait_for() { # wait_for SECONDS CMD...
  local secs=$1
  shift
  for _ in $(seq 1 "$secs"); do
    if "$@" >/dev/null 2>&1; then return 0; fi
    sleep 1
  done
  return 1
}

up() {
  local db=1 auth=1 build=1
  for arg in "$@"; do
    case "$arg" in
      --no-db) db=0 ;;
      --no-auth) auth=0 ;;
      --no-build) build=0 ;;
      *) die "unknown option $arg (try --help)" ;;
    esac
  done
  need_container

  if [[ $build == 1 ]]; then
    say "building $IMAGE"
    container build --tag "$IMAGE" --file Dockerfile .
  fi

  local env=(
    --env PROTEUS_LOG_FORMAT=text
    --env PROTEUS_LOG_LEVEL=debug
    --env PROTEUS_RATE_LIMIT=0
    --env PROTEUS_TIMEOUT=60s
    --env PROTEUS_SHUTDOWN_TIMEOUT=5s
  )
  if [[ $auth == 1 ]]; then
    env+=(--env "PROTEUS_API_KEYS=dev:$(key_hash "$DEV_KEY")")
  else
    env+=(--env PROTEUS_ALLOW_NO_AUTH=true)
  fi

  if [[ $db == 1 ]]; then
    if ! running "$PG"; then
      remove "$PG"
      if ! container volume ls --quiet | grep -qx "$VOLUME"; then
        container volume create "$VOLUME" >/dev/null
      fi
      say "starting Postgres ($PG, 127.0.0.1:$PG_PORT)"
      # PGDATA sits in a subdirectory: a fresh volume has lost+found at its root, which initdb refuses.
      container run --detach --name "$PG" \
        --env POSTGRES_USER=proteus --env POSTGRES_PASSWORD=proteus --env POSTGRES_DB=proteus \
        --env PGDATA=/var/lib/postgresql/data/pgdata \
        --volume "$VOLUME:/var/lib/postgresql/data" \
        --publish "127.0.0.1:$PG_PORT:5432" \
        postgres:17 >/dev/null
    fi
    wait_for 60 container exec "$PG" pg_isready -U proteus -d proteus ||
      { container logs -n 50 "$PG" >&2 || true; die "Postgres did not become ready"; }
    local pg_ip
    pg_ip="$(ip "$PG")" || die "could not read the IP address of $PG"
    env+=(--env "PROTEUS_DATABASE_URL=postgres://proteus:proteus@$pg_ip:5432/proteus?sslmode=disable")
  fi

  remove "$APP"
  say "starting proteus-server ($APP, 127.0.0.1:$HTTP_PORT)"
  container run --detach --name "$APP" "${env[@]}" \
    --publish "127.0.0.1:$HTTP_PORT:8080" \
    --publish "127.0.0.1:$ADMIN_PORT:9090" \
    "$IMAGE" >/dev/null
  # The image is distroless; its own healthcheck subcommand does a gRPC health call.
  wait_for 90 container exec "$APP" /proteus-server healthcheck ||
    { container logs -n 50 "$APP" >&2 || true; die "proteus-server did not become healthy"; }

  say "ready"
  cat >&2 <<EOF
  rpc:      http://127.0.0.1:$HTTP_PORT  (gRPC h2c, gRPC-Web, Connect)
  metrics:  http://127.0.0.1:$ADMIN_PORT/metrics
  jobs:     $([[ $db == 1 ]] && echo "Postgres, postgres://proteus:proteus@127.0.0.1:$PG_PORT/proteus" || echo "in memory")
  auth:     $([[ $auth == 1 ]] && echo "export PROTEUS_API_KEY=$DEV_KEY" || echo "off")

  try it:   scripts/container-dev.sh smoke
  logs:     scripts/container-dev.sh logs
  stop:     scripts/container-dev.sh down
EOF
}

smoke() {
  command -v go >/dev/null || die "smoke needs Go to run samples/client"
  running "$APP" || die "$APP is not running; start it with scripts/container-dev.sh up"
  export PROTEUS_URL="http://127.0.0.1:$HTTP_PORT" PROTEUS_API_KEY="$DEV_KEY"
  for protocol in grpc connect grpcweb; do
    say "samples over $protocol"
    go run ./samples/client -protocol "$protocol" samples -out "samples/out/container-$protocol"
  done
  if command -v buf >/dev/null && command -v jq >/dev/null; then
    say "buf curl"
    samples/buf-curl.sh
  fi
}

cmd="${1:-up}"
[[ $# -gt 0 ]] && shift
case "$cmd" in
  up) up "$@" ;;
  --no-*) up "$cmd" "$@" ;;
  logs) need_container; container logs --follow "$APP" ;;
  status) need_container; container ls --all | awk -v a="$APP" -v p="$PG" 'NR == 1 || $1 == a || $1 == p' ;;
  smoke) need_container; smoke ;;
  down) need_container; remove "$APP"; remove "$PG"; say "stopped (volume $VOLUME kept)" ;;
  reset)
    need_container
    remove "$APP"
    remove "$PG"
    if container volume ls --quiet | grep -qx "$VOLUME"; then container volume delete "$VOLUME" >/dev/null; fi
    say "stopped and deleted volume $VOLUME"
    ;;
  -h | --help | help) sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//' ;;
  *) die "unknown command $cmd (try --help)" ;;
esac
