#!/usr/bin/env bash
# Phase 3A syscall/cycle evidence for CPU-bound fair handler routes.
#
# This script compares Kruda and Actix under the same wrk load while collecting:
#   - wrk --latency output
#   - perf stat counters attached to the server process
#   - strace -c syscall summaries attached to the server process
#
# It is diagnostic evidence only. It does not benchmark DB/fortunes routes.

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/syscall-$TIMESTAMP"}"
RAW_DIR="$RESULT_DIR/raw"
SUMMARY_CSV="$RESULT_DIR/summary.csv"
SUMMARY_MD="$RESULT_DIR/summary.md"
META_FILE="$RESULT_DIR/environment.txt"

FRAMEWORKS=(kruda actix)
if [ "$#" -gt 0 ]; then
  ROUTES=("$@")
else
  ROUTES=(plaintext-handler json-static json-serialize)
fi

PROFILE_DURATION="${PROFILE_DURATION:-10s}"
WARMUP_DURATION="${WARMUP_DURATION:-5s}"
WRK_THREADS="${WRK_THREADS:-4}"
WRK_CONNECTIONS="${WRK_CONNECTIONS:-256}"
GOMAXPROCS_VALUE="${GOMAXPROCS:-8}"
KRUDA_WORKERS_VALUE="${KRUDA_WORKERS:-4}"
KRUDA_READ_BUF_SIZE_VALUE="${KRUDA_READ_BUF_SIZE:-4096}"
BENCH_ENABLE_DB_VALUE="${BENCH_ENABLE_DB:-0}"
KRUDA_GO_TAGS_VALUE="${KRUDA_GO_TAGS-kruda_stdjson}"
if [ "$KRUDA_GO_TAGS_VALUE" = "default" ] || [ "$KRUDA_GO_TAGS_VALUE" = "none" ]; then
  KRUDA_GO_TAGS_VALUE=""
fi
PERF_EVENTS="${PERF_EVENTS:-task-clock,cycles,instructions,context-switches,cpu-migrations,page-faults}"
PROFILE_SUDO="${PROFILE_SUDO:-0}"

SERVER_PID=""
TRACE_PID=""

mkdir -p "$RAW_DIR"

duration_seconds() {
  case "$1" in
    *s) printf '%s\n' "${1%s}" ;;
    *) printf '%s\n' "$1" ;;
  esac
}

PROFILE_SECONDS="$(duration_seconds "$PROFILE_DURATION")"

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
    kruda) echo "${KRUDA_PORT:-13000}" ;;
    actix) echo "${ACTIX_PORT:-13003}" ;;
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
  require_cmd cargo
  require_cmd wrk
  require_cmd curl
  require_cmd perf
  require_cmd strace

  local kruda_tag_args=()
  if [ -n "$KRUDA_GO_TAGS_VALUE" ]; then
    kruda_tag_args=(-tags "$KRUDA_GO_TAGS_VALUE")
  fi

  (cd "$SCRIPT_DIR/kruda" && GOWORK=off go build "${kruda_tag_args[@]}" -o kruda-bench .)
  (cd "$SCRIPT_DIR/actix" && cargo build --release)
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
    echo "profile_duration=$PROFILE_DURATION"
    echo "warmup_duration=$WARMUP_DURATION"
    echo "wrk_threads=$WRK_THREADS"
    echo "wrk_connections=$WRK_CONNECTIONS"
    echo "perf_events=$PERF_EVENTS"
    echo "profile_sudo=$PROFILE_SUDO"
    echo "gomaxprocs=$GOMAXPROCS_VALUE"
    echo "kruda_workers=$KRUDA_WORKERS_VALUE"
    echo "kruda_read_buf_size=$KRUDA_READ_BUF_SIZE_VALUE"
    echo "bench_enable_db=$BENCH_ENABLE_DB_VALUE"
    echo "kruda_go_tags=${KRUDA_GO_TAGS_VALUE:-default}"
    echo "routes=${ROUTES[*]}"
    echo "frameworks=${FRAMEWORKS[*]}"
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
    if [ -r /proc/sys/kernel/perf_event_paranoid ]; then
      echo "perf_event_paranoid=$(cat /proc/sys/kernel/perf_event_paranoid)"
    fi
    if [ -r /proc/sys/kernel/yama/ptrace_scope ]; then
      echo "ptrace_scope=$(cat /proc/sys/kernel/yama/ptrace_scope)"
    fi
    echo
    echo "== Toolchain =="
    go version
    rustc --version
    cargo --version
    wrk --version 2>&1 || true
    perf --version 2>&1 || true
    strace --version 2>&1 | head -1 || true
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
          KRUDA_READ_BUF_SIZE="$KRUDA_READ_BUF_SIZE_VALUE" PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" \
          "$bin"
      ) > "$log" 2>&1 &
      ;;
    actix)
      (
        cd "$SCRIPT_DIR/actix"
        env PORT="$port" BENCH_ENABLE_DB="$BENCH_ENABLE_DB_VALUE" "$bin"
      ) > "$log" 2>&1 &
      ;;
  esac
  SERVER_PID="$!"
}

