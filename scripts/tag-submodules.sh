#!/usr/bin/env bash
# Tag Sub-Modules — Kruda Framework
# Updates sub-module go.mod deps and creates version tags for all sub-modules.
#
# Usage:
#   ./scripts/tag-submodules.sh v1.0.3
#
# This will:
#   1. Update all sub-module go.mod to require kruda $VERSION
#   2. Commit the changes
#   3. Tag each sub-module (contrib/*/vX.Y.Z, transport/wing/vX.Y.Z)
#   4. Push tags

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

# Collect sub-modules
SUBMODS=(contrib/cache contrib/compress contrib/etag contrib/jwt contrib/otel contrib/prometheus contrib/ratelimit contrib/session contrib/swagger contrib/ws transport/wing)

# 1. Update go.mod files
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

# Also update core go.mod to point to new transport/wing version
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

# 2. Tag sub-modules
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

# 3. Summary
echo ""
if [ ${#TAGS[@]} -eq 0 ]; then
    echo -e "${GREEN}${BOLD}No new tags to push.${NC}"
else
    echo -e "${BOLD}Tags created:${NC}"
    for tag in "${TAGS[@]}"; do
        echo "  $tag"
    done
    echo ""
    echo -e "${BOLD}Push all tags:${NC}"
    echo "  git push origin ${TAGS[*]}"
    echo ""
    read -p "Push now? (y/N): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git push origin "${TAGS[@]}"
        ok "All tags pushed"
    fi
fi
