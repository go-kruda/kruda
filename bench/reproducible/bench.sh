#!/usr/bin/env bash
# Reproducible CPU-bound benchmark: Kruda vs Fiber vs Actix.
#
# Default routes avoid PostgreSQL and measure handler CPU paths only:
#   /plaintext-handler, /json-static, /json-serialize
#
# Usage:
#   ./bench.sh                         # run default CPU-bound routes
#   ./bench.sh json-static             # run one route
#   BENCH_ENABLE_DB=1 ./bench.sh db    # opt in to DB routes

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/$TIMESTAMP"}"
RAW_DIR="$RESULT_DIR/raw"
SUMMARY_CSV="$RESULT_DIR/summary.csv"
SUMMARY_MD="$RESULT_DIR/summary.md"
META_FILE="$RESULT_DIR/environment.txt"

GOMAXPROCS_VALUE="${GOMAXPROCS:-8}"
# The benchmark profiles use wrk -t4. Keep Kruda's harness worker count aligned
# with the active load-generator threads unless the run is explicitly studying
# worker scaling. This does not change Kruda's framework default.
KRUDA_WORKERS_VALUE="${KRUDA_WORKERS:-4}"
ACTIX_WORKERS_VALUE="${BENCH_ACTIX_WORKERS:-}"
KRUDA_READ_BUF_SIZE_VALUE="${KRUDA_READ_BUF_SIZE:-}"
KRUDA_POOL_SIZE_VALUE="${KRUDA_POOL_SIZE:-}"
BENCH_ENABLE_DB_VALUE="${BENCH_ENABLE_DB:-0}"
BENCH_ENABLE_PPROF_VALUE="${BENCH_ENABLE_PPROF:-0}"
BENCH_KRUDA_DB_DISPATCH_VALUE="${BENCH_KRUDA_DB_DISPATCH:-takeover}"
BENCH_KRUDA_CPU_DISPATCH_VALUE="${BENCH_KRUDA_CPU_DISPATCH:-inline}"
KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS-kruda_stdjson}"
if [ "$KRUDA_GO_TAGS_VALUE" = "default" ] || [ "$KRUDA_GO_TAGS_VALUE" = "none" ]; then
  KRUDA_GO_TAGS_VALUE=""
fi
if [ "$BENCH_ENABLE_PPROF_VALUE" = "1" ]; then
  case " $KRUDA_GO_TAGS_VALUE " in
    *" bench_pprof "*) ;;
    *) KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS_VALUE:+$KRUDA_GO_TAGS_VALUE }bench_pprof" ;;
  esac
fi
BENCH_ROUNDS_VALUE="${BENCH_ROUNDS:-5}"
BENCH_DURATION_VALUE="${BENCH_DURATION:-15s}"
DATABASE_BASE_URL_VALUE="${BENCH_DATABASE_BASE_URL:-postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world}"
DATABASE_PGX_URL_VALUE="${DATABASE_BASE_URL_VALUE}?pool_max_conns=64&pool_min_conns=8"
KRUDA_DATABASE_URL_VALUE="${KRUDA_DATABASE_URL:-${DATABASE_URL:-$DATABASE_PGX_URL_VALUE}}"
FIBER_DATABASE_URL_VALUE="${FIBER_DATABASE_URL:-${DATABASE_URL:-$DATABASE_PGX_URL_VALUE}}"
ACTIX_DATABASE_URL_VALUE="${ACTIX_DATABASE_URL:-${DATABASE_URL:-$DATABASE_BASE_URL_VALUE}}"

read -r -a FRAMEWORKS <<< "${BENCH_FRAMEWORKS:-kruda fiber actix}"
KRUDA_PID=""
FIBER_PID=""
ACTIX_PID=""

PROFILES=(
  "latency:-t4 -c128 -d$BENCH_DURATION_VALUE"
  "throughput:-t4 -c256 -d$BENCH_DURATION_VALUE"
)