wait_ready() {
  local url="$1"
  for _ in $(seq 1 100); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.05
  done
  echo "server not ready: $url" >&2
  return 1
}

run_wrk() {
  local url="$1"
  local duration="$2"
  local out="$3"
  wrk -t"$WRK_THREADS" -c"$WRK_CONNECTIONS" -d"$duration" --latency "$url" > "$out"
}

run_perf_stat() {
  local fw="$1"
  local route="$2"
  local port
  port="$(port_for "$fw")"
  local url="http://127.0.0.1:$port/$route"
  local prefix="$fw-$route-perf"

  start_server "$fw" "$port" "$RAW_DIR/server-$prefix.log"
  wait_ready "$url"
  run_wrk "$url" "$WARMUP_DURATION" "$RAW_DIR/$prefix-warmup.txt"

  set +e
  $(profile_prefix) perf stat -x, -e "$PERF_EVENTS" -p "$SERVER_PID" -o "$RAW_DIR/$prefix-perf.csv" -- sleep "$PROFILE_SECONDS" &
  local perf_pid="$!"
  sleep 0.3
  run_wrk "$url" "$PROFILE_DURATION" "$RAW_DIR/$prefix-wrk.txt"
  local wrk_status="$?"
  wait "$perf_pid"
  local perf_status="$?"
  set -e

  {
    echo "wrk_status=$wrk_status"
    echo "perf_status=$perf_status"
  } > "$RAW_DIR/$prefix-status.txt"
  cleanup
}

run_strace_count() {
  local fw="$1"
  local route="$2"
  local port
  port="$(port_for "$fw")"
  local url="http://127.0.0.1:$port/$route"
  local prefix="$fw-$route-strace"

  start_server "$fw" "$port" "$RAW_DIR/server-$prefix.log"
  wait_ready "$url"
  run_wrk "$url" "$WARMUP_DURATION" "$RAW_DIR/$prefix-warmup.txt"

  set +e
  $(profile_prefix) strace -f -c -p "$SERVER_PID" -o "$RAW_DIR/$prefix-strace.txt" > "$RAW_DIR/$prefix-strace-attach.log" 2>&1 &
  TRACE_PID="$!"
  sleep 0.5
  run_wrk "$url" "$PROFILE_DURATION" "$RAW_DIR/$prefix-wrk.txt"
  local wrk_status="$?"
  stop_trace
  wait "$TRACE_PID"
  local strace_status="$?"
  set -e

  {
    echo "wrk_status=$wrk_status"
    echo "strace_status=$strace_status"
  } > "$RAW_DIR/$prefix-status.txt"
  TRACE_PID=""
  cleanup
}

