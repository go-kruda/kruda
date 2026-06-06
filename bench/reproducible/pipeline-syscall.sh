#!/usr/bin/env bash
# Diagnostic syscall-count harness for HTTP/1.1 pipelined workloads.
#
# This script is intentionally separate from pipeline.sh. It uses strace -c,
# so the RPS and latency rows are intrusive diagnostic data. Use the syscall
# counts, especially requests_per_write_send, to evaluate whether a server is
# reducing write/send syscalls under pipelined request batches.

set -euo pipefail

export PATH="$PATH:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/pipeline-syscall-$TIMESTAMP"}"
RAW_DIR="$RESULT_DIR/raw"
SUMMARY_CSV="$RESULT_DIR/summary.csv"
SUMMARY_MD="$RESULT_DIR/summary.md"
META_FILE="$RESULT_DIR/environment.txt"
CLIENT_BIN="$SCRIPT_DIR/pipeline-client/pipeline-client"

GOMAXPROCS_VALUE="${GOMAXPROCS:-8}"
KRUDA_WORKERS_VALUE="${KRUDA_WORKERS:-4}"
KRUDA_READ_BUF_SIZE_VALUE="${KRUDA_READ_BUF_SIZE:-}"
KRUDA_POOL_SIZE_VALUE="${KRUDA_POOL_SIZE:-}"
BENCH_ENABLE_DB_VALUE="${BENCH_ENABLE_DB:-0}"
BENCH_KRUDA_DB_DISPATCH_VALUE="${BENCH_KRUDA_DB_DISPATCH:-takeover}"
KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS-kruda_stdjson}"
if [ "$KRUDA_GO_TAGS_VALUE" = "default" ] || [ "$KRUDA_GO_TAGS_VALUE" = "none" ]; then
  KRUDA_GO_TAGS_VALUE=""
fi
PIPELINE_DURATION_VALUE="${PIPELINE_DURATION:-10s}"
PIPELINE_WARMUP_VALUE="${PIPELINE_WARMUP:-3s}"
PIPELINE_TIMEOUT_VALUE="${PIPELINE_TIMEOUT:-5s}"
PROFILE_SUDO="${PROFILE_SUDO:-0}"
DATABASE_BASE_URL_VALUE="${BENCH_DATABASE_BASE_URL:-postgres://benchmarkdbuser:benchmarkdbpass@localhost:5432/hello_world}"
DATABASE_PGX_URL_VALUE="${DATABASE_BASE_URL_VALUE}?pool_max_conns=64&pool_min_conns=8"
KRUDA_DATABASE_URL_VALUE="${KRUDA_DATABASE_URL:-${DATABASE_URL:-$DATABASE_PGX_URL_VALUE}}"
ACTIX_DATABASE_URL_VALUE="${ACTIX_DATABASE_URL:-${DATABASE_URL:-$DATABASE_BASE_URL_VALUE}}"
STRACE_TRACE="${STRACE_TRACE:-read,readv,recvfrom,recvmsg,write,writev,sendto,sendmsg,epoll_wait,epoll_pwait,epoll_pwait2,epoll_ctl,futex}"

read -r -a FRAMEWORKS <<< "${BENCH_FRAMEWORKS:-kruda actix}"
read -r -a PIPELINE_PROFILES <<< "${PIPELINE_PROFILES:-baseline-c128-d1:128:1 pipeline-c128-d8:128:8 pipeline-c256-d8:256:8}"

if [ "$#" -gt 0 ]; then
  ROUTES=("$@")
else
  ROUTES=(plaintext-handler json-static json-serialize)
fi

SERVER_PID=""
TRACE_PID=""

mkdir -p "$RAW_DIR"

cleanup() {
  if [ -n "$TRACE_PID" ] && kill -0 "$TRACE_PID" >/dev/null 2>&1; then
    stop_trace
    wait "$TRACE_PID" >/dev/null 2>&1 || true
  fi
  TRACE_PID=""
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" >/dev/null 2>&1; then
    kill "$SERVER_PID" >/dev/null 2>&1 || true
    wait "$SERVER_PID" >/dev/null 2>&1 || true
  fi
  SERVER_PID=""
}
trap cleanup EXIT INT TERM

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

profile_prefix() {
  if [ "$PROFILE_SUDO" = "1" ]; then
    printf '%s\n' sudo -n
  fi
}

