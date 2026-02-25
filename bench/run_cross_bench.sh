#!/bin/bash

set -e

# Check dependencies
if ! command -v bombardier &> /dev/null; then
    echo "Error: bombardier not found. Install with:"
    echo "  go install github.com/codesenberg/bombardier@latest"
    exit 1
fi

if ! command -v bun &> /dev/null; then
    echo "Error: bun not found. Install from https://bun.sh"
    exit 1
fi

# Cleanup function
cleanup() {
    echo "Stopping servers..."
    kill -9 $KRUDA_PID 2>/dev/null || true
    kill -9 $ELYSIA_PID 2>/dev/null || true
    # kill any leftover bun/kruda processes on our ports
    lsof -ti:3000,3001 | xargs kill -9 2>/dev/null || true
}
trap cleanup EXIT

# Start servers
echo "Starting Kruda server on port 3000 (fasthttp)..."
cd kruda_server
go build -o kruda_server_bin . 2>/dev/null
PORT=3000 TRANSPORT=fasthttp ./kruda_server_bin &
KRUDA_PID=$!
cd ..

echo "Installing Elysia dependencies..."
cd elysia
bun install --silent
echo "Starting Elysia server on port 3001..."
PORT=3001 bun start &
ELYSIA_PID=$!
cd ..

# Wait for servers to be ready
echo "Waiting for servers to start..."
for i in {1..20}; do
    if curl -s http://localhost:3000/ > /dev/null && curl -s http://localhost:3001/ > /dev/null; then
        echo "Both servers ready!"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "Timeout waiting for servers"
        exit 1
    fi
    sleep 0.5
done

# Benchmark parameters
CONNECTIONS=100
DURATION=5s

echo "Running benchmarks..."
echo "===================="

# GET / (plaintext)
echo "Benchmark: GET / (plaintext)"
echo "Kruda (Go):"
bombardier -c $CONNECTIONS -d $DURATION http://localhost:3000/ | grep "Reqs/sec"
echo "Elysia (Bun):"
bombardier -c $CONNECTIONS -d $DURATION http://localhost:3001/ | grep "Reqs/sec"
echo

# GET /users/:id (param)
echo "Benchmark: GET /users/:id (param)"
echo "Kruda (Go):"
bombardier -c $CONNECTIONS -d $DURATION http://localhost:3000/users/42 | grep "Reqs/sec"
echo "Elysia (Bun):"
bombardier -c $CONNECTIONS -d $DURATION http://localhost:3001/users/42 | grep "Reqs/sec"
echo

# POST /json (JSON)
echo "Benchmark: POST /json (JSON)"
echo "Kruda (Go):"
bombardier -c $CONNECTIONS -d $DURATION -m POST -H "Content-Type: application/json" -b '{"name":"John","email":"john@example.com"}' http://localhost:3000/json | grep "Reqs/sec"
echo "Elysia (Bun):"
bombardier -c $CONNECTIONS -d $DURATION -m POST -H "Content-Type: application/json" -b '{"name":"John","email":"john@example.com"}' http://localhost:3001/json | grep "Reqs/sec"

echo
echo "Benchmark complete!"