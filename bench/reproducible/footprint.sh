#!/usr/bin/env bash
# Measure Kruda runtime footprint: binary size, startup time, idle RSS.
#
# Profiling-only, not a benchmark claim source. Numbers are dominated by the Go
# runtime and the app's own deps (e.g. pgx); use this to spot framework-side
# footprint regressions, not as a cross-runtime claim.
#
# Usage:
#   ./footprint.sh                 # default (Sonic) build for the runtime metrics
#   KRUDA_GO_TAGS=kruda_stdjson ./footprint.sh
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
KRUDA_DIR="$SCRIPT_DIR/kruda"
PORT_VALUE="${PORT:-3400}"
TAGS="${KRUDA_GO_TAGS-}"

filesize() { stat -c%s "$1" 2>/dev/null || stat -f%z "$1"; }
mb() { awk "BEGIN{printf \"%.2f\", $1/1048576}"; }
out() { printf '%-26s %s\n' "$1" "$2"; }

cd "$KRUDA_DIR"

# --- binary size, both builds ---
GOWORK=off go build -tags kruda_stdjson -o /tmp/kruda-fp-stdjson .
GOWORK=off go build -o /tmp/kruda-fp-default .
out "binary stdjson (MB)" "$(mb "$(filesize /tmp/kruda-fp-stdjson)")"
out "binary default/Sonic (MB)" "$(mb "$(filesize /tmp/kruda-fp-default)")"

# --- startup + idle RSS (CPU route only; no DB needed) ---
BIN=/tmp/kruda-fp-stdjson
if [ -n "$TAGS" ] && [ "$TAGS" != "kruda_stdjson" ]; then BIN=/tmp/kruda-fp-default; fi

env PORT="$PORT_VALUE" BENCH_ENABLE_DB=0 "$BIN" >/tmp/kruda-fp.log 2>&1 &
PID=$!
trap 'kill "$PID" 2>/dev/null || true' EXIT

t0=$(date +%s%N)
ready=0
for _ in $(seq 1 1000); do
  if curl -fsS "http://127.0.0.1:$PORT_VALUE/plaintext-handler" >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.002
done
t1=$(date +%s%N)
if [ "$ready" != 1 ]; then echo "server did not become ready" >&2; cat /tmp/kruda-fp.log >&2; exit 1; fi
out "startup to first 200 (ms)" "$(( (t1 - t0) / 1000000 ))"

sleep 0.5
rss_kb="$(ps -o rss= -p "$PID" | tr -d ' ')"
out "idle RSS (MB)" "$(mb "$(( rss_kb * 1024 ))")"
out "build measured (runtime)" "$(basename "$BIN")"
