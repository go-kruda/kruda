#!/usr/bin/env bash
set -euo pipefail
export PATH=$HOME/go/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH

cd "$(dirname "$0")"

DUR=${1:-10}
CONNS=${2:-256}
THREADS=$(nproc)

echo "=== Building servers ==="
go build -o /tmp/bench-compare .

echo "=== Starting fasthttp on :18080 ==="
/tmp/bench-compare -duration 600s -conns 1 -warmup 0s &>/dev/null &
BENCH_PID=$!
sleep 2

# The binary starts both servers on random ports — not useful for wrk.
# Kill it and use a different approach: start servers via go run with fixed ports.
kill $BENCH_PID 2>/dev/null || true
wait $BENCH_PID 2>/dev/null || true

echo "=== Done ==="