# Opt-in low-concurrency profiles (BENCH_LOWC=1) for characterizing latency when
# the server is not saturated — closer to many real-world services than c128/c256.
# Default runs are unchanged.
if [ "${BENCH_LOWC:-0}" = "1" ]; then
  PROFILES+=(
    "lowc8:-t4 -c8 -d$BENCH_DURATION_VALUE"
    "lowc16:-t4 -c16 -d$BENCH_DURATION_VALUE"
    "lowc32:-t4 -c32 -d$BENCH_DURATION_VALUE"
  )
fi

if [ "$#" -gt 0 ]; then
  ROUTES=("$@")
else
  ROUTES=(plaintext-handler json-static json-serialize)
fi

mkdir -p "$RAW_DIR"

cleanup() {
  for fw in "${FRAMEWORKS[@]}"; do
    stop_server "$fw"
  done
}
trap cleanup EXIT INT TERM

port_for() {
  case "$1" in
    kruda) echo "${KRUDA_PORT:-3000}" ;;
    fiber) echo "${FIBER_PORT:-3002}" ;;
    actix) echo "${ACTIX_PORT:-3003}" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

pid_for() {
  case "$1" in
    kruda) echo "$KRUDA_PID" ;;
    fiber) echo "$FIBER_PID" ;;
    actix) echo "$ACTIX_PID" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

set_pid() {
  case "$1" in
    kruda) KRUDA_PID="$2" ;;
    fiber) FIBER_PID="$2" ;;
    actix) ACTIX_PID="$2" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

clear_pid() {
  case "$1" in
    kruda) KRUDA_PID="" ;;
    fiber) FIBER_PID="" ;;
    actix) ACTIX_PID="" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

build_all() {
  require_cmd go
  require_cmd wrk
  require_cmd curl

  local kruda_tag_args=()
  if [ -n "$KRUDA_GO_TAGS_VALUE" ]; then
    kruda_tag_args=(-tags "$KRUDA_GO_TAGS_VALUE")
  fi

  for fw in "${FRAMEWORKS[@]}"; do
    case "$fw" in
      kruda)
        (cd "$SCRIPT_DIR/kruda" && GOWORK=off go build "${kruda_tag_args[@]}" -o kruda-bench .)
        ;;
      fiber)
        (cd "$SCRIPT_DIR/fiber" && GOWORK=off go build -o fiber-bench .)
        ;;
      actix)
        require_cmd cargo
        (cd "$SCRIPT_DIR/actix" && cargo build --release)
        ;;
      *)
        echo "unknown framework: $fw" >&2
        exit 1
        ;;
    esac
  done
}

