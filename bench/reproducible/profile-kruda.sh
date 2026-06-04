#!/usr/bin/env bash
# Capture Kruda-only CPU profiles for reproducible benchmark routes.
#
# This script is intentionally profiling-focused, not a benchmark claim source.
# Use it to choose the next Wing candidate only after route-level A/B evidence
# shows whether that candidate should ship.
#
# Default routes avoid PostgreSQL and profile handler CPU paths only:
#   /plaintext-handler, /json-static, /json-serialize
#
# Usage:
#   ./profile-kruda.sh                         # profile default CPU routes
#   ./profile-kruda.sh json-static             # profile one CPU route
#   BENCH_ENABLE_DB=1 ./profile-kruda.sh db    # opt in to DB routes

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
KRUDA_DIR="$SCRIPT_DIR/kruda"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/profile-$TIMESTAMP"}"
RAW_DIR="$RESULT_DIR/raw"
PROFILE_DIR="$RESULT_DIR/profiles"
REPORT_DIR="$RESULT_DIR/reports"
META_FILE="$RESULT_DIR/environment.txt"

PORT_VALUE="${PORT:-3000}"
PPROF_PORT_VALUE="${PPROF_PORT:-6060}"
GOMAXPROCS_VALUE="${GOMAXPROCS:-8}"
KRUDA_WORKERS_VALUE="${KRUDA_WORKERS:-4}"
KRUDA_READ_BUF_SIZE_VALUE="${KRUDA_READ_BUF_SIZE:-4096}"
KRUDA_POOL_SIZE_VALUE="${KRUDA_POOL_SIZE:-}"
BENCH_ENABLE_DB_VALUE="${BENCH_ENABLE_DB:-0}"
BENCH_KRUDA_DB_DISPATCH_VALUE="${BENCH_KRUDA_DB_DISPATCH:-takeover}"
BENCH_DURATION_VALUE="${BENCH_DURATION:-15}"
WARMUP_DURATION_VALUE="${WARMUP_DURATION:-3}"
THREADS_VALUE="${THREADS:-4}"
CONNECTIONS_VALUE="${CONNECTIONS:-256}"
DATABASE_URL_VALUE="${DATABASE_URL:-postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world?pool_max_conns=64&pool_min_conns=8}"
KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS-kruda_stdjson}"
if [ "$KRUDA_GO_TAGS_VALUE" = "default" ] || [ "$KRUDA_GO_TAGS_VALUE" = "none" ]; then
  KRUDA_GO_TAGS_VALUE=""
fi
case " $KRUDA_GO_TAGS_VALUE " in
  *" bench_pprof "*) ;;
  *) KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS_VALUE:+$KRUDA_GO_TAGS_VALUE }bench_pprof" ;;
esac

ROUTES=("$@")
if [ "${#ROUTES[@]}" -eq 0 ]; then
  ROUTES=(plaintext-handler json-static json-serialize)
fi

KRUDA_PID=""