stop_trace() {
  if [ -z "$TRACE_PID" ]; then
    return 0
  fi
  if [ "$PROFILE_SUDO" = "1" ]; then
    sudo -n kill -INT "$TRACE_PID" >/dev/null 2>&1 || true
  else
    kill -INT "$TRACE_PID" >/dev/null 2>&1 || true
  fi
}

port_for() {
  case "$1" in
    kruda) echo "${KRUDA_PORT:-3300}" ;;
    actix) echo "${ACTIX_PORT:-3303}" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

bin_for() {
  case "$1" in
    kruda) echo "$SCRIPT_DIR/kruda/kruda-bench" ;;
    actix) echo "$SCRIPT_DIR/actix/target/release/actix-bench" ;;
    *) echo "unknown framework: $1" >&2; exit 1 ;;
  esac
}

build_all() {
  require_cmd go
  require_cmd curl
  require_cmd strace

  (cd "$SCRIPT_DIR/pipeline-client" && GOWORK=off go build -o "$CLIENT_BIN" .)

  local kruda_tag_args=()
  if [ -n "$KRUDA_GO_TAGS_VALUE" ]; then
    kruda_tag_args=(-tags "$KRUDA_GO_TAGS_VALUE")
  fi

  for fw in "${FRAMEWORKS[@]}"; do
    case "$fw" in
      kruda)
        (cd "$SCRIPT_DIR/kruda" && GOWORK=off go build "${kruda_tag_args[@]}" -o kruda-bench .)
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
    echo "result_dir=$RESULT_DIR"
    echo "workload=http1_pipelined_syscall_diagnostic"
    echo "profile_sudo=$PROFILE_SUDO"
    echo "strace_trace=$STRACE_TRACE"
    echo "bench_enable_db=$BENCH_ENABLE_DB_VALUE"
    echo "bench_kruda_db_dispatch=$BENCH_KRUDA_DB_DISPATCH_VALUE"
    if [ -n "${DATABASE_URL:-}" ]; then echo "database_url_common_override=1"; else echo "database_url_common_override=0"; fi
    if [ -n "${KRUDA_DATABASE_URL:-}" ]; then echo "kruda_database_url_override=1"; else echo "kruda_database_url_override=0"; fi
    if [ -n "${ACTIX_DATABASE_URL:-}" ]; then echo "actix_database_url_override=1"; else echo "actix_database_url_override=0"; fi
    echo "kruda_go_tags=${KRUDA_GO_TAGS_VALUE:-default}"
    echo "gomaxprocs=$GOMAXPROCS_VALUE"
    echo "kruda_workers=$KRUDA_WORKERS_VALUE"
    echo "kruda_read_buf_size=${KRUDA_READ_BUF_SIZE_VALUE:-default}"
    echo "kruda_pool_size=${KRUDA_POOL_SIZE_VALUE:-default}"
    echo "pipeline_duration=$PIPELINE_DURATION_VALUE"
    echo "pipeline_warmup=$PIPELINE_WARMUP_VALUE"
    echo "pipeline_timeout=$PIPELINE_TIMEOUT_VALUE"
    echo "pipeline_profiles=${PIPELINE_PROFILES[*]}"
    echo "frameworks=${FRAMEWORKS[*]}"
    echo "routes=${ROUTES[*]}"
    echo
    echo "== CPU =="
    if command -v lscpu >/dev/null 2>&1; then
      lscpu
    fi
    echo
    echo "== OS =="
    uname -a
    echo
    echo "== Kernel profiling policy =="
    if [ -r /proc/sys/kernel/yama/ptrace_scope ]; then
      echo "ptrace_scope=$(cat /proc/sys/kernel/yama/ptrace_scope)"
    fi
    echo
    echo "== Toolchain =="
    go version
    if command -v rustc >/dev/null 2>&1; then rustc --version; fi
    if command -v cargo >/dev/null 2>&1; then cargo --version; fi
    strace --version 2>&1 | head -1 || true
    "$CLIENT_BIN" -h 2>&1 | head -n 20 || true
  } > "$META_FILE"
}

