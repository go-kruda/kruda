#!/usr/bin/env bash
set -euo pipefail

# TFB Local Benchmark: Kruda vs Fiber (SEQUENTIAL, dual-mode)
# Runs single-process mode.
# Bash 3.2 compatible (no associative arrays).

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$SCRIPT_DIR/results"
THREADS=${THREADS:-4}; CONNS=${CONNS:-128}; DURATION=${DURATION:-10s}; WARMUP=${WARMUP:-5s}; ROUNDS=${ROUNDS:-3}; PORT=8080
# Multi-process mode needs higher concurrency to saturate all children.
# Default: -t8 -c512 for multi (64 conns/child @ 8 children).
MULTI_THREADS=${MULTI_THREADS:-8}; MULTI_CONNS=${MULTI_CONNS:-512}
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
DATABASE_URL="postgres://benchmarkdbuser:benchmarkdbpass@localhost:5433/hello_world?sslmode=disable"
mkdir -p "$RESULTS_DIR"
log()  { echo -e "${CYAN}[bench]${NC} $*"; }
ok()   { echo -e "${GREEN}[  OK ]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
SERVER_PID=""
cleanup() {
    log "Cleaning up..."
    if [ -n "$SERVER_PID" ]; then kill "$SERVER_PID" 2>/dev/null || true; fi
    docker compose -f "$SCRIPT_DIR/docker-compose.yml" down -v 2>/dev/null || true
}
trap cleanup EXIT
wait_for_port() {
    local port=$1 name=$2
    for i in $(seq 1 30); do
        if curl -sf "http://localhost:$port/plaintext" >/dev/null 2>&1; then
            ok "$name ready on :$port"; return 0
        fi; sleep 0.5
    done
    fail "$name failed to start on :$port"; return 1
}
wait_port_free() {
    local port=$1
    for i in $(seq 1 30); do
        if ! ss -tlnp | grep -q ":$port "; then return 0; fi
        sleep 0.5
    done
    # Force kill anything still on the port
    fuser -k "${port}/tcp" 2>/dev/null || true
    sleep 1
    return 0
}
wait_for_pg() {
    for i in $(seq 1 30); do
        if docker compose -f "$SCRIPT_DIR/docker-compose.yml" exec -T postgres \
            pg_isready -U benchmarkdbuser -d hello_world >/dev/null 2>&1; then
            return 0
        fi; sleep 1
    done; return 1
}
run_wrk() {
    local url=$1 outfile=$2 t=${3:-$THREADS} c=${4:-$CONNS}
    wrk -t"$t" -c"$c" -d"$WARMUP" "$url" >/dev/null 2>&1 || true
    wrk -t"$t" -c"$c" -d"$DURATION" "$url" 2>&1 | tee "$outfile"
}
extract_rps() { grep "Requests/sec:" "$1" | awk '{print $2}'; }
median3() { printf '%s\n' "$1" "$2" "$3" | sort -g | sed -n '2p'; }
# warmup_server warms up all 7 TFB endpoints on the given port.
# Light pass for all endpoints, heavy pass for DB-intensive ones.
warmup_server() {
    local port=$1
    for path in /json /plaintext /db "/queries?queries=20" /fortunes "/cached-queries?count=20" "/updates?queries=20"; do
        wrk -t2 -c64 -d3s "http://localhost:$port$path" >/dev/null 2>&1 || true
    done
    for path in /db "/queries?queries=20" "/updates?queries=20"; do
        wrk -t4 -c128 -d5s "http://localhost:$port$path" >/dev/null 2>&1 || true
    done
}

# Run rounds of wrk for a given server+test, store median in MEDIAN_RESULT
# Args: $1=server_name $2=test_name $3=test_path $4=threads(opt) $5=conns(opt)
MEDIAN_RESULT=""
run_test() {
    local server_name=$1 test_name=$2 test_path=$3 t=${4:-$THREADS} c=${5:-$CONNS}
    echo -e "${BOLD}--- $server_name: $test_name (t=$t c=$c) ---${NC}"
    local r1="" r2="" r3=""
    for r in 1 2 3; do
        log "Round $r/$ROUNDS"
        run_wrk "http://localhost:$PORT$test_path" "$RESULTS_DIR/${server_name}-${test_name}-r${r}.txt" "$t" "$c"
        local rps
        rps=$(extract_rps "$RESULTS_DIR/${server_name}-${test_name}-r${r}.txt")
        log "  $server_name $test_name r$r: $rps"
        eval "r${r}=\$rps"
    done
    MEDIAN_RESULT=$(median3 "$r1" "$r2" "$r3")
    ok "$server_name $test_name median: $MEDIAN_RESULT"
    echo ""
}

# ---------------------------------------------------------------------------
# print_results: compare two result arrays and print table
# Args: $1=mode_label, arrays K_MED[] and F_MED[] must be populated
# ---------------------------------------------------------------------------
print_results() {
    local mode_label=$1
    echo ""
    echo -e "${BOLD}================================================================${NC}"
    echo -e "${BOLD}  RESULTS — Kruda vs Fiber ($mode_label)${NC}"
    echo -e "${BOLD}================================================================${NC}"
    echo ""

    printf "${BOLD}%-14s %12s %12s %10s %8s${NC}\n" "Test" "Kruda" "Fiber" "Diff%" "Winner"
    echo "--------------------------------------------------------------"

    KRUDA_WINS=0; FIBER_WINS=0; TIES=0

    for i in 0 1 2 3 4 5 6; do
        k="${K_MED[$i]}"
        f="${F_MED[$i]}"
        name="${T_NAMES[$i]}"

        if [ -z "$k" ] || [ -z "$f" ]; then
            printf "%-14s %12s %12s %10s %8s\n" "$name" "${k:-N/A}" "${f:-N/A}" "N/A" "N/A"
            continue
        fi

        diff=$(awk "BEGIN { if ($f > 0) printf \"%.1f\", (($k - $f) / $f) * 100; else print \"0.0\" }")
        k_int=$(printf "%.0f" "$k")
        f_int=$(printf "%.0f" "$f")
        if [ "$k_int" -gt "$f_int" ]; then
            winner="${GREEN}KRUDA${NC}"; KRUDA_WINS=$((KRUDA_WINS + 1))
            diff_str="${GREEN}+${diff}%${NC}"
        elif [ "$f_int" -gt "$k_int" ]; then
            winner="${RED}FIBER${NC}"; FIBER_WINS=$((FIBER_WINS + 1))
            diff_str="${RED}${diff}%${NC}"
        else
            winner="${YELLOW}TIE${NC}"; TIES=$((TIES + 1))
            diff_str="${YELLOW}${diff}%${NC}"
        fi

        printf "%-14s %12s %12s   " "$name" "$k" "$f"
        echo -e "${diff_str}   ${winner}"
    done

    echo ""
    echo "--------------------------------------------------------------"
    echo -e "${BOLD}Score: Kruda ${KRUDA_WINS} — Fiber ${FIBER_WINS}  (Ties: ${TIES})${NC}"
    echo ""

    if [ "$KRUDA_WINS" -eq 7 ]; then
        echo -e "${GREEN}${BOLD}🏆 KRUDA WINS ALL 7 TESTS! PERFECT SCORE! 🏆${NC}"
    elif [ "$KRUDA_WINS" -gt "$FIBER_WINS" ]; then
        echo -e "${GREEN}${BOLD}✅ Kruda wins overall ($KRUDA_WINS/$((KRUDA_WINS+FIBER_WINS+TIES)))${NC}"
    elif [ "$FIBER_WINS" -gt "$KRUDA_WINS" ]; then
        echo -e "${RED}${BOLD}❌ Fiber wins overall ($FIBER_WINS/$((KRUDA_WINS+FIBER_WINS+TIES)))${NC}"
    else
        echo -e "${YELLOW}${BOLD}🤝 It's a draw!${NC}"
    fi
    echo ""
}

# ---------------------------------------------------------------------------
# run_mode: run all 7 tests for both Kruda and Fiber in a given mode
# Args: $1=mode ("single" or "multi")
# Populates K_MED[] and F_MED[]
# ---------------------------------------------------------------------------
run_mode() {
    local mode=$1
    local kruda_env="" fiber_env=""
    local mode_label=""
    local t="$THREADS" c="$CONNS"

    if [ "$mode" = "multi" ]; then
        mode_label="Multi-Process (Prefork)"
        kruda_env=""
        fiber_env="FIBER_PREFORK=1"
        t="$MULTI_THREADS"; c="$MULTI_CONNS"
    else
        mode_label="Single-Process"
        kruda_env=""
        fiber_env=""
    fi

    K_MED=(); F_MED=()

    echo ""
    echo -e "${BOLD}================================================================${NC}"
    echo -e "${BOLD}  Kruda vs Fiber — $mode_label${NC}"
    echo -e "${BOLD}  Threads=$t Conns=$c Duration=$DURATION Rounds=$ROUNDS${NC}"
    echo -e "${BOLD}================================================================${NC}"
    echo ""

    # ===== KRUDA =====
    echo -e "${BOLD}========== KRUDA ($mode_label) ==========${NC}"
    log "Starting Kruda on :$PORT..."
    if [ "$mode" = "multi" ]; then
        GOGC=${GOGC:-400} DATABASE_URL="$DATABASE_URL" PORT="$PORT" "$RESULTS_DIR/kruda-bench" &
    else
        GOGC=${GOGC:-400} DATABASE_URL="$DATABASE_URL" PORT="$PORT" "$RESULTS_DIR/kruda-bench" &
    fi
    SERVER_PID=$!
    wait_for_port "$PORT" "Kruda" || exit 1

    log "Warming up Kruda..."
    warmup_server "$PORT"
    ok "Kruda warmed up"; sleep 2

    for i in 0 1 2 3 4 5 6; do
        run_test "kruda-${mode}" "${T_NAMES[$i]}" "${T_PATHS[$i]}" "$t" "$c"
        K_MED[$i]="$MEDIAN_RESULT"
    done

    log "Stopping Kruda..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    SERVER_PID=""
    wait_port_free "$PORT"
    ok "Kruda stopped, port $PORT free"; sleep 3

    # ===== FIBER =====
    echo -e "${BOLD}========== FIBER ($mode_label) ==========${NC}"
    log "Starting Fiber on :$PORT..."
    if [ "$mode" = "multi" ]; then
        GOGC=${GOGC:-400} FIBER_PREFORK=1 FIBER_WORKERS="$(nproc)" DATABASE_URL="$DATABASE_URL" PORT="$PORT" "$RESULTS_DIR/fiber-bench" &
    else
        GOGC=${GOGC:-400} DATABASE_URL="$DATABASE_URL" PORT="$PORT" "$RESULTS_DIR/fiber-bench" &
    fi
    SERVER_PID=$!
    wait_for_port "$PORT" "Fiber" || exit 1

    log "Warming up Fiber..."
    warmup_server "$PORT"
    ok "Fiber warmed up"; sleep 2

    for i in 0 1 2 3 4 5 6; do
        run_test "fiber-${mode}" "${T_NAMES[$i]}" "${T_PATHS[$i]}" "$t" "$c"
        F_MED[$i]="$MEDIAN_RESULT"
    done

    log "Stopping Fiber..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
    SERVER_PID=""
    wait_port_free "$PORT"
    ok "Fiber stopped"; sleep 3

    print_results "$mode_label"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

# Test definitions — name|path
T_NAMES=(json plaintext db queries fortunes cached updates)
T_PATHS=(/json /plaintext /db "/queries?queries=20" /fortunes "/cached-queries?count=20" "/updates?queries=20")

# Parse mode argument: "single", "multi", or "both" (default)
MODE="${1:-both}"
case "$MODE" in
    single|multi|both) ;;
    *) echo "Usage: $0 [single|multi|both]"; exit 1 ;;
