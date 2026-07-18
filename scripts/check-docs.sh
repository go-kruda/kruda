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
done < <(git ls-files 'docs/guide/*.md' 'docs/api/*.md' 'docs/faq.md' 'docs/troubleshooting.md' \
  'docs/release-process.md' 'contrib/*/README.md' 'README.md' 'CONTRIBUTING.md')

module_files=()
while IFS= read -r file; do
  module_files+=("$file")
done < <(git ls-files '*go.mod')

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

if hits=$(git grep -nF 'transport/wing' -- CONTRIBUTING.md || true); [[ -n "$hits" ]]; then
  echo "$hits"
  fail "contributor setup references the removed transport/wing package"
fi

if hits=$(git grep -nF 'github.com/go-kruda/kruda/transport/wing' -- "${module_files[@]}" || true); [[ -n "$hits" ]]; then
  echo "$hits"
  fail "module configuration references the removed transport/wing module"
fi

ok "current docs and module configuration pass removed Wing checks"