start_server() {
  local fw="$1"
  local port="$2"
  local log="$3"
  local bin
  bin="$(bin_for "$fw")"
  cleanup

  case "$fw" in
    kruda)
      (
        cd "$SCRIPT_DIR/kruda"
        env GOMAXPROCS="$GOMAXPROCS_VALUE" KRUDA_WORKERS="$KRUDA_WORKERS_VALUE" \
          KRUDA_READ_BUF_SIZE="$KRUDA_READ_BUF_SIZE_VALUE" \
          KRUDA_POOL_SIZE="$KRUDA_POOL_SIZE_VALUE" \
          PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" \
          BENCH_KRUDA_DB_DISPATCH="$BENCH_KRUDA_DB_DISPATCH_VALUE" \
          DATABASE_URL="$KRUDA_DATABASE_URL_VALUE" \
          "$bin"
      ) > "$log" 2>&1 &
      ;;
    actix)
      (
        cd "$SCRIPT_DIR/actix"
        env PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" DATABASE_URL="$ACTIX_DATABASE_URL_VALUE" "$bin"
      ) > "$log" 2>&1 &
      ;;
  esac
  SERVER_PID="$!"
}

wait_ready() {
  local url="$1"
  local log="$2"
  for _ in $(seq 1 100); do
    if ! kill -0 "$SERVER_PID" >/dev/null 2>&1; then
      echo "server exited before ready: $url" >&2
      cat "$log" >&2 || true
      exit 1
    fi
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.05
  done
  echo "server not ready: $url" >&2
  cat "$log" >&2 || true
  exit 1
}

run_pipeline_client() {
  local url="$1"
  local connections="$2"
  local depth="$3"
  local duration="$4"
  local out="$5"
  "$CLIENT_BIN" \
    -url "$url" \
    -connections "$connections" \
    -depth "$depth" \
    -duration "$duration" \
    -warmup 0s \
    -timeout "$PIPELINE_TIMEOUT_VALUE" \
    > "$out" 2>&1
}

parse_profile() {
  local profile_def="$1"
  local name connections depth
  IFS=: read -r name connections depth <<< "$profile_def"
  if [ -z "$name" ] || [ -z "$connections" ] || [ -z "$depth" ]; then
    echo "bad PIPELINE_PROFILES entry: $profile_def" >&2
    echo "expected format: name:connections:depth" >&2
    exit 1
  fi
  echo "$name" "$connections" "$depth"
}

run_profile() {
  local fw="$1"
  local route="$2"
  local profile="$3"
  local connections="$4"
  local depth="$5"
  local port
  port="$(port_for "$fw")"
  local url="http://127.0.0.1:$port/$route"
  local prefix="${fw}__${profile}__${route}"
  local server_log="$RAW_DIR/server-$prefix.log"
  local warmup_raw="$RAW_DIR/$prefix-warmup.txt"
  local raw="$RAW_DIR/$prefix-pipeline.txt"
  local strace_raw="$RAW_DIR/$prefix-strace.txt"
  local attach_log="$RAW_DIR/$prefix-strace-attach.log"
  local status_file="$RAW_DIR/$prefix-status.txt"

  start_server "$fw" "$port" "$server_log"
  wait_ready "$url" "$server_log"
  run_pipeline_client "$url" "$connections" "$depth" "$PIPELINE_WARMUP_VALUE" "$warmup_raw"

  set +e
  $(profile_prefix) strace -f -c -e trace="$STRACE_TRACE" -p "$SERVER_PID" -o "$strace_raw" > "$attach_log" 2>&1 &
  TRACE_PID="$!"
  sleep 0.5
  run_pipeline_client "$url" "$connections" "$depth" "$PIPELINE_DURATION_VALUE" "$raw"
  local pipeline_status="$?"
  stop_trace
  wait "$TRACE_PID"
  local strace_status="$?"
  set -e

  {
    echo "pipeline_status=$pipeline_status"
    echo "strace_status=$strace_status"
  } > "$status_file"
  TRACE_PID=""
  cleanup
}

