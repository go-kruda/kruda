#!/usr/bin/env bash
# Checks current user-facing docs for references to removed APIs.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

fail() {
  printf "\033[31m ✗\033[0m %s\n" "$*" >&2
  exit 1
}

ok() { printf "\033[32m ✓\033[0m %s\n" "$*"; }

current_docs=()
while IFS= read -r file; do
  current_docs+=("$file")
done < <(git ls-files 'docs/guide/*.md' 'docs/api/*.md' 'docs/release-process.md' 'README.md')

if [[ ${#current_docs[@]} -eq 0 ]]; then
  fail "no current docs found to scan"
fi

removed_api='\<Wing(Plaintext|JSON|Query|Render|StaticText|StaticJSON)\>'
if hits=$(git grep -nE "$removed_api" -- "${current_docs[@]}" || true); [[ -n "$hits" ]]; then
  echo "$hits"
  fail "current docs reference removed Wing helper APIs"
fi

stale_wing_shim='transport/wing.*(still exists|continues to work|compatibility shim|deprecation alias|re-exports|slated for removal|v2\.0\.0)'
if hits=$(git grep -nE "$stale_wing_shim" -- "${current_docs[@]}" || true); [[ -n "$hits" ]]; then
  echo "$hits"
  fail "current docs describe transport/wing as a live compatibility shim"
fi

ok "current docs do not reference removed Wing APIs"
