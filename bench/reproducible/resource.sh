#!/usr/bin/env bash
# Reproducible CPU/RAM resource benchmark: Kruda vs Fiber vs Actix.
#
# Default routes avoid PostgreSQL and measure handler CPU paths only:
#   /plaintext-handler, /json-static, /json-serialize
#
# Usage:
#   ./resource.sh                         # run default CPU-bound routes
#   ./resource.sh json-static             # run one route
#   BENCH_ENABLE_DB=1 ./resource.sh db    # opt in to DB routes

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/resource-$TIMESTAMP"}"
RAW_DIR="$RESULT_DIR/raw"
SUMMARY_CSV="$RESULT_DIR/resource-summary.csv"
AGGREGATED_CSV="$RESULT_DIR/resource-aggregated.csv"
SUMMARY_MD="$RESULT_DIR/summary.md"
META_FILE="$RESULT_DIR/environment.txt"

GOMAXPROCS_VALUE="${GOMAXPROCS:-8}"
KRUDA_WORKERS_VALUE="${KRUDA_WORKERS:-$GOMAXPROCS_VALUE}"
BENCH_ENABLE_DB_VALUE="${BENCH_ENABLE_DB:-0}"
BENCH_DURATION_VALUE="${BENCH_DURATION:-15s}"
RESOURCE_INTERVAL_VALUE="${RESOURCE_INTERVAL:-1}"
RESOURCE_MIN_CPU_SAMPLE_VALUE="${RESOURCE_MIN_CPU_SAMPLE:-50}"
DATABASE_URL_VALUE="${DATABASE_URL:-postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world?pool_max_conns=64&pool_min_conns=8}"

FRAMEWORKS=(kruda fiber actix)
KRUDA_PID=""
FIBER_PID=""
ACTIX_PID=""
PIDSTAT_PID=""

PROFILES=(
  "latency:-t4 -c128 -d$BENCH_DURATION_VALUE"
  "throughput:-t4 -c256 -d$BENCH_DURATION_VALUE"
)

if [ "$#" -gt 0 ]; then
  ROUTES=("$@")
else
  ROUTES=(plaintext-handler json-static json-serialize)
fi

mkdir -p "$RAW_DIR"

cleanup() {
  stop_pidstat
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
  require_cmd cargo
  require_cmd wrk
  require_cmd curl
  require_cmd pidstat

  (cd "$SCRIPT_DIR/kruda" && GOWORK=off go build -tags kruda_stdjson -o kruda-bench .)
  (cd "$SCRIPT_DIR/fiber" && GOWORK=off go build -o fiber-bench .)
  (cd "$SCRIPT_DIR/actix" && cargo build --release)
}

