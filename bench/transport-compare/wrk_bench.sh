#!/usr/bin/env bash
# Wing vs fasthttp wrk benchmark
# Usage: ./wrk_bench.sh [duration_secs] [connections]
set -euo pipefail
export PATH=$HOME/go/bin:/usr/local/go/bin:/usr/bin:/bin:/usr/sbin:/sbin:$PATH

DUR=${1:-10}
CONNS=${2:-256}
THREADS=$(nproc)
DIR="$(cd "$(dirname "$0")" && pwd)"

FPID="" ; WPID=""
cleanup() { [[ -n "$FPID" ]] && kill $FPID 2>/dev/null; [[ -n "$WPID" ]] && kill $WPID 2>/dev/null; wait 2>/dev/null; }
trap cleanup EXIT

echo "Building..."
cd "$DIR"
go build -tags fasthttp_server -o /tmp/srv_fast ./cmd/fast
go build -tags wing_server  -o /tmp/srv_wing ./cmd/wing
echo "Build OK"

/tmp/srv_fast &
FPID=$!
/tmp/srv_wing &
WPID=$!
sleep 2

curl -sf http://127.0.0.1:18080/plaintext >/dev/null && echo "fasthttp :18080 OK" || { echo "fasthttp FAIL"; exit 1; }
curl -sf http://127.0.0.1:18081/plaintext >/dev/null && echo "wing :18081 OK" || { echo "wing FAIL"; exit 1; }

echo ""
echo "=== System ==="
echo "CPU: $(lscpu | grep 'Model name' | sed 's/.*: *//')"
echo "Cores: $(nproc)  |  Kernel: $(uname -r)  |  Go: $(go version | awk '{print $3}')"
echo ""

echo "Warmup (3s)..."
wrk -t$THREADS -c$CONNS -d3s http://127.0.0.1:18080/plaintext >/dev/null 2>&1
wrk -t$THREADS -c$CONNS -d3s http://127.0.0.1:18081/plaintext >/dev/null 2>&1

for TEST in plaintext json; do
    echo ""
    echo "━━━ $TEST — ${DUR}s, ${CONNS}c, ${THREADS}t ━━━"
    echo "[fasthttp]"
    wrk -t$THREADS -c$CONNS -d${DUR}s http://127.0.0.1:18080/$TEST
    sleep 1
    echo "[wing]"
    wrk -t$THREADS -c$CONNS -d${DUR}s http://127.0.0.1:18081/$TEST
    sleep 1
done

echo ""
echo "Done!"
