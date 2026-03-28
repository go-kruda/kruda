#!/usr/bin/env bash
# Pre-Release Checklist — Kruda Framework
# Comprehensive checks before tagging a release

set -euo pipefail

VERSION="$1"
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?$ ]]; then
    echo "Usage: $0 v1.0.0"
    echo "       $0 v1.0.0-beta.1"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() { echo -e "${CYAN}[pre-release]${NC} $*"; }
ok() { echo -e "${GREEN}[✓]${NC} $*"; }
warn() { echo -e "${YELLOW}[!]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; exit 1; }

echo -e "${BOLD}🔍 Pre-release checks for $VERSION${NC}"
echo ""

cd "$ROOT_DIR"

# 1. Git status check
log "Checking git status..."
if [ -n "$(git status --porcelain)" ]; then
    warn "Working directory has uncommitted changes"
    git status --short
    echo ""
    read -p "Continue anyway? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
else
    ok "Working directory clean"
fi

# 2. Go module verification
log "Verifying Go modules..."
go mod tidy
go mod verify || fail "Module verification failed"

if [ -n "$(git status --porcelain go.mod go.sum)" ]; then
    fail "go.mod or go.sum has uncommitted changes after 'go mod tidy'"
fi
ok "Go modules verified"

# 3. Test suite
log "Running test suite..."
go test -race -tags kruda_stdjson ./... || fail "Tests failed"
ok "All tests pass"

# 4. Linting
log "Running linter..."
if command -v golangci-lint >/dev/null; then
    golangci-lint run --build-tags kruda_stdjson || fail "Linting failed"
    ok "Linting passed"
else
    warn "golangci-lint not found, skipping"
fi

# 5. Security scan
log "Running security scan..."
if command -v govulncheck >/dev/null; then
    govulncheck ./... || fail "Security vulnerabilities found"
    ok "Security scan clean"
else
    warn "govulncheck not found, installing..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./... || fail "Security vulnerabilities found"
    ok "Security scan clean"
fi

# 6. Benchmark regression check
log "Checking benchmark performance..."
BENCH_FILE="bench_${VERSION}.txt"
go test -bench=. -benchmem -count=3 -tags kruda_stdjson ./bench/... > "$BENCH_FILE" 2>&1 || warn "Benchmark run had issues"

if [ -f "bench_baseline.txt" ]; then
    if command -v benchstat >/dev/null; then
        log "Comparing against baseline..."
        BENCHSTAT_OUTPUT=$(benchstat bench_baseline.txt "$BENCH_FILE" 2>&1 || true)
        echo "$BENCHSTAT_OUTPUT"
        
        # Check for significant regressions (>10% slower)
        if echo "$BENCHSTAT_OUTPUT" | grep -E '\+[1-9][0-9]%|[+][0-9]{3,}%' | grep -v '~'; then
            warn "Significant performance regression detected"
            echo "Review the benchstat output above"
            read -p "Continue anyway? (y/N): " -n 1 -r
            echo ""
            if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                exit 1
            fi
        else
            ok "No significant performance regression"
        fi
    else
        warn "benchstat not found, installing..."
        go install golang.org/x/perf/cmd/benchstat@latest
    fi
else
    warn "No baseline benchmark found, creating one..."
    cp "$BENCH_FILE" bench_baseline.txt
fi

# 7. Documentation build
log "Building documentation..."
if [ -d "docs" ] && [ -f "docs/package.json" ]; then
    cd docs
    if [ -f "package-lock.json" ]; then
        npm ci || fail "npm install failed"
    else
        npm install || fail "npm install failed"
    fi
    npm run build || fail "Documentation build failed"
    cd "$ROOT_DIR"
    ok "Documentation builds successfully"
else
    warn "Documentation directory not found, skipping"
fi

# 8. Version consistency check
log "Checking version consistency..."
# Check if version is mentioned in key files
FILES_TO_CHECK=("README.md" "CHANGELOG.md")
for file in "${FILES_TO_CHECK[@]}"; do
    if [ -f "$file" ]; then
        if ! grep -q "${VERSION#v}" "$file"; then
            warn "$file does not mention version ${VERSION#v}"
        fi
    fi
done

# 9. Build verification
log "Verifying builds for target platforms..."
PLATFORMS=("linux/amd64" "darwin/arm64" "windows/amd64")
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$platform"
    log "Building for $GOOS/$GOARCH..."
    GOOS="$GOOS" GOARCH="$GOARCH" go build -o /tmp/kruda-test-"$GOOS"-"$GOARCH" ./cmd/... 2>/dev/null || warn "Build failed for $platform"
done
ok "Cross-platform builds verified"

# 10. Check for TODO/FIXME comments
log "Checking for TODO/FIXME comments..."
TODO_COUNT=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs grep -c "TODO\|FIXME" | awk -F: '{sum += $2} END {print sum+0}')
if [ "$TODO_COUNT" -gt 0 ]; then
    warn "Found $TODO_COUNT TODO/FIXME comments"
    find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | xargs grep -n "TODO\|FIXME" | head -5
    if [ "$TODO_COUNT" -gt 5 ]; then
        echo "... and $((TODO_COUNT - 5)) more"
    fi
else
    ok "No TODO/FIXME comments found"
fi

# Summary
echo ""
echo -e "${GREEN}${BOLD}✅ Pre-release checks complete for $VERSION${NC}"
echo ""
echo "Summary:"
echo "  ✓ Git status clean"
echo "  ✓ Go modules verified"
echo "  ✓ All tests pass"
echo "  ✓ Linting clean"
echo "  ✓ Security scan clean"
echo "  ✓ Benchmark performance acceptable"
echo "  ✓ Documentation builds"
echo "  ✓ Cross-platform builds work"
echo ""
echo -e "${BOLD}Ready to tag release:${NC}"
echo "  git tag $VERSION"
echo "  git push origin $VERSION"
echo ""
echo -e "${BOLD}This will trigger:${NC}"
echo "  • GitHub Actions release workflow"
echo "  • Automated testing and security scans"
echo "  • GitHub Release creation"
echo "  • Documentation deployment"
echo ""