esac

log "Checking prerequisites..."
command -v docker >/dev/null || { fail "docker not found"; exit 1; }
command -v go     >/dev/null || { fail "go not found"; exit 1; }
command -v wrk    >/dev/null || { fail "wrk not found (brew install wrk)"; exit 1; }
ok "All prerequisites found"

log "Starting PostgreSQL..."
docker compose -f "$SCRIPT_DIR/docker-compose.yml" down -v 2>/dev/null || true
docker compose -f "$SCRIPT_DIR/docker-compose.yml" up -d 2>&1
log "Waiting for PostgreSQL..."
wait_for_pg && ok "PostgreSQL ready" || { fail "PostgreSQL failed"; exit 1; }
sleep 2

log "Building Kruda..."
(cd "$ROOT_DIR/frameworks/Go/kruda/src" && go build -gcflags="all=-B" -ldflags="-s -w" -o "$RESULTS_DIR/kruda-bench" .) 2>&1
ok "Kruda built"
log "Building Fiber..."
(cd "$SCRIPT_DIR/fiber" && go build -gcflags="all=-B" -ldflags="-s -w" -o "$RESULTS_DIR/fiber-bench" .) 2>&1
ok "Fiber built"

# Run requested mode(s)
if [ "$MODE" = "single" ] || [ "$MODE" = "both" ]; then
    run_mode "single"
fi
if [ "$MODE" = "multi" ] || [ "$MODE" = "both" ]; then
    run_mode "multi"
fi

log "Benchmark complete."