summarize() {
  python3 - "$RESULT_DIR" "$SUMMARY_CSV" "$SUMMARY_MD" <<'PY'
import pathlib
import re
import statistics
import sys

root = pathlib.Path(sys.argv[1])
csv_path = pathlib.Path(sys.argv[2])
md_path = pathlib.Path(sys.argv[3])

def parse_wrk(path):
    text = path.read_text(errors="replace")
    def latency_pct(pct):
        m = re.search(rf"\n\s*{pct}%\s+([0-9.]+[a-z]+)", text)
        return m.group(1) if m else ""
    latency_line = re.search(r"Latency\s+[^\n]*?\s+([0-9.]+[a-z]+)\s+[0-9.]+%", text)
    errors = re.search(r"Socket errors: connect (\d+), read (\d+), write (\d+), timeout (\d+)", text)
    non2xx = re.search(r"Non-2xx or 3xx responses: (\d+)", text)
    rps = re.search(r"Requests/sec:\s+([0-9.]+)", text)
    return {
        "rps": float(rps.group(1)) if rps else 0.0,
        "p50": latency_pct("50"),
        "p90": latency_pct("90"),
        "p99": latency_pct("99"),
        "max": latency_line.group(1) if latency_line else "",
        "socket_errors": sum(map(int, errors.groups())) if errors else 0,
        "non2xx": int(non2xx.group(1)) if non2xx else 0,
    }

def parse_perf(path):
    metrics = {}
    if not path.exists():
        return metrics
    for line in path.read_text(errors="replace").splitlines():
        if not line or line.startswith("#"):
            continue
        parts = [p.strip() for p in line.split(",")]
        if len(parts) >= 3 and parts[0] and parts[2]:
            metrics[parts[2]] = parts[0]
    return metrics

def parse_strace(path):
    calls = {}
    if not path.exists():
        return calls
    for line in path.read_text(errors="replace").splitlines():
        cols = line.split()
        if len(cols) >= 5 and cols[-1].isidentifier() and cols[3].isdigit():
            calls[cols[-1]] = cols[3]
    return calls

def sum_calls(calls, names):
    total = 0
    seen = False
    for name in names:
        if name in calls:
            seen = True
            total += int(calls[name])
    return str(total) if seen else ""

rows = []
for wrk in sorted((root / "raw").glob("*-wrk.txt")):
    stem = wrk.name[:-len("-wrk.txt")]
    fw, rest = stem.split("-", 1)
    route, kind = rest.rsplit("-", 1)
    data = parse_wrk(wrk)
    perf = parse_perf(root / "raw" / f"{fw}-{route}-perf-perf.csv") if kind == "perf" else {}
    strace = parse_strace(root / "raw" / f"{fw}-{route}-strace-strace.txt") if kind == "strace" else {}
    rows.append({
        "framework": fw,
        "route": route,
        "kind": kind,
        **data,
        "cycles": perf.get("cycles", ""),
        "instructions": perf.get("instructions", ""),
        "task_clock": perf.get("task-clock", ""),
        "context_switches": perf.get("context-switches", ""),
        "read_recv_calls": sum_calls(strace, ("read", "readv", "recvfrom", "recvmsg")),
        "write_send_calls": sum_calls(strace, ("write", "writev", "sendto", "sendmsg")),
        "epoll_wait_calls": sum_calls(strace, ("epoll_wait", "epoll_pwait", "epoll_pwait2")),
        "epoll_ctl_calls": strace.get("epoll_ctl", ""),
        "futex_calls": strace.get("futex", ""),
    })

headers = [
    "framework", "route", "kind", "rps", "p50", "p90", "p99", "max",
    "socket_errors", "non2xx", "cycles", "instructions", "task_clock",
    "context_switches", "read_recv_calls", "write_send_calls", "epoll_wait_calls",
    "epoll_ctl_calls", "futex_calls",
]
csv_path.write_text(",".join(headers) + "\n" + "\n".join(
    ",".join(str(r.get(h, "")) for h in headers) for r in rows
) + "\n")

lines = [
    "# Syscall Profile Summary",
    "",
    "Perf rows are the comparable wrk pass. Strace rows are intrusive syscall-count diagnostics; use their syscall counts, not their RPS or latency, for performance conclusions. A strace status of 130 is expected because the harness stops strace with SIGINT after the measured wrk run.",
    "",
    "| Framework | Route | Kind | RPS | p99 | Socket errors | Non-2xx | read/recv | write/send | epoll wait | epoll_ctl | futex | cycles | instructions | context switches |",
    "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|",
]
for r in rows:
    lines.append(
        f"| {r['framework']} | {r['route']} | {r['kind']} | {r['rps']:.2f} | {r['p99']} | "
        f"{r['socket_errors']} | {r['non2xx']} | {r['read_recv_calls']} | {r['write_send_calls']} | "
        f"{r['epoll_wait_calls']} | {r['epoll_ctl_calls']} | {r['futex_calls']} | "
        f"{r['cycles']} | {r['instructions']} | {r['context_switches']} |"
    )
md_path.write_text("\n".join(lines) + "\n")

print(f"result_dir={root}")
for route in sorted({r["route"] for r in rows}):
    perf_rows = [r for r in rows if r["route"] == route and r["kind"] == "perf"]
    if len(perf_rows) >= 2:
        by_fw = {r["framework"]: r for r in perf_rows}
        if "kruda" in by_fw and "actix" in by_fw and by_fw["actix"]["rps"]:
            delta = (by_fw["kruda"]["rps"] / by_fw["actix"]["rps"] - 1.0) * 100.0
            print(f"{route}: kruda_perf_rps={by_fw['kruda']['rps']:.2f} actix_perf_rps={by_fw['actix']['rps']:.2f} delta={delta:+.2f}%")
PY
}

build_all
write_environment

for route in "${ROUTES[@]}"; do
  for fw in "${FRAMEWORKS[@]}"; do
    run_perf_stat "$fw" "$route"
    run_strace_count "$fw" "$route"
  done
done

summarize
