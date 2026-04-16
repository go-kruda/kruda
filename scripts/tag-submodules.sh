#!/usr/bin/env bash
# Tag Sub-Modules — Kruda Framework
# Updates sub-module go.mod deps, syncs go.sum, and creates version tags.
#
# Usage:
#   ./scripts/tag-submodules.sh v1.1.0
#
# Flow:
#   1. Update all sub-module go.mod to require kruda $VERSION
#   2. Update core go.mod to require transport/wing $VERSION
#   3. Commit go.mod changes
#   4. Tag sub-modules + push tags (so proxy can resolve them)
#   5. Run go mod tidy to update go.sum with new hashes
#   6. Commit go.sum changes
#   7. Tag core $VERSION
#   8. Push everything

set -euo pipefail

VERSION="${1:-}"
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Usage: $0 v1.0.3"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log() { echo -e "${CYAN}[tag-sub]${NC} $*"; }
ok() { echo -e "${GREEN}[✓]${NC} $*"; }
fail() { echo -e "${RED}[✗]${NC} $*"; exit 1; }

# Collect sub-modules
SUBMODS=(contrib/cache contrib/compress contrib/etag contrib/jwt contrib/otel contrib/prometheus contrib/ratelimit contrib/session contrib/swagger contrib/ws transport/wing)

# === Step 1: Update go.mod files ===
log "Updating sub-module go.mod to require kruda $VERSION..."
CHANGED=false
for mod in "${SUBMODS[@]}"; do
    if [ -f "$mod/go.mod" ]; then
        if grep -q "github.com/go-kruda/kruda v" "$mod/go.mod"; then
            sed -i '' "s|github.com/go-kruda/kruda v[0-9]*\.[0-9]*\.[0-9]*|github.com/go-kruda/kruda $VERSION|g" "$mod/go.mod"
            CHANGED=true
        fi
    fi
done

log "Updating core go.mod to require transport/wing $VERSION..."
if grep -q "github.com/go-kruda/kruda/transport/wing v" go.mod; then
    sed -i '' "s|github.com/go-kruda/kruda/transport/wing v[0-9]*\.[0-9]*\.[0-9]*|github.com/go-kruda/kruda/transport/wing $VERSION|g" go.mod
    CHANGED=true
fi

if [ "$CHANGED" = true ] && [ -n "$(git status --porcelain)" ]; then
    git add go.mod contrib/*/go.mod transport/wing/go.mod
    git commit -m "chore: sync sub-module deps to $VERSION"
    ok "Committed go.mod updates"
else
    ok "All go.mod files already up to date"
fi

# === Step 2: Tag sub-modules ===
log "Creating sub-module tags..."
TAGS=()
for mod in "${SUBMODS[@]}"; do
    TAG="$mod/$VERSION"
    if git rev-parse "$TAG" >/dev/null 2>&1; then
        log "  $TAG already exists, skipping"
    else
        git tag -a "$TAG" -m "$mod $VERSION"
        TAGS+=("$TAG")
        ok "  Tagged $TAG"
    fi
done

# === Step 3: Push sub-module tags so proxy can resolve them ===
if [ ${#TAGS[@]} -gt 0 ]; then
    log "Pushing sub-module tags..."
    git push origin main "${TAGS[@]}"
    ok "Sub-module tags pushed"

    # Wait for proxy to index
    log "Waiting for Go module proxy to index tags..."
    sleep 5
    GONOSUMCHECK=* GONOSUMDB=* go list -m "github.com/go-kruda/kruda/transport/wing@$VERSION" > /dev/null 2>&1 || true
fi

# === Step 4: Update go.sum with new version hashes ===
log "Running go mod tidy to update go.sum..."
GONOSUMCHECK=* GONOSUMDB=* GOWORK=off go mod tidy 2>&1 || fail "go mod tidy failed for core"
(cd transport/wing && GONOSUMCHECK=* GONOSUMDB=* GOWORK=off go mod tidy 2>&1) || fail "go mod tidy failed for transport/wing"

if [ -n "$(git status --porcelain -- go.sum transport/wing/go.sum)" ]; then
    git add go.sum transport/wing/go.sum
    git commit -m "chore: update go.sum for $VERSION"
    ok "Committed go.sum updates"
else
    ok "go.sum already up to date"
fi

# === Step 5: Tag core ===
CORE_TAG="$VERSION"
if git rev-parse "$CORE_TAG" >/dev/null 2>&1; then
    log "Core tag $CORE_TAG already exists, skipping"
else
    git tag -a "$CORE_TAG" -m "$CORE_TAG"
    ok "Tagged $CORE_TAG"
fi

# === Step 6: Push everything ===
echo ""
echo -e "${BOLD}Ready to push:${NC}"
echo "  - main branch (go.sum commit)"
echo "  - $CORE_TAG"
echo ""
read -p "Push now? (y/N): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    git push origin main "$CORE_TAG"
    ok "All pushed — $VERSION released!"
    echo ""
    echo -e "${BOLD}Verify:${NC}"
    echo "  curl -s https://proxy.golang.org/github.com/go-kruda/kruda/@v/$VERSION.info"
fi
