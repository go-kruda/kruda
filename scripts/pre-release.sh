#!/usr/bin/env bash
# Pre-release validation for Kruda core and independently versioned modules.
# Run this from the repo root before tagging. Exits non-zero on any failure.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

ok() { printf "\033[32m ✓\033[0m %s\n" "$*"; }
fail() { printf "\033[31m ✗\033[0m %s\n" "$*" >&2; exit 1; }
section() { printf "\n\033[1m== %s ==\033[0m\n" "$*"; }

# Homebrew Go can report GOROOT as the formula root when invoked from some
# non-interactive shells. The actual toolchain is under libexec.
if ! go tool vet -h >/dev/null 2>&1; then
  goroot=$(go env GOROOT)
  if [[ -d "$goroot/libexec/pkg/tool" ]]; then
    export GOROOT="$goroot/libexec"
  fi
fi

section "tracked working tree clean"
if ! git diff --quiet || ! git diff --cached --quiet; then
  git status --short --untracked-files=no
  fail "tracked working tree has uncommitted changes"
fi
if [[ -n "$(git status --porcelain --untracked-files=normal | grep '^??' || true)" ]]; then
  echo "  warning: untracked local files are present but ignored by release checks"
fi
ok "no tracked uncommitted changes"

section "no replace directives in committed go.mod files (released modules only)"
# examples/ and bench/ are internal-use modules; their go.mod can keep replace
# directives that point at the local repo. Released modules (root, cmd/kruda,
# contrib/*) must NOT have replace directives.
violations=$(git ls-files '*go.mod' | grep -vE '^(examples|bench)/' | xargs grep -l '^replace ' 2>/dev/null || true)
if [[ -n "$violations" ]]; then
  echo "$violations"
  fail "replace directive found in a released module's go.mod"
fi
ok "no replace directives in released go.mod"

section "tests (host platform)"
go test -race -tags kruda_stdjson ./...
ok "stdlib JSON tests passed"
go test -race ./...
ok "default JSON engine tests passed"

section "standalone module tests"
scripts/test-standalone-modules.sh
ok "standalone modules"

section "cross-platform builds"
GOOS=linux go build ./...
ok "linux build"
GOOS=darwin go build ./...
ok "darwin build"
GOOS=windows go build ./...
ok "windows build"

section "fuzz suites (30s each)"
for fuzz in FuzzRouterPattern FuzzRouterMatch FuzzBindJSON FuzzValidateString FuzzParseHTTPRequest FuzzParserDifferential; do
  if go test -tags kruda_stdjson -fuzz="$fuzz" -fuzztime=30s -run=^$ . >/tmp/fuzz-"$fuzz".log 2>&1; then
    ok "$fuzz no crashes"
  else
    cat /tmp/fuzz-"$fuzz".log
    fail "$fuzz reported a crash or failure"
  fi
done

section "current documentation correctness"
scripts/check-docs.sh
ok "current docs checked"

section "godoc completeness on exported symbols (released code only)"
scripts/check-godoc.sh
ok "godoc complete on released code"

section "examples build"
for dir in examples/*/; do
  if [[ -f "$dir/main.go" ]]; then
    if ! (cd "$dir" && GOWORK=off go build ./... >/dev/null 2>&1); then
      fail "example failed to build: $dir"
    fi
  fi
done
ok "all examples build"

section "CHANGELOG has an entry for the next version"
# This check is informational — it doesn't know what version is being tagged,
# so it just verifies CHANGELOG.md was updated recently.
if [[ $(git log -1 --format=%ct CHANGELOG.md) -lt $(date -d '30 days ago' +%s 2>/dev/null || date -v-30d +%s 2>/dev/null || echo 0) ]]; then
  echo "  warning: CHANGELOG.md hasn't been updated in 30+ days"
fi
ok "CHANGELOG checked"

printf "\n\033[1;32m== ALL PRE-RELEASE CHECKS PASSED ==\033[0m\n"