cleanup() {
  if [ -n "$KRUDA_PID" ] && kill -0 "$KRUDA_PID" >/dev/null 2>&1; then
    kill "$KRUDA_PID" >/dev/null 2>&1 || true
    wait "$KRUDA_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

abs_path() {
  local path="$1"
  local dir
  dir="$(cd -- "$(dirname -- "$path")" && pwd)"
  printf '%s/%s\n' "$dir" "$(basename -- "$path")"
}

wait_ready() {
  local url="$1"
  local log="$2"
  for _ in $(seq 1 100); do
    if ! kill -0 "$KRUDA_PID" >/dev/null 2>&1; then
      echo "kruda exited before it became ready" >&2
      cat "$log" >&2 || true
      exit 1
    fi
    if curl -fsS "$url" >/dev/null 2>&1; then
      sleep 0.05
      if ! kill -0 "$KRUDA_PID" >/dev/null 2>&1; then
        echo "kruda exited after readiness check at $url" >&2
        cat "$log" >&2 || true
        exit 1
      fi
      return
    fi
    sleep 0.1
  done
  echo "kruda did not become ready at $url" >&2
  cat "$log" >&2 || true
  exit 1
}

write_environment() {
  {
    echo "timestamp_utc=$TIMESTAMP"
    echo "script_dir=$SCRIPT_DIR"
    echo "result_dir=$RESULT_DIR"
    echo "routes=${ROUTES[*]}"
    echo "port=$PORT_VALUE"
    echo "pprof_port=$PPROF_PORT_VALUE"
    echo "gomaxprocs=$GOMAXPROCS_VALUE"
    echo "kruda_workers=$KRUDA_WORKERS_VALUE"
    echo "kruda_read_buf_size=$KRUDA_READ_BUF_SIZE_VALUE"
    echo "kruda_pool_size=${KRUDA_POOL_SIZE_VALUE:-default}"
    echo "bench_enable_db=$BENCH_ENABLE_DB_VALUE"
    echo "bench_kruda_db_dispatch=$BENCH_KRUDA_DB_DISPATCH_VALUE"
    echo "threads=$THREADS_VALUE"
    echo "connections=$CONNECTIONS_VALUE"
    echo "profile_seconds=$BENCH_DURATION_VALUE"
    echo "warmup_seconds=$WARMUP_DURATION_VALUE"
    echo "kruda_go_tags=${KRUDA_GO_TAGS_VALUE:-default}"
    echo
    echo "== CPU =="
    if command -v lscpu >/dev/null 2>&1; then
      lscpu
    elif command -v sysctl >/dev/null 2>&1; then
      sysctl -n machdep.cpu.brand_string 2>/dev/null || true
      sysctl -n hw.physicalcpu hw.logicalcpu 2>/dev/null || true
    fi
    echo
    echo "== OS =="
    uname -a
    echo
    echo "== Toolchain =="
    go version
    wrk --version 2>&1 || true
  } > "$META_FILE"
}

build_kruda() {
  require_cmd go
  require_cmd wrk
  require_cmd curl
  mkdir -p "$RAW_DIR" "$PROFILE_DIR" "$REPORT_DIR"
  local kruda_tag_args=()
  if [ -n "$KRUDA_GO_TAGS_VALUE" ]; then
    kruda_tag_args=(-tags "$KRUDA_GO_TAGS_VALUE")
  fi

  (cd "$KRUDA_DIR" && GOWORK=off go build "${kruda_tag_args[@]}" -o kruda-bench .)
}

start_kruda() {
  local log="$RAW_DIR/server-kruda.log"
  local ready_url="http://127.0.0.1:$PORT_VALUE/plaintext-handler"
  local pprof_ready_url="http://127.0.0.1:$PPROF_PORT_VALUE/debug/pprof/"
  if curl -fsS --max-time 1 "$ready_url" >/dev/null 2>&1; then
    echo "kruda readiness URL already responds before start: $ready_url" >&2
    echo "choose a free port with PORT" >&2
    exit 1
  fi
  if curl -fsS --max-time 1 "$pprof_ready_url" >/dev/null 2>&1; then
    echo "kruda pprof URL already responds before start: $pprof_ready_url" >&2
    echo "choose a free port with PPROF_PORT" >&2
    exit 1
  fi
  (
    cd "$KRUDA_DIR"
    env GOMAXPROCS="$GOMAXPROCS_VALUE" \
      KRUDA_WORKERS="$KRUDA_WORKERS_VALUE" \
      KRUDA_READ_BUF_SIZE="$KRUDA_READ_BUF_SIZE_VALUE" \
      KRUDA_POOL_SIZE="$KRUDA_POOL_SIZE_VALUE" \
      PORT="$PORT_VALUE" \
      PPROF_PORT="$PPROF_PORT_VALUE" \
      BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" \
      BENCH_ENABLE_PPROF=1 \
      BENCH_KRUDA_DB_DISPATCH="$BENCH_KRUDA_DB_DISPATCH_VALUE" \
      DATABASE_URL="$DATABASE_URL_VALUE" \
      ./kruda-bench
  ) > "$log" 2>&1 &
  KRUDA_PID="$!"
  wait_ready "http://127.0.0.1:$PORT_VALUE/plaintext-handler" "$log"
  wait_ready "http://127.0.0.1:$PPROF_PORT_VALUE/debug/pprof/" "$log"
}

capture_route() {
  local route="$1"
  local url="http://127.0.0.1:$PORT_VALUE/$route"
  local pprof_url="http://127.0.0.1:$PPROF_PORT_VALUE/debug/pprof/profile?seconds=$BENCH_DURATION_VALUE"
  local wrk_raw="$RAW_DIR/kruda-$route-wrk.txt"
  local profile="$PROFILE_DIR/kruda-$route-cpu.pb.gz"
  local top_report="$REPORT_DIR/kruda-$route-top.txt"
  local profile_abs
  local top_report_abs
  local load_seconds=$((BENCH_DURATION_VALUE + 2))

  echo "warmup $route"
  wrk --latency -t"$THREADS_VALUE" -c"$CONNECTIONS_VALUE" -d"${WARMUP_DURATION_VALUE}s" "$url" > "$RAW_DIR/kruda-$route-warmup.txt"

  echo "profile $route"
  wrk --latency -t"$THREADS_VALUE" -c"$CONNECTIONS_VALUE" -d"${load_seconds}s" "$url" > "$wrk_raw" &
  local wrk_pid="$!"
  sleep 1
  curl -fsS "$pprof_url" > "$profile"
  wait "$wrk_pid"
  profile_abs="$(abs_path "$profile")"
  top_report_abs="$(abs_path "$top_report")"
  (cd "$KRUDA_DIR" && go tool pprof -top -nodecount=40 ./kruda-bench "$profile_abs") > "$top_report_abs"
}

main() {
  build_kruda
  write_environment
  start_kruda
  for route in "${ROUTES[@]}"; do
    capture_route "$route"
  done
  echo "profile results: $RESULT_DIR"
}

main "$@"
