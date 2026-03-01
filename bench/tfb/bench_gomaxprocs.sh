#!/usr/bin/env bash
# Quick GOMAXPROCS comparison: 4 configs × 7 tests × 1 round
# Usage: bash bench_gomaxprocs.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
DB="postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world?sslmode=disable"
PORT=8080
THREADS=4; CONNS=256; DURATION=8s

T_NAMES=(json plaintext db queries fortunes cached updates)
T_PATHS=(/json /plaintext /db "/queries?queries=20" /fortunes "/cached-queries?count=20" "/updates?queries=20")

mkdir -p "$RESULTS_DIR"
PID=""

cleanup() { [ -n "$PID" ] && kill "$PID" 2>/dev/null || true; docker compose -f "$SCRIPT_DIR/docker-compose.yml" down -v 2>/dev/null || true; }
trap cleanup EXIT

wait_up()   { for i in $(seq 1 30); do curl -sf "http://localhost:$PORT/json" >/dev/null 2>&1 && return 0; sleep 0.5; done; echo "FAIL: server not up"; exit 1; }
wait_free() { for i in $(seq 1 20); do ss -tlnp | grep -q ":$PORT " || return 0; sleep 0.5; done; fuser -k "${PORT}/tcp" 2>/dev/null || true; sleep 1; }

run_config() {
    local label=$1 bin=$2; shift 2
    # remaining args are env vars: KEY=VAL ...
    echo ""
    echo "========== $label =========="
    env GOGC=400 DATABASE_URL="$DB" PORT=$PORT "$@" "$bin" &
    PID=$!
    wait_up
    for p in /json /db "/updates?queries=20"; do wrk -t2 -c32 -d3s "http://localhost:$PORT$p" >/dev/null 2>&1 || true; done
    sleep 1

    printf "%-12s %12s\n" "Test" "req/s"
    echo "------------------------"
    for i in 0 1 2 3 4 5 6; do
        rps=$(wrk -t"$THREADS" -c"$CONNS" -d"$DURATION" "http://localhost:$PORT${T_PATHS[$i]}" 2>&1 | grep "Requests/sec:" | awk '{print $2}')
        printf "%-12s %12s\n" "${T_NAMES[$i]}" "$rps"
    done

    kill "$PID" 2>/dev/null; wait "$PID" 2>/dev/null || true; PID=""
    wait_free; sleep 2
}

echo "Starting PostgreSQL..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" down -v 2>/dev/null || true
docker compose -f "$SCRIPT_DIR/docker-compose.yml" up -d 2>&1 | tail -2
for i in $(seq 1 20); do docker compose -f "$SCRIPT_DIR/docker-compose.yml" exec -T postgres pg_isready -U benchmarkdbuser -d hello_world >/dev/null 2>&1 && break; sleep 1; done
sleep 1

echo "Building..."
(cd "$ROOT_DIR/frameworks/Go/kruda/src" && go build -gcflags="all=-B" -ldflags="-s -w" -o "$RESULTS_DIR/kruda-bench" .) 2>&1
(cd "$SCRIPT_DIR/fiber" && go build -gcflags="all=-B" -ldflags="-s -w" -o "$RESULTS_DIR/fiber-bench" .) 2>&1
echo "Done. Running 4 configs: -t$THREADS -c$CONNS -d$DURATION"

NCPU=$(nproc)
KRUDA="$RESULTS_DIR/kruda-bench"
FIBER="$RESULTS_DIR/fiber-bench"

run_config "Kruda turbo  GOMAXPROCS=1/child  (current)" "$KRUDA" \
    KRUDA_TURBO=1 KRUDA_WORKERS=$NCPU

run_config "Kruda turbo  GOMAXPROCS=default" "$KRUDA" \
    KRUDA_TURBO=1 KRUDA_WORKERS=$NCPU KRUDA_NO_GOMAXPROCS=1

run_config "Fiber prefork GOMAXPROCS=1/child (fair)" "$FIBER" \
    FIBER_PREFORK=1 FIBER_WORKERS=$NCPU GOMAXPROCS=1

run_config "Fiber prefork GOMAXPROCS=default (TFB)" "$FIBER" \
    FIBER_PREFORK=1 FIBER_WORKERS=$NCPU

echo ""
echo "Done."
