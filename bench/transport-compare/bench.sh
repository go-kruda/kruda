#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────────────────
# Kruda Transport Benchmark — Wing vs fasthttp (wrk edition)
#
# Prerequisites:
#   - wrk: apt install wrk  OR  brew install wrk
#   - Go 1.24+
#   - Linux (kernel 5.1+), macOS, or Windows
#
# Usage:
#   ./bench.sh                    # default: 10s, 256 conns, pipeline 16
#   ./bench.sh -d 30 -c 512       # 30s duration, 512 connections
#   ./bench.sh -t plaintext       # only plaintext test
#
# ──────────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ── Defaults ──────────────────────────────────────────────────────────────────
DURATION=10
CONNS=256
THREADS=$(nproc 2>/dev/null || echo 4)
PIPELINE=16
TESTS="plaintext json"
WARMUP=3
WING_WORKERS=0  # 0 = NumCPU

# ── Parse args ────────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        -d|--duration) DURATION=$2; shift 2 ;;
        -c|--conns)    CONNS=$2;    shift 2 ;;
        -t|--test)     TESTS=$2;    shift 2 ;;
        -w|--workers)  WING_WORKERS=$2; shift 2 ;;
        -p|--pipeline) PIPELINE=$2; shift 2 ;;
        --warmup)      WARMUP=$2;   shift 2 ;;
        -h|--help)
            echo "Usage: $0 [-d duration] [-c conns] [-t test] [-w workers] [-p pipeline]"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ── Colors ────────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

# ── Checks ────────────────────────────────────────────────────────────────────
check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo -e "${RED}Error: $1 not found. Install with: $2${NC}"
        exit 1
    fi
}

check_cmd wrk "apt install wrk"
check_cmd go "https://go.dev/dl/"

OS=$(uname)
case "$OS" in
    Linux|Darwin|MINGW*|MSYS*|CYGWIN*) ;;
    *)
        echo -e "${RED}Error: Wing requires Linux, macOS, or Windows${NC}"
        exit 1
        ;;
esac

# ── Build benchmark servers ───────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
KRUDA_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR=$(mktemp -d)
FAST_PID=""
WING_PID=""
kill_servers() {
    [[ -n "$FAST_PID" ]] && kill "$FAST_PID" 2>/dev/null || true
    [[ -n "$WING_PID" ]] && kill "$WING_PID" 2>/dev/null || true
    wait 2>/dev/null || true
}
trap 'rm -rf "$BUILD_DIR"; kill_servers' EXIT

echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║  Kruda Transport Benchmark — Wing vs fasthttp           ║${NC}"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
echo

# Build server binaries
echo -e "${CYAN}Building benchmark servers...${NC}"

# fasthttp server
cat > "$BUILD_DIR/fasthttp_server.go" << 'GOEOF'
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"context"
	"time"

	kruda "github.com/go-kruda/kruda"
)

var jsonResp = []byte(`{"message":"Hello, World!"}`)
var textResp = []byte("Hello, World!")

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	app := kruda.New(kruda.FastHTTP())
	app.Get("/plaintext", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "text/plain")
		return c.SendBytes(textResp)
	})
	app.Get("/json", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "application/json")
		return c.SendBytes(jsonResp)
	})
	app.Post("/echo", func(c *kruda.Ctx) error {
		body, _ := c.BodyBytes()
		return c.SendBytes(body)
	})
	app.Compile()

	go func() {
		log.Printf("fasthttp listening on %s", *addr)
		if err := app.Listen(*addr); err != nil {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app.Shutdown(ctx)
}
GOEOF

# Wing server
cat > "$BUILD_DIR/wing_server.go" << 'GOEOF'
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"context"
	"time"

	kruda "github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport/wing"
)

var jsonResp = []byte(`{"message":"Hello, World!"}`)
var textResp = []byte("Hello, World!")