write_environment() {
  {
    echo "timestamp_utc=$TIMESTAMP"
    echo "script_dir=$SCRIPT_DIR"
    echo "repo_root=$REPO_ROOT"
    echo "git_commit=$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || true)"
    if git -C "$REPO_ROOT" diff --quiet --ignore-submodules -- && git -C "$REPO_ROOT" diff --cached --quiet --ignore-submodules --; then
      echo "git_tracked_dirty=0"
    else
      echo "git_tracked_dirty=1"
    fi
    echo "result_dir=$RESULT_DIR"
    echo "bench_enable_db=$BENCH_ENABLE_DB_VALUE"
    echo "bench_enable_pprof=$BENCH_ENABLE_PPROF_VALUE"
    echo "bench_kruda_db_dispatch=$BENCH_KRUDA_DB_DISPATCH_VALUE"
    echo "bench_kruda_cpu_dispatch=$BENCH_KRUDA_CPU_DISPATCH_VALUE"
    if [ -n "${DATABASE_URL:-}" ]; then echo "database_url_common_override=1"; else echo "database_url_common_override=0"; fi
    if [ -n "${KRUDA_DATABASE_URL:-}" ]; then echo "kruda_database_url_override=1"; else echo "kruda_database_url_override=0"; fi
    if [ -n "${FIBER_DATABASE_URL:-}" ]; then echo "fiber_database_url_override=1"; else echo "fiber_database_url_override=0"; fi
    if [ -n "${ACTIX_DATABASE_URL:-}" ]; then echo "actix_database_url_override=1"; else echo "actix_database_url_override=0"; fi
    echo "kruda_go_tags=${KRUDA_GO_TAGS_VALUE:-default}"
    echo "gomaxprocs=$GOMAXPROCS_VALUE"
    echo "kruda_workers=$KRUDA_WORKERS_VALUE"
    echo "actix_workers=${ACTIX_WORKERS_VALUE:-default}"
    echo "kruda_read_buf_size=${KRUDA_READ_BUF_SIZE_VALUE:-default}"
    echo "kruda_pool_size=${KRUDA_POOL_SIZE_VALUE:-default}"
    echo "bench_rounds=$BENCH_ROUNDS_VALUE"
    echo "bench_duration=$BENCH_DURATION_VALUE"
    echo "frameworks=${FRAMEWORKS[*]}"
    echo "routes=${ROUTES[*]}"
    echo "profiles=${PROFILES[*]}"
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
    if command -v rustc >/dev/null 2>&1; then
      rustc --version 2>/dev/null || echo "rustc=not usable"
    else
      echo "rustc=not found"
    fi
    if command -v cargo >/dev/null 2>&1; then
      cargo --version 2>/dev/null || echo "cargo=not usable"
    else
      echo "cargo=not found"
    fi
    wrk --version 2>&1 || true
  } > "$META_FILE"
}

start_server() {
  local fw="$1"
  local port
  port="$(port_for "$fw")"
  local log="$RAW_DIR/server-$fw.log"

  case "$fw" in
    kruda)
      (
        cd "$SCRIPT_DIR/kruda"
        env GOMAXPROCS="$GOMAXPROCS_VALUE" KRUDA_WORKERS="$KRUDA_WORKERS_VALUE" \
          KRUDA_READ_BUF_SIZE="$KRUDA_READ_BUF_SIZE_VALUE" \
          KRUDA_POOL_SIZE="$KRUDA_POOL_SIZE_VALUE" \
          PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" BENCH_ENABLE_PPROF="$BENCH_ENABLE_PPROF_VALUE" \
          BENCH_KRUDA_DB_DISPATCH="$BENCH_KRUDA_DB_DISPATCH_VALUE" \
          BENCH_KRUDA_CPU_DISPATCH="$BENCH_KRUDA_CPU_DISPATCH_VALUE" \
          DATABASE_URL="$KRUDA_DATABASE_URL_VALUE" \
          ./kruda-bench
      ) > "$log" 2>&1 &
      ;;
    fiber)
      (
        cd "$SCRIPT_DIR/fiber"
        env GOMAXPROCS="$GOMAXPROCS_VALUE" PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" \
          DATABASE_URL="$FIBER_DATABASE_URL_VALUE" ./fiber-bench
      ) > "$log" 2>&1 &
      ;;
    actix)
      (
        cd "$SCRIPT_DIR/actix"
        env PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" BENCH_ACTIX_WORKERS="$ACTIX_WORKERS_VALUE" \
          DATABASE_URL="$ACTIX_DATABASE_URL_VALUE" \
          ./target/release/actix-bench
      ) > "$log" 2>&1 &
      ;;
    *)
      echo "unknown framework: $fw" >&2
      exit 1
      ;;
  esac

  set_pid "$fw" "$!"
  wait_ready "$fw" "$port" "$log"
}

stop_server() {
  local fw="$1"
  local pid
  pid="$(pid_for "$fw")"
  if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  clear_pid "$fw"
}

