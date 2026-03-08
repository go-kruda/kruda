#!/usr/bin/env bash
# Linux multi-core scaling benchmark: Kruda turbo vs Fiber prefork
# Run on Linux (bare metal or WSL2) for meaningful results.
# Usage: ./scaling_linux.sh [port]
set -euo pipefail

PORT=${1:-8080}
CONNS=256
DURATION=10s
WARMUP=5s
RESULTS_DIR="$(dirname "$0")/results/scaling"
mkdir -p "$RESULTS_DIR"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

log()  { echo -e "${CYAN}[scale]${NC} $*"; }
ok()   { echo -e "${GREEN}[  OK ]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*" >&2; }

# Detect available cores
MAX_CORES=$(nproc)
log "Detected $MAX_CORES CPU cores"

# Build core list: 1, 2, 4, 8, ... up to MAX_CORES
CORE_LIST=(1)
c=2
while [ "$c" -le "$MAX_CORES" ]; do
    CORE_LIST+=("$c")
    c=$((c * 2))
done
# Always include max if not already there
if [ "${CORE_LIST[-1]}" -ne "$MAX_CORES" ]; then
    CORE_LIST+=("$MAX_CORES")
fi

command -v wrk  >/dev/null || { fail "wrk not found. Install: apt install wrk"; exit 1; }
command -v go   >/dev/null || { fail "go not found"; exit 1; }

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

log "Building Kruda..."
(cd "$ROOT_DIR/frameworks/Go/kruda/src" && \
    go build -ldflags="-s -w" -o "$RESULTS_DIR/kruda-bench" .) 2>&1
ok "Kruda built"

log "Building Fiber..."
(cd "$ROOT_DIR/bench/tfb/fiber" && \
    go build -ldflags="-s -w" -o "$RESULTS_DIR/fiber-bench" .) 2>&1
ok "Fiber built"

SERVER_PID=""
cleanup() {
    [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

wait_ready() {
    for i in $(seq 1 30); do
        curl -sf "http://localhost:$PORT/plaintext" >/dev/null 2>&1 && return 0
        sleep 0.3
    done
    fail "Server not ready"; return 1
}

wait_free() {
    for i in $(seq 1 20); do
        ! ss -tlnp | grep -q ":$PORT " && return 0
        sleep 0.5
    done
    fail "Port $PORT still in use"; return 1
}

run_wrk() {
    local cores=$1 url=$2 out=$3
    local threads=$((cores < 4 ? cores : 4))
    wrk -t"$threads" -c"$CONNS" -d"$WARMUP" "$url" >/dev/null 2>&1 || true
    wrk -t"$threads" -c"$CONNS" -d"$DURATION" "$url" 2>&1 | tee "$out"
}

extract_rps() { grep "Requests/sec:" "$1" | awk '{print $2}'; }

# CSV output
CSV="$RESULTS_DIR/scaling_$(date +%Y%m%d_%H%M%S).csv"
echo "framework,mode,cores,rps,p99_ms" > "$CSV"

echo ""
echo -e "${BOLD}================================================================${NC}"
echo -e "${BOLD}  Multi-Core Scaling: Kruda Turbo vs Fiber Prefork${NC}"
echo -e "${BOLD}  Conns=$CONNS  Duration=$DURATION  URL=/plaintext${NC}"
echo -e "${BOLD}================================================================${NC}"
printf "\n${BOLD}%-10s %8s %12s %12s %8s${NC}\n" "Cores" "" "Kruda Turbo" "Fiber Prefork" "Winner"
echo "------------------------------------------------------------"

declare -a KRUDA_RPS=()
declare -a FIBER_RPS=()

for cores in "${CORE_LIST[@]}"; do
    # --- Kruda turbo ---
    log "Kruda turbo: $cores workers..."
    GOGC=off KRUDA_WORKERS="$cores" PORT="$PORT" \
        "$RESULTS_DIR/kruda-bench" &
    SERVER_PID=$!
    wait_ready
    out="$RESULTS_DIR/kruda_${cores}c.txt"
    run_wrk "$cores" "http://localhost:$PORT/plaintext" "$out" >/dev/null
    k_rps=$(extract_rps "$out")
    kill "$SERVER_PID" 2>/dev/null; wait "$SERVER_PID" 2>/dev/null || true
    SERVER_PID=""
    wait_free; sleep 1

    # --- Fiber prefork ---
    log "Fiber prefork: $cores workers..."
    GOGC=off FIBER_PREFORK=1 FIBER_WORKERS="$cores" PORT="$PORT" \
        "$RESULTS_DIR/fiber-bench" &
    SERVER_PID=$!
    wait_ready
    out="$RESULTS_DIR/fiber_prefork_${cores}c.txt"
    run_wrk "$cores" "http://localhost:$PORT/plaintext" "$out" >/dev/null
    f_rps=$(extract_rps "$out")
    kill "$SERVER_PID" 2>/dev/null; wait "$SERVER_PID" 2>/dev/null || true
    SERVER_PID=""
    wait_free; sleep 1

    KRUDA_RPS+=("$k_rps")
    FIBER_RPS+=("$f_rps")

    # Winner
    k_int=$(printf "%.0f" "${k_rps:-0}")
    f_int=$(printf "%.0f" "${f_rps:-0}")
    if [ "$k_int" -gt "$f_int" ]; then
        diff=$(awk "BEGIN { printf \"%.1f\", (($k_int - $f_int) / ($f_int > 0 ? $f_int : 1)) * 100 }")
        winner="${GREEN}KRUDA +${diff}%${NC}"
    elif [ "$f_int" -gt "$k_int" ]; then
        diff=$(awk "BEGIN { printf \"%.1f\", (($f_int - $k_int) / ($k_int > 0 ? $k_int : 1)) * 100 }")
        winner="${RED}FIBER +${diff}%${NC}"
    else
        winner="${YELLOW}TIE${NC}"
    fi

    printf "%-10s %8s %12s %12s   " "$cores" "" "${k_rps:-N/A}" "${f_rps:-N/A}"
    echo -e "$winner"

    echo "kruda,turbo,$cores,${k_rps:-0},0" >> "$CSV"
    echo "fiber,prefork,$cores,${f_rps:-0},0" >> "$CSV"
done

echo ""
ok "Results saved to $CSV"
log "Plot with: python3 -c \""
cat << 'EOF'
import csv, sys
data = list(csv.DictReader(open(sys.argv[1])))
k = [(int(r['cores']), float(r['rps'])) for r in data if r['framework']=='kruda']
f = [(int(r['cores']), float(r['rps'])) for r in data if r['framework']=='fiber']
print('Cores | Kruda      | Fiber      | Delta')
for (kc,kr),(fc,fr) in zip(k,f):
    d = (kr-fr)/fr*100 if fr else 0
    print(f'{kc:5} | {kr:10.0f} | {fr:10.0f} | {d:+.1f}%')
EOF
log "\""
