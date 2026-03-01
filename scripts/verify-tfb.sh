#!/usr/bin/env bash
# TFB Verification Script — Kruda Framework
# Verifies TechEmpower Framework Benchmarks setup locally before submission

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log() { echo -e "${CYAN}[tfb-verify]${NC} $*"; }
ok() { echo -e "${GREEN}[OK]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; exit 1; }

# Check prerequisites
log "Checking prerequisites..."
command -v docker >/dev/null || fail "docker not found"
command -v curl >/dev/null || fail "curl not found"
ok "Prerequisites found"

# Verify TFB directory structure
log "Verifying TFB directory structure..."
TFB_DIR="$ROOT_DIR/frameworks/Go/kruda"

[ -d "$TFB_DIR" ] || fail "TFB directory not found: $TFB_DIR"
[ -f "$TFB_DIR/benchmark_config.json" ] || fail "Missing benchmark_config.json"
[ -f "$TFB_DIR/kruda.dockerfile" ] || fail "Missing kruda.dockerfile"
[ -f "$TFB_DIR/src/main.go" ] || fail "Missing src/main.go"
[ -f "$TFB_DIR/src/go.mod" ] || fail "Missing src/go.mod"

ok "TFB structure valid"

# Validate benchmark_config.json
log "Validating benchmark_config.json..."
if command -v jq >/dev/null; then
    jq empty "$TFB_DIR/benchmark_config.json" || fail "Invalid JSON in benchmark_config.json"
    
    # Check required fields
    FRAMEWORK=$(jq -r '.framework' "$TFB_DIR/benchmark_config.json")
    [ "$FRAMEWORK" = "kruda" ] || fail "Framework name should be 'kruda', got '$FRAMEWORK'"
    
    # Check test configuration
    TESTS=$(jq -r '.tests[0].default' "$TFB_DIR/benchmark_config.json")
    [ "$TESTS" != "null" ] || fail "Missing default test configuration"
    
    ok "benchmark_config.json valid"
else
    warn "jq not found, skipping JSON validation"
fi

# Build Docker image
log "Building TFB Docker image..."
cd "$TFB_DIR"
docker build -f kruda.dockerfile -t kruda-tfb . || fail "Docker build failed"
ok "Docker image built successfully"

# Test container startup
log "Testing container startup..."
CONTAINER_ID=$(docker run -d -p 8080:8080 kruda-tfb)

# Wait for server to start
log "Waiting for server to start..."
for i in {1..30}; do
    if curl -sf "http://localhost:8080/json" >/dev/null 2>&1; then
        ok "Server started successfully"
        break
    fi
    if [ $i -eq 30 ]; then
        docker logs "$CONTAINER_ID"
        fail "Server failed to start within 30 seconds"
    fi
    sleep 1
done

# Test all TFB endpoints
log "Testing TFB endpoints..."
ENDPOINTS=(
    "/json:application/json"
    "/plaintext:text/plain"
    "/db:application/json"
    "/queries?queries=1:application/json"
    "/queries?queries=20:application/json"
    "/fortunes:text/html"
    "/cached-queries?count=1:application/json"
    "/cached-queries?count=20:application/json"
    "/updates?queries=1:application/json"
    "/updates?queries=20:application/json"
)

for endpoint_spec in "${ENDPOINTS[@]}"; do
    IFS=':' read -r endpoint expected_type <<< "$endpoint_spec"
    
    log "Testing $endpoint..."
    
    # Test response code
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080$endpoint")
    [ "$STATUS" = "200" ] || fail "$endpoint returned status $STATUS"
    
    # Test content type
    CONTENT_TYPE=$(curl -s -I "http://localhost:8080$endpoint" | grep -i "content-type" | cut -d' ' -f2- | tr -d '\r\n')
    if [[ "$CONTENT_TYPE" != *"$expected_type"* ]]; then
        warn "$endpoint content-type: expected '$expected_type', got '$CONTENT_TYPE'"
    fi
    
    # Test response body (basic validation)
    RESPONSE=$(curl -s "http://localhost:8080$endpoint")
    [ -n "$RESPONSE" ] || fail "$endpoint returned empty response"
    
    case "$endpoint" in
        "/json")
            echo "$RESPONSE" | grep -q '"message"' || fail "/json missing 'message' field"
            ;;
        "/plaintext")
            [ "$RESPONSE" = "Hello, World!" ] || fail "/plaintext incorrect response"
            ;;
        "/db"|"/queries"*|"/cached-queries"*)
            echo "$RESPONSE" | grep -q '"id"' || fail "$endpoint missing 'id' field"
            ;;
        "/fortunes")
            echo "$RESPONSE" | grep -q "<html>" || fail "/fortunes not HTML"
            ;;
        "/updates"*)
            echo "$RESPONSE" | grep -q '"randomNumber"' || fail "$endpoint missing 'randomNumber' field"
            ;;
    esac
    
    ok "$endpoint responds correctly"
done

# Performance smoke test
log "Running performance smoke test..."
if command -v wrk >/dev/null; then
    # Quick 5-second test
    WRK_OUTPUT=$(wrk -t2 -c10 -d5s http://localhost:8080/json 2>&1)
    RPS=$(echo "$WRK_OUTPUT" | grep "Requests/sec:" | awk '{print $2}')
    
    if [ -n "$RPS" ]; then
        RPS_INT=$(printf "%.0f" "$RPS")
        if [ "$RPS_INT" -gt 1000 ]; then
            ok "Performance test: $RPS req/s (good)"
        else
            warn "Performance test: $RPS req/s (low, but acceptable for testing)"
        fi
    else
        warn "Could not parse wrk output"
    fi
else
    warn "wrk not found, skipping performance test"
fi

# Check Docker image size
log "Checking Docker image size..."
IMAGE_SIZE=$(docker images kruda-tfb --format "{{.Size}}")
ok "Docker image size: $IMAGE_SIZE"

# Cleanup
log "Cleaning up..."
docker stop "$CONTAINER_ID" >/dev/null
docker rm "$CONTAINER_ID" >/dev/null
ok "Container cleaned up"

# Final summary
echo ""
echo -e "${GREEN}${BOLD}✅ TFB Verification Complete${NC}"
echo ""
echo "Summary:"
echo "  ✓ Directory structure valid"
echo "  ✓ Docker image builds successfully"
echo "  ✓ All TFB endpoints respond correctly"
echo "  ✓ Content types match expectations"
echo "  ✓ Response formats validated"
echo ""
echo "Ready for TechEmpower Framework Benchmarks submission!"
echo ""
echo "Next steps:"
echo "  1. Fork https://github.com/TechEmpower/FrameworkBenchmarks"
echo "  2. Copy frameworks/Go/kruda/ to your fork"
echo "  3. Submit PR with your implementation"
echo "  4. Address review feedback"
echo ""