func main() {
	addr := flag.String("addr", ":8081", "listen address")
	workers := flag.Int("workers", 0, "Wing workers (0=NumCPU)")
	flag.Parse()

	tr := wing.New(wing.Config{
		Workers:     *workers,
		RingSize:    4096,
		ReadBufSize: 16384,
	})

	app := kruda.New(kruda.WithTransport(tr))
	app.Get("/plaintext", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "text/plain")
		return c.SendBytes(textResp)
	})
	app.Get("/json", func(c *kruda.Ctx) error {
		c.SetHeader("Content-Type", "application/json")
		return c.SendBytes(jsonResp)
	})
	app.Post("/echo", func(c *kruda.Ctx) error {
		body, _ := c.BodyBytes()
		return c.SendBytes(body)
	})
	app.Compile()

	go func() {
		log.Printf("Wing listening on %s (workers=%d)", *addr, *workers)
		if err := app.Listen(*addr); err != nil {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app.Shutdown(ctx)
}
GOEOF

# Create temporary go.mod for building
cat > "$BUILD_DIR/go.mod" << MODEOF
module bench-transport

go 1.24.0

require (
	github.com/go-kruda/kruda v0.0.0
	github.com/go-kruda/kruda/transport/wing v0.0.0
)

replace (
	github.com/go-kruda/kruda => $KRUDA_ROOT
	github.com/go-kruda/kruda/transport/wing => $KRUDA_ROOT/transport/wing
)
MODEOF

cd "$BUILD_DIR"
go mod tidy
go build -o fasthttp_server fasthttp_server.go
go build -o wing_server wing_server.go
echo -e "${GREEN}  ✓ Builds successful${NC}"

# ── Server management ─────────────────────────────────────────────────────────
FAST_PORT=18080
WING_PORT=18081

"$BUILD_DIR/fasthttp_server" -addr ":$FAST_PORT" &
FAST_PID=$!

"$BUILD_DIR/wing_server" -addr ":$WING_PORT" -workers "$WING_WORKERS" &
WING_PID=$!

# Wait for servers
sleep 1
for port in $FAST_PORT $WING_PORT; do
    for i in $(seq 1 50); do
        if nc -z 127.0.0.1 "$port" 2>/dev/null; then
            break
        fi
        sleep 0.1
    done
done

echo -e "${GREEN}  ✓ fasthttp on :$FAST_PORT (PID $FAST_PID)${NC}"
echo -e "${GREEN}  ✓ Wing on :$WING_PORT (PID $WING_PID)${NC}"

# ── wrk pipeline script ──────────────────────────────────────────────────────
WRK_SCRIPT="$BUILD_DIR/pipeline.lua"
cat > "$WRK_SCRIPT" << 'LUA'
init = function(args)
    local method = args[1] or "GET"
    local path = args[2] or "/"
    local pipeline = tonumber(args[3]) or 16

    local req = wrk.format(method, path)
    local reqs = ""
    for i = 1, pipeline do
        reqs = reqs .. req
    end
    wrk.init = function() end
    wrk.request = function() return reqs end
end

request = function()
    return wrk.format("GET", "/")
end
LUA

# ── Run benchmarks ────────────────────────────────────────────────────────────
declare -A RESULTS_FAST
declare -A RESULTS_WING

run_wrk() {
    local name=$1 port=$2 method=$3 path=$4

    wrk -t "$THREADS" -c "$CONNS" -d "${DURATION}s" \
        -s "$WRK_SCRIPT" \
        -- "$method" "$path" "$PIPELINE" \
        "http://127.0.0.1:$port$path" 2>&1
}

for test in $TESTS; do
    case "$test" in
        plaintext) METHOD="GET";  URL_PATH="/plaintext" ;;
        json)      METHOD="GET";  URL_PATH="/json" ;;
        echo)      METHOD="POST"; URL_PATH="/echo" ;;
        *) echo "Unknown test: $test"; continue ;;
    esac

    echo
    echo -e "${BOLD}━━━ $test ($METHOD $URL_PATH) ━━━${NC}"

    # Warmup
    if [[ $WARMUP -gt 0 ]]; then
        echo -e "${YELLOW}  Warming up ($WARMUP s)...${NC}"
        wrk -t "$THREADS" -c "$CONNS" -d "${WARMUP}s" "http://127.0.0.1:$FAST_PORT$URL_PATH" >/dev/null 2>&1 || true
        wrk -t "$THREADS" -c "$CONNS" -d "${WARMUP}s" "http://127.0.0.1:$WING_PORT$URL_PATH" >/dev/null 2>&1 || true
    fi

    # fasthttp
    echo -e "${BLUE}  [fasthttp] Running ${DURATION}s, ${CONNS} connections, pipeline ${PIPELINE}...${NC}"
    FAST_OUT=$(wrk -t "$THREADS" -c "$CONNS" -d "${DURATION}s" \
        -s "$WRK_SCRIPT" \
        "http://127.0.0.1:$FAST_PORT$URL_PATH" \
        -- "$METHOD" "$URL_PATH" "$PIPELINE" 2>&1) || true
    echo "$FAST_OUT"
    RESULTS_FAST[$test]="$FAST_OUT"

    sleep 1

    # Wing
    echo -e "${CYAN}  [Wing] Running ${DURATION}s, ${CONNS} connections, pipeline ${PIPELINE}...${NC}"
    WING_OUT=$(wrk -t "$THREADS" -c "$CONNS" -d "${DURATION}s" \
        -s "$WRK_SCRIPT" \
        "http://127.0.0.1:$WING_PORT$URL_PATH" \
        -- "$METHOD" "$URL_PATH" "$PIPELINE" 2>&1) || true
    echo "$WING_OUT"
    RESULTS_WING[$test]="$WING_OUT"

    # Extract RPS for comparison
    FAST_RPS=$(echo "$FAST_OUT" | grep "Requests/sec" | awk '{print $2}' || echo "0")
    WING_RPS=$(echo "$WING_OUT" | grep "Requests/sec" | awk '{print $2}' || echo "0")

    if [[ "$FAST_RPS" != "0" && "$WING_RPS" != "0" ]]; then
        RATIO=$(echo "scale=2; $WING_RPS / $FAST_RPS" | bc 2>/dev/null || echo "N/A")
        PCTDIFF=$(echo "scale=1; ($WING_RPS / $FAST_RPS - 1) * 100" | bc 2>/dev/null || echo "N/A")
        echo
        if (( $(echo "$WING_RPS > $FAST_RPS" | bc -l 2>/dev/null || echo 0) )); then
            echo -e "  ${GREEN}🔺 Wing: ${RATIO}x fasthttp (+${PCTDIFF}%)${NC}"
        else
            echo -e "  ${RED}🔻 Wing: ${RATIO}x fasthttp (${PCTDIFF}%)${NC}"
        fi
    fi
done

# ── Summary ───────────────────────────────────────────────────────────────────
echo
echo -e "${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BOLD}║  Summary                                                    ║${NC}"
echo -e "${BOLD}╠══════════════════════════════════════════════════════════════╣${NC}"
echo -e "  CPU cores: $(nproc)"
echo -e "  Go: $(go version | awk '{print $3}')"
echo -e "  Kernel: $(uname -r)"
echo -e "  Connections: $CONNS | Threads: $THREADS | Pipeline: $PIPELINE"
echo -e "  Duration: ${DURATION}s per test | Warmup: ${WARMUP}s"
echo -e "${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"

echo
echo -e "${GREEN}Done!${NC}"
