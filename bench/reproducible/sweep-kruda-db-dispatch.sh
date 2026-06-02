#!/usr/bin/env bash
# Kruda-only DB dispatch sweep.
#
# This is candidate-discovery tooling, not cross-runtime benchmark evidence.
# It runs explicit DB routes across Kruda dispatch modes and writes an
# aggregate median summary from the per-run bench.sh summaries.
#
# Usage:
#   BENCH_ENABLE_DB=1 ./sweep-kruda-db-dispatch.sh
#   ./sweep-kruda-db-dispatch.sh db queries

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TIMESTAMP="$(date -u '+%Y%m%dT%H%M%SZ')"
RESULT_DIR="${RESULT_DIR:-"$SCRIPT_DIR/results/kruda-db-dispatch-sweep-$TIMESTAMP"}"
RUN_DIR="$RESULT_DIR/runs"
SUMMARY_CSV="$RESULT_DIR/dispatch-summary.csv"
SUMMARY_MD="$RESULT_DIR/summary.md"
META_FILE="$RESULT_DIR/environment.txt"

read -r -a MODES <<< "${BENCH_KRUDA_DB_DISPATCH_MODES:-takeover inline pool spawn}"
POOL_SIZE_VALUE="${KRUDA_POOL_SIZE:-64}"
BENCH_ROUNDS_VALUE="${BENCH_ROUNDS:-3}"
BENCH_DURATION_VALUE="${BENCH_DURATION:-10s}"
KRUDA_PORT_VALUE="${KRUDA_PORT:-3230}"

ROUTES=("$@")
if [ "${#ROUTES[@]}" -eq 0 ]; then
  ROUTES=(db queries fortunes updates)
fi

mkdir -p "$RUN_DIR"

write_environment() {
  {
    echo "timestamp_utc=$TIMESTAMP"
    echo "script_dir=$SCRIPT_DIR"
    echo "result_dir=$RESULT_DIR"
    echo "routes=${ROUTES[*]}"
    echo "modes=${MODES[*]}"
    echo "bench_rounds=$BENCH_ROUNDS_VALUE"
    echo "bench_duration=$BENCH_DURATION_VALUE"
    echo "kruda_port=$KRUDA_PORT_VALUE"
    echo "pool_size=$POOL_SIZE_VALUE"
    echo
    echo "== OS =="
    uname -a
    echo
    echo "== Toolchain =="
    go version
    wrk --version 2>&1 || true
  } > "$META_FILE"
}

init_summary() {
  echo "mode,route,profile,median_rps,median_p99_ms,max_socket_errors,max_non_2xx,run_dir" > "$SUMMARY_CSV"
  {
    echo "# Kruda DB Dispatch Sweep"
    echo
    echo "Environment: \`$META_FILE\`"
    echo
    echo "| Mode | Route | Profile | Median RPS | Median p99 ms | Max socket errors | Max non-2xx | Run dir |"
    echo "|---|---|---|---:|---:|---:|---:|---|"
  } > "$SUMMARY_MD"
}

append_aggregate() {
  local mode="$1"
  local route="$2"
  local run_dir="$3"
  local summary="$run_dir/summary.csv"

  for profile in latency throughput; do
    awk -F, -v mode="$mode" -v route="$route" -v profile="$profile" -v run_dir="$run_dir" '
      NR > 1 && $2 == profile {
        n++
        rps[n] = $7 + 0
        p99[n] = $10 + 0
        if (($12 + 0) > socket_errors) socket_errors = $12 + 0
        if (($13 + 0) > non_2xx) non_2xx = $13 + 0
      }
      function sort_values(a, count,   i, j, tmp) {
        for (i = 1; i <= count; i++) {
          for (j = i + 1; j <= count; j++) {
            if (a[i] > a[j]) {
              tmp = a[i]
              a[i] = a[j]
              a[j] = tmp
            }
          }
        }
      }
      function median(a, count) {
        if (count % 2 == 1) return a[(count + 1) / 2]
        return (a[count / 2] + a[count / 2 + 1]) / 2
      }
      END {
        if (n == 0) exit 1
        sort_values(rps, n)
        sort_values(p99, n)
        printf "%s,%s,%s,%.2f,%.3f,%d,%d,%s\n",
          mode, route, profile, median(rps, n), median(p99, n), socket_errors, non_2xx, run_dir
      }
    ' "$summary" >> "$SUMMARY_CSV"
  done
}

write_markdown_summary() {
  awk -F, '
    NR > 1 {
      printf "| %s | %s | %s | %.2f | %.3f | %d | %d | `%s` |\n",
        $1, $2, $3, $4, $5, $6, $7, $8
    }
  ' "$SUMMARY_CSV" >> "$SUMMARY_MD"
}

run_case() {
  local mode="$1"
  local route="$2"
  local run_dir="$RUN_DIR/$route-$mode"

  echo "== $route $mode =="
  if [ "$mode" = "pool" ]; then
    (
      cd "$SCRIPT_DIR"
      RESULT_DIR="$run_dir" \
        BENCH_FRAMEWORKS=kruda \
        BENCH_ENABLE_DB=1 \
        BENCH_KRUDA_DB_DISPATCH="$mode" \
        KRUDA_POOL_SIZE="$POOL_SIZE_VALUE" \
        BENCH_ROUNDS="$BENCH_ROUNDS_VALUE" \
        BENCH_DURATION="$BENCH_DURATION_VALUE" \
        KRUDA_PORT="$KRUDA_PORT_VALUE" \
        ./bench.sh "$route"
    )
  else
    (
      cd "$SCRIPT_DIR"
      RESULT_DIR="$run_dir" \
        BENCH_FRAMEWORKS=kruda \
        BENCH_ENABLE_DB=1 \
        BENCH_KRUDA_DB_DISPATCH="$mode" \
        BENCH_ROUNDS="$BENCH_ROUNDS_VALUE" \
        BENCH_DURATION="$BENCH_DURATION_VALUE" \
        KRUDA_PORT="$KRUDA_PORT_VALUE" \
        ./bench.sh "$route"
    )
  fi
  append_aggregate "$mode" "$route" "$run_dir"
}

write_environment
init_summary

for route in "${ROUTES[@]}"; do
  for mode in "${MODES[@]}"; do
    run_case "$mode" "$route"
  done
done

write_markdown_summary

echo "summary: $SUMMARY_MD"
echo "csv: $SUMMARY_CSV"