wait_ready() {
  local fw="$1"
  local port="$2"
  local log="$3"
  local url="http://127.0.0.1:$port/plaintext-handler"

  for _ in $(seq 1 100); do
    if ! kill -0 "$(pid_for "$fw")" 2>/dev/null; then
      echo "$fw exited before it became ready on port $port" >&2
      cat "$log" >&2 || true
      exit 1
    fi
    if curl -fsS "$url" >/dev/null 2>&1; then
      return
    fi
    sleep 0.1
  done

  echo "$fw did not become ready on port $port" >&2
  cat "$log" >&2 || true
  exit 1
}

extract_summary() {
  local raw="$1"
  awk '
    function to_ms(v, num, unit) {
      num = v
      unit = v
      gsub(/[a-zA-Z]+/, "", num)
      gsub(/[0-9.]+/, "", unit)
      if (unit == "us") return num / 1000
      if (unit == "s") return num * 1000
      if (unit == "m") return num * 60000
      return num
    }
    $1 == "50%" { p50 = to_ms($2) }
    $1 == "90%" { p90 = to_ms($2) }
    $1 == "99%" { p99 = to_ms($2) }
    $1 == "Latency" && max == "" { max = to_ms($4) }
    /Requests\/sec:/ { rps = $2 }
    /Socket errors:/ {
      gsub(/,/, "")
      socket_errors = $4 + $6 + $8 + $10
    }
    /Non-2xx/ { non2xx = $NF }
    END {
      printf "%.2f,%.3f,%.3f,%.3f,%.3f,%d,%d", rps+0, p50+0, p90+0, p99+0, max+0, socket_errors+0, non2xx+0
    }
  ' "$raw"
}

run_wrk() {
  local fw="$1"
  local route="$2"
  local profile="$3"
  local wrk_args="$4"
  local round="$5"
  local port
  port="$(port_for "$fw")"
  local url="http://127.0.0.1:$port/$route"
  local raw="$RAW_DIR/$fw-$profile-$route-round-$round.txt"

  wrk --latency $wrk_args "$url" > "$raw" 2>&1
  local values
  values="$(extract_summary "$raw")"
  echo "$TIMESTAMP,$profile,$fw,$route,$round,$wrk_args,$values,$raw" >> "$SUMMARY_CSV"
  echo "| $profile | $fw | $route | $round | ${values//,/ | } | $raw |" >> "$SUMMARY_MD"
}

init_summary() {
  echo "timestamp_utc,profile,framework,route,round,wrk_args,rps,p50_ms,p90_ms,p99_ms,max_ms,socket_errors,non_2xx,raw_file" > "$SUMMARY_CSV"
  {
    echo "# Benchmark Summary"
    echo
    echo "Environment: \`$META_FILE\`"
    echo
    echo "| Profile | Framework | Route | Round | RPS | p50 ms | p90 ms | p99 ms | max ms | Socket errors | Non-2xx | Raw file |"
    echo "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|"
  } > "$SUMMARY_MD"
}

echo "== Building benchmark binaries =="
build_all
write_environment
init_summary

for fw in "${FRAMEWORKS[@]}"; do
  echo "== $fw =="
  start_server "$fw"
  for profile_def in "${PROFILES[@]}"; do
    profile="${profile_def%%:*}"
    wrk_args="${profile_def#*:}"
    for route in "${ROUTES[@]}"; do
      port="$(port_for "$fw")"
      echo "warmup: $fw $profile /$route"
      wrk --latency $wrk_args "http://127.0.0.1:$port/$route" \
        > "$RAW_DIR/$fw-$profile-$route-warmup.txt" 2>&1
      for round in $(seq 1 "$BENCH_ROUNDS_VALUE"); do
        echo "round $round: $fw $profile /$route"
        run_wrk "$fw" "$route" "$profile" "$wrk_args" "$round"
      done
    done
  done
  stop_server "$fw"
done

echo "summary: $SUMMARY_MD"
echo "raw: $RAW_DIR"