summarize() {
  python3 - "$RESULT_DIR" "$SUMMARY_CSV" "$SUMMARY_MD" <<'PY'
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
csv_path = pathlib.Path(sys.argv[2])
md_path = pathlib.Path(sys.argv[3])

def parse_pipeline(path):
    text = path.read_text(errors="replace")
    def find_float(label):
        m = re.search(rf"^{re.escape(label)}:\s+([0-9.]+)", text, re.M)
        return float(m.group(1)) if m else 0.0
    def find_int(label):
        m = re.search(rf"^{re.escape(label)}:\s+([0-9]+)", text, re.M)
        return int(m.group(1)) if m else 0
    return {
        "requests": find_int("Requests"),
        "rps": find_float("Requests/sec"),
        "p50_ms": find_float("Latency p50"),
        "p90_ms": find_float("Latency p90"),
        "p99_ms": find_float("Latency p99"),
        "max_ms": find_float("Latency max"),
        "socket_errors": find_int("Socket errors"),
        "non2xx": find_int("Non-2xx responses"),
    }

def parse_strace(path):
    calls = {}
    if not path.exists():
        return calls
    for line in path.read_text(errors="replace").splitlines():
        cols = line.split()
        if len(cols) >= 5 and cols[-1].replace("_", "").isalnum() and cols[3].isdigit():
            calls[cols[-1]] = int(cols[3])
    return calls

def sum_calls(calls, names):
    return sum(calls.get(name, 0) for name in names)

rows = []
for raw in sorted((root / "raw").glob("*-pipeline.txt")):
    stem = raw.name[:-len("-pipeline.txt")]
    fw, profile, route = stem.split("__", 2)
    data = parse_pipeline(raw)
    calls = parse_strace(root / "raw" / f"{stem}-strace.txt")
    read_calls = sum_calls(calls, ("read", "readv", "recvfrom", "recvmsg"))
    write_calls = sum_calls(calls, ("write", "writev", "sendto", "sendmsg"))
    epoll_wait_calls = sum_calls(calls, ("epoll_wait", "epoll_pwait", "epoll_pwait2"))
    requests = data["requests"]
    rows.append({
        "framework": fw,
        "profile": profile,
        "route": route,
        **data,
        "read_recv_calls": read_calls,
        "write_send_calls": write_calls,
        "requests_per_read_recv": (requests / read_calls) if read_calls else 0.0,
        "requests_per_write_send": (requests / write_calls) if write_calls else 0.0,
        "epoll_wait_calls": epoll_wait_calls,
        "epoll_ctl_calls": calls.get("epoll_ctl", 0),
        "futex_calls": calls.get("futex", 0),
        "strace_raw": str(root / "raw" / f"{stem}-strace.txt"),
        "pipeline_raw": str(raw),
    })

headers = [
    "framework", "profile", "route", "requests", "rps", "p50_ms", "p90_ms",
    "p99_ms", "max_ms", "socket_errors", "non2xx", "read_recv_calls",
    "write_send_calls", "requests_per_read_recv", "requests_per_write_send",
    "epoll_wait_calls", "epoll_ctl_calls", "futex_calls", "pipeline_raw",
    "strace_raw",
]
csv_path.write_text(",".join(headers) + "\n" + "\n".join(
    ",".join(str(r.get(h, "")) for h in headers) for r in rows
) + "\n")

lines = [
    "# HTTP/1.1 Pipelined Syscall Summary",
    "",
    "This is intrusive strace diagnostic evidence. Use syscall counts and requests-per-syscall ratios, not RPS or latency, for performance conclusions.",
    "",
    f"Environment: `{root / 'environment.txt'}`",
    "",
    "| Framework | Profile | Route | Requests | RPS | p99 ms | Socket errors | Non-2xx | read/recv | write/send | req/read | req/write | epoll wait | epoll_ctl | futex |",
    "|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
]
for r in rows:
    lines.append(
        f"| {r['framework']} | {r['profile']} | {r['route']} | {r['requests']} | "
        f"{r['rps']:.2f} | {r['p99_ms']:.3f} | {r['socket_errors']} | {r['non2xx']} | "
        f"{r['read_recv_calls']} | {r['write_send_calls']} | "
        f"{r['requests_per_read_recv']:.2f} | {r['requests_per_write_send']:.2f} | "
        f"{r['epoll_wait_calls']} | {r['epoll_ctl_calls']} | {r['futex_calls']} |"
    )
md_path.write_text("\n".join(lines) + "\n")

print(f"result_dir={root}")
for r in rows:
    print(
        f"{r['framework']} {r['profile']} {r['route']}: "
        f"requests_per_write_send={r['requests_per_write_send']:.2f} "
        f"write_send_calls={r['write_send_calls']} requests={r['requests']}"
    )
PY
}

echo "== Building pipelined syscall benchmark binaries =="
build_all
write_environment

for fw in "${FRAMEWORKS[@]}"; do
  for profile_def in "${PIPELINE_PROFILES[@]}"; do
    read -r profile connections depth < <(parse_profile "$profile_def")
    for route in "${ROUTES[@]}"; do
      echo "syscall: $fw $profile /$route connections=$connections depth=$depth"
      run_profile "$fw" "$route" "$profile" "$connections" "$depth"
    done
  done
done

summarize
