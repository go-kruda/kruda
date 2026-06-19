#!/usr/bin/env bash
# D3 observability enabled-path A/B.
#
# Measures the per-request cost of observability.Enable on Kruda's Wing fast-lane
# routes (plaintext / JSON-serialize / static-JSON) by load-testing one kruda-bench
# binary in two arms:
#   off — BENCH_ENABLE_OBS unset (baseline)
#   on  — BENCH_ENABLE_OBS=1 with OTEL_TRACES_EXPORTER=none OTEL_METRICS_EXPORTER=none
#         (spans + RED metrics are still created/recorded; only the network export
#          is a no-op, so the number is the per-request hot-path overhead a user
#          pays regardless of the telemetry backend).
#
# Requires wrk. Run from bench/reproducible: ./obs-ab.sh
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PORT="${KRUDA_PORT:-3478}"
ROUNDS="${ROUNDS:-5}"
DUR="${DUR:-10s}"
CONNS="${CONNS:-128}"
THREADS="${THREADS:-4}"
ROUTES=(/ /json /json-static)
OUT="${OUT:-/tmp/obs-ab-out}"
mkdir -p "$OUT"

command -v wrk >/dev/null || { echo "wrk required"; exit 1; }
echo "[build] kruda-bench (GOWORK=off, kruda_stdjson)"
( cd "$SCRIPT_DIR/kruda" && GOWORK=off go build -tags kruda_stdjson -o /tmp/kruda-bench-obs . ) || exit 1

run_arm() {
  local arm="$1"; shift
  env "$@" PORT="$PORT" /tmp/kruda-bench-obs >"$OUT/server-$arm.log" 2>&1 &
  local pid=$!
  local up=0
  for _ in $(seq 1 100); do curl -s -o /dev/null "http://127.0.0.1:$PORT/" && { up=1; break; }; sleep 0.1; done
  [ "$up" = 1 ] || { echo "[$arm] server did not come up"; kill "$pid" 2>/dev/null; return 1; }
  for r in $(seq 1 "$ROUNDS"); do
    for route in "${ROUTES[@]}"; do
      local tag; tag=$(echo "$route" | tr '/' '_'); [ "$tag" = "_" ] && tag="_root"
      local raw="$OUT/$arm-r$r$tag.txt"
      wrk -t"$THREADS" -c"$CONNS" -d"$DUR" --latency "http://127.0.0.1:$PORT$route" >"$raw" 2>&1
      local rps p99
      rps=$(awk '/Requests\/sec/{print $2}' "$raw")
      p99=$(awk '/^[[:space:]]*99%/{print $2}' "$raw")
      echo "$arm,$route,$r,$rps,$p99" >> "$OUT/summary.csv"
    done
  done
  kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
}

echo "arm,route,round,rps,p99" > "$OUT/summary.csv"
run_arm off
run_arm on BENCH_ENABLE_OBS=1 OTEL_TRACES_EXPORTER=none OTEL_METRICS_EXPORTER=none
echo "=== summary.csv ==="
cat "$OUT/summary.csv"
echo "OUT=$OUT"
