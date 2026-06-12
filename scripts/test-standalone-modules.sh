#!/usr/bin/env bash
# Test Kruda standalone modules with an ephemeral local replace.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

race=false
verbose=false
modules=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --race)
      race=true
      shift
      ;;
    --verbose)
      verbose=true
      shift
      ;;
    contrib)
      while IFS= read -r dir; do
        modules+=("$dir")
      done < <(find contrib -mindepth 2 -maxdepth 2 -name go.mod -print | sed 's#/go.mod$##' | sort)
      shift
      ;;
    *)
      modules+=("${1%/}")
      shift
      ;;
  esac
done

if [[ ${#modules[@]} -eq 0 ]]; then
  while IFS= read -r dir; do
    modules+=("$dir")
  done < <(find contrib -mindepth 2 -maxdepth 2 -name go.mod -print | sed 's#/go.mod$##' | sort)
  modules+=("cmd/kruda")
fi

test_flags=(-tags kruda_stdjson)
if [[ "$race" == true ]]; then
  test_flags=(-race "${test_flags[@]}")
fi
if [[ "$verbose" == true ]]; then
  test_flags=(-v "${test_flags[@]}")
fi

restore_module() {
  local module_dir=$1
  local gomod_backup=$2
  local gosum_backup=$3

  cp "$gomod_backup" "$module_dir/go.mod"
  if [[ -n "$gosum_backup" ]]; then
    cp "$gosum_backup" "$module_dir/go.sum"
  else
    rm -f "$module_dir/go.sum"
  fi
}

test_replaced_module() {
  local module_dir=$1
  local gomod_backup gosum_backup status

  gomod_backup=$(mktemp)
  cp "$module_dir/go.mod" "$gomod_backup"
  if [[ -f "$module_dir/go.sum" ]]; then
    gosum_backup=$(mktemp)
    cp "$module_dir/go.sum" "$gosum_backup"
  else
    gosum_backup=""
  fi

  status=0
  (
    cd "$module_dir"
    GOWORK=off go mod edit -replace github.com/go-kruda/kruda="$ROOT"
    GOWORK=off go mod tidy
    GOWORK=off go test "${test_flags[@]}" ./...
  ) || status=$?
  restore_module "$module_dir" "$gomod_backup" "$gosum_backup"
  rm -f "$gomod_backup" "$gosum_backup"
  return "$status"
}

test_plain_module() {
  local module_dir=$1
  (cd "$module_dir" && GOWORK=off go test "${test_flags[@]}" ./...)
}

for module_dir in "${modules[@]}"; do
  if [[ ! -f "$module_dir/go.mod" ]]; then
    echo "missing go.mod: $module_dir" >&2
    exit 1
  fi

  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::group::Testing $module_dir"
  else
    echo "==> Testing $module_dir"
  fi

  if [[ "$module_dir" == "cmd/kruda" ]]; then
    test_plain_module "$module_dir"
  else
    test_replaced_module "$module_dir"
  fi

  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::endgroup::"
  fi
done
