#!/usr/bin/env bash
# Checks exported symbol comments in released modules.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

fail() {
  printf "\033[31m ✗\033[0m %s\n" "$*" >&2
  exit 1
}

ok() { printf "\033[32m ✓\033[0m %s\n" "$*"; }

if ! command -v revive >/dev/null; then
  fail "revive is required for godoc checks; install with: go install github.com/mgechev/revive@latest"
fi

config=$(mktemp)
trap 'rm -f "$config"' EXIT
printf '[rule.exported]\n' > "$config"

contrib_modules=()
while IFS= read -r module; do
  contrib_modules+=("$module")
done < <(find contrib -mindepth 2 -maxdepth 2 -name go.mod -print | sed 's#/go.mod$##' | sort)
modules=(".")
modules+=("${contrib_modules[@]}")
modules+=("cmd/kruda")

gooses=(linux darwin windows)
missing=""

for module in "${modules[@]}"; do
  for goos in "${gooses[@]}"; do
    output=$(cd "$module" && GOOS="$goos" revive -config "$config" ./... 2>&1 || true)
    output=$(printf "%s\n" "$output" | grep "should have comment" | grep -vE '^examples/' || true)
    if [[ -n "$output" ]]; then
      missing+=$'\n'"== $module ($goos) =="$'\n'"$output"
    fi
  done
done

if [[ -n "$missing" ]]; then
  printf "%s\n" "$missing" | head -40
  fail "missing godoc on exported symbols in released code"
fi

ok "godoc complete on released code"