write_environment() {
  {
    echo "timestamp_utc=$TIMESTAMP"
    echo "script_dir=$SCRIPT_DIR"
    echo "result_dir=$RESULT_DIR"
    echo "bench_enable_db=$BENCH_ENABLE_DB_VALUE"
    echo "gomaxprocs=$GOMAXPROCS_VALUE"
    echo "kruda_workers=$KRUDA_WORKERS_VALUE"
    echo "bench_duration=$BENCH_DURATION_VALUE"
    echo "resource_interval=$RESOURCE_INTERVAL_VALUE"
    echo "resource_min_cpu_sample=$RESOURCE_MIN_CPU_SAMPLE_VALUE"
    echo "kruda_port=$(port_for kruda)"
    echo "fiber_port=$(port_for fiber)"
    echo "actix_port=$(port_for actix)"
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
    echo "== Memory =="
    if command -v free >/dev/null 2>&1; then
      free -h
    elif command -v vm_stat >/dev/null 2>&1; then
      vm_stat
    fi
    echo
    echo "== OS =="
    uname -a
    echo
    echo "== Toolchain =="
    go version
    rustc --version
    cargo --version
    wrk --version 2>&1 || true
    pidstat -V 2>&1 || true
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
          PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" DATABASE_URL="$DATABASE_URL_VALUE" \
          ./kruda-bench
      ) > "$log" 2>&1 &
      ;;
    fiber)
      (
        cd "$SCRIPT_DIR/fiber"
        env GOMAXPROCS="$GOMAXPROCS_VALUE" PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" \
          DATABASE_URL="$DATABASE_URL_VALUE" ./fiber-bench
      ) > "$log" 2>&1 &
      ;;
    actix)
      (
        cd "$SCRIPT_DIR/actix"
        env PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" DATABASE_URL="$DATABASE_URL_VALUE" \
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

stop_pidstat() {
  if [ -n "$PIDSTAT_PID" ] && kill -0 "$PIDSTAT_PID" 2>/dev/null; then
    kill "$PIDSTAT_PID" 2>/dev/null || true
    wait "$PIDSTAT_PID" 2>/dev/null || true
  fi
  PIDSTAT_PID=""
}

extract_wrk_summary() {
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

extract_pidstat_summary() {
  local raw="$1"
  local server_pid="$2"
  awk -v server_pid="$server_pid" -v min_cpu="$RESOURCE_MIN_CPU_SAMPLE_VALUE" '
    /%usr/ && /%CPU/ {
      mode = "cpu"
      next
    }
    /minflt\/s/ && /RSS/ {
      mode = "mem"
      next
    }
    /cswch\/s/ && /nvcswch\/s/ {
      mode = "ctx"
      next
    }
    /^[0-9]/ && $4 == server_pid {
      if (mode == "cpu") {
        cpu = $9 + 0
        all_cpu_sum += cpu
        all_cpu_count++
        if (cpu > all_cpu_max) all_cpu_max = cpu
        if (cpu >= min_cpu) {
          active_cpu_sum += cpu
          active_cpu_count++
          if (cpu > active_cpu_max) active_cpu_max = cpu
        }
      } else if (mode == "mem") {
        rss = $8 + 0
        if (rss > 0) {
          rss_sum += rss
          rss_count++
          if (rss > rss_max) rss_max = rss
        }
      } else if (mode == "ctx") {
        cswch_sum += $5 + 0
        nvcswch_sum += $6 + 0
        ctx_count++
      }
    }
    END {
      if (active_cpu_count > 0) {
        avg_cpu = active_cpu_sum / active_cpu_count
        max_cpu = active_cpu_max
      } else if (all_cpu_count > 0) {
        avg_cpu = all_cpu_sum / all_cpu_count
        max_cpu = all_cpu_max
      }
      if (rss_count > 0) avg_rss = rss_sum / rss_count
      if (ctx_count > 0) {
        avg_cswch = cswch_sum / ctx_count
        avg_nvcswch = nvcswch_sum / ctx_count
      }
      printf "%.2f,%.2f,%.0f,%.0f,%.2f,%.2f", avg_cpu+0, max_cpu+0, rss_max+0, avg_rss+0, avg_cswch+0, avg_nvcswch+0
    }
  ' "$raw"
}

run_resource() {
  local fw="$1"
  local route="$2"
  local profile="$3"
  local wrk_args="$4"
  local port
  port="$(port_for "$fw")"
  local server_pid
  server_pid="$(pid_for "$fw")"
  local url="http://127.0.0.1:$port/$route"
  local warmup_raw="$RAW_DIR/$fw-$profile-$route-warmup.txt"
  local wrk_raw="$RAW_DIR/$fw-$profile-$route-wrk.txt"
  local pidstat_raw="$RAW_DIR/$fw-$profile-$route-pidstat.txt"
  local wrk_rel="${wrk_raw#"$RESULT_DIR/"}"
  local pidstat_rel="${pidstat_raw#"$RESULT_DIR/"}"

  wrk --latency $wrk_args "$url" > "$warmup_raw" 2>&1

  pidstat -u -r -w -p "$server_pid" "$RESOURCE_INTERVAL_VALUE" > "$pidstat_raw" 2>&1 &
  PIDSTAT_PID="$!"
  sleep 0.2
  wrk --latency $wrk_args "$url" > "$wrk_raw" 2>&1
  stop_pidstat

  local wrk_values
  wrk_values="$(extract_wrk_summary "$wrk_raw")"
  local resource_values
  resource_values="$(extract_pidstat_summary "$pidstat_raw" "$server_pid")"

  local rps p50 p90 p99 max_ms socket_errors non2xx avg_cpu max_cpu max_rss_kb avg_rss_kb cswch nvcswch
  IFS=',' read -r rps p50 p90 p99 max_ms socket_errors non2xx <<< "$wrk_values"
  IFS=',' read -r avg_cpu max_cpu max_rss_kb avg_rss_kb cswch nvcswch <<< "$resource_values"

  local max_rss_mb avg_rss_mb rps_per_core
  max_rss_mb="$(awk -v kb="$max_rss_kb" 'BEGIN { printf "%.2f", kb / 1024 }')"
  avg_rss_mb="$(awk -v kb="$avg_rss_kb" 'BEGIN { printf "%.2f", kb / 1024 }')"
  rps_per_core="$(awk -v rps="$rps" -v cpu="$avg_cpu" 'BEGIN { if (cpu > 0) printf "%.0f", rps / (cpu / 100); else printf "0" }')"

  echo "$TIMESTAMP,$profile,$fw,$route,$wrk_args,$wrk_values,$avg_cpu,$max_cpu,$max_rss_kb,$avg_rss_kb,$max_rss_mb,$avg_rss_mb,$rps_per_core,$cswch,$nvcswch,$wrk_rel,$pidstat_rel" >> "$SUMMARY_CSV"
  echo "$TIMESTAMP,$profile,$fw,$route,$rps,$p99,$socket_errors,$non2xx,$avg_cpu,$max_cpu,$max_rss_mb,$avg_rss_mb,$rps_per_core,$cswch,$nvcswch" >> "$AGGREGATED_CSV"
  echo "| $profile | $fw | $route | $rps | $p99 | $avg_cpu | $max_cpu | $max_rss_mb | $avg_rss_mb | $rps_per_core | $socket_errors | $non2xx | $wrk_rel | $pidstat_rel |" >> "$SUMMARY_MD"
}

init_summary() {
  echo "timestamp_utc,profile,framework,route,wrk_args,rps,p50_ms,p90_ms,p99_ms,max_ms,socket_errors,non_2xx,avg_cpu_pct,max_cpu_pct,max_rss_kb,avg_rss_kb,max_rss_mb,avg_rss_mb,rps_per_core,cswch_s,nvcswch_s,raw_wrk,pidstat_log" > "$SUMMARY_CSV"
  echo "timestamp_utc,profile,framework,route,rps,p99_ms,socket_errors,non_2xx,avg_cpu_pct,max_cpu_pct,max_rss_mb,avg_rss_mb,rps_per_core,cswch_s,nvcswch_s" > "$AGGREGATED_CSV"
  {
    echo "# Resource Benchmark Summary"
    echo
    echo "Environment: \`$META_FILE\`"
    echo
    echo "CPU percentage is process CPU across cores. RSS is resident memory from pidstat. RPS/core is RPS divided by avg CPU cores consumed."
    echo
    echo "| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max CPU % | Max RSS MB | Avg RSS MB | RPS/core | Socket errors | Non-2xx | Raw wrk | pidstat |"
    echo "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|"
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
      echo "resource: $fw $profile /$route"
      run_resource "$fw" "$route" "$profile" "$wrk_args"
    done
  done
  stop_server "$fw"
done

echo "summary: $SUMMARY_MD"
echo "aggregated: $AGGREGATED_CSV"
echo "raw: $RAW_DIR"
