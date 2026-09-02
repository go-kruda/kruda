#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
# v1.7.2 is the first core release containing the transport and toolchain
# hardening that every newly tagged contrib module must retain under Go MVS.
MINIMUM_CORE_VERSION=v1.7.2

version_at_least() {

  local actual=${1#v}
  local minimum=${2#v}
  local actual_base=${actual%%[-+]*}
  local minimum_base=${minimum%%[-+]*}
  local actual_major actual_minor actual_patch
  local minimum_major minimum_minor minimum_patch

  IFS=. read -r actual_major actual_minor actual_patch <<<"$actual_base"
  IFS=. read -r minimum_major minimum_minor minimum_patch <<<"$minimum_base"

  if [[ ! "$actual_major" =~ ^[0-9]+$ || ! "$actual_minor" =~ ^[0-9]+$ || ! "$actual_patch" =~ ^[0-9]+$ ]]; then
    return 1
  fi

  ((actual_major > minimum_major)) && return 0
  ((actual_major < minimum_major)) && return 1
  ((actual_minor > minimum_minor)) && return 0
  ((actual_minor < minimum_minor)) && return 1
  ((actual_patch > minimum_patch)) && return 0
  ((actual_patch < minimum_patch)) && return 1
  [[ "$actual" != *-* ]]
}

for module_file in "$ROOT"/contrib/*/go.mod; do
  module_dir=${module_file%/go.mod}
  module_path=$(sed -n 's/^module[[:space:]][[:space:]]*//p' "$module_file")
  consumer_dir=$(mktemp -d)
  exit_code=0

  (
    cd "$consumer_dir"
    GOWORK=off go mod init mvs-check >/dev/null 2>&1
    GOWORK=off go mod edit -require="${module_path}@v0.0.0"
    GOWORK=off go mod edit -replace="${module_path}=${module_dir}"
    GOWORK=off go mod edit -replace="github.com/go-kruda/kruda=${ROOT}"

    selected=$(GOWORK=off go list -m -f '{{.Version}}' github.com/go-kruda/kruda)
    if ! version_at_least "$selected" "$MINIMUM_CORE_VERSION"; then
      echo "$module_path selects Kruda core $selected; want $MINIMUM_CORE_VERSION or newer" >&2
      exit 1
    fi

    echo "==> $module_path selects Kruda core $selected"
  ) || exit_code=$?

  rm -rf "$consumer_dir"
  if ((exit_code != 0)); then
    exit "$exit_code"
  fi

done
