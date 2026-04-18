#!/usr/bin/env bash
# Pre-release validation for Kruda v1.2.0+ single-tag flow.
# Run this from the repo root before tagging. Exits non-zero on any failure.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

ok() { printf "\033[32m ✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m ✗\033[0m %s\n" "$*" >&2; exit 1; }
section() { printf "\n\033[1m== %s ==\033[0m\n" "$*"; }

section "working tree clean"
if [[ -n "$(git status --porcelain)" ]]; then
  git status --porcelain
  fail "working tree has uncommitted changes"
fi
ok "no uncommitted changes"

section "no replace directives in committed go.mod files"
violations=$(git ls-files '*go.mod' | xargs grep -l '^replace ' 2>/dev/null || true)
if [[ -n "$violations" ]]; then
  echo "$violations"
  fail "replace directive found in committed go.mod files"
fi
ok "no replace directives in committed go.mod"

section "tests (host platform)"
go test -race ./...
ok "tests passed"

section "cross-platform builds"
GOOS=linux go build ./... && ok "linux build"
GOOS=darwin go build ./... && ok "darwin build"
GOOS=windows go build ./... && ok "windows build"

section "fuzz suites (30s each)"
for fuzz in FuzzRouterPattern FuzzRouterMatch FuzzBindJSON FuzzValidateString; do
  if go test -tags kruda_stdjson -fuzz="$fuzz" -fuzztime=30s -run=^$ . >/tmp/fuzz-"$fuzz".log 2>&1; then
    ok "$fuzz no crashes"
  else
    cat /tmp/fuzz-"$fuzz".log
    fail "$fuzz reported a crash or failure"
  fi
done

section "godoc completeness on exported symbols"
if command -v revive >/dev/null; then
  if revive -config <(echo '[rule.exported]') ./... 2>&1 | grep -q "should have comment"; then
    revive -config <(echo '[rule.exported]') ./... 2>&1 | grep "should have comment" | head -10
    fail "missing godoc on exported symbols"
  fi
  ok "godoc complete"
else
  echo "  (revive not installed, skipping godoc check)"
fi

section "examples build"
for dir in examples/*/; do
  if [[ -f "$dir/main.go" ]]; then
    if ! (cd "$dir" && go build ./... >/dev/null 2>&1); then
      fail "example failed to build: $dir"
    fi
  fi
done
ok "all examples build"

section "CHANGELOG has an entry for the next version"
# This check is informational — it doesn't know what version is being tagged,
# so it just verifies CHANGELOG.md was updated recently.
if [[ $(git log -1 --format=%ct CHANGELOG.md) -lt $(date -d '30 days ago' +%s 2>/dev/null || date -v-30d +%s) ]]; then
  echo "  warning: CHANGELOG.md hasn't been updated in 30+ days"
fi
ok "CHANGELOG checked"

printf "\n\033[1;32m== ALL PRE-RELEASE CHECKS PASSED ==\033[0m\n"
