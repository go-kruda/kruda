#!/usr/bin/env bash
# Run govulncheck against every released module: root, cmd/kruda, and contrib.
# The root `govulncheck ./...` does not descend into nested modules, so each
# standalone module must be scanned in its own module context to catch a
# regression to a vulnerable dependency.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

local_core=false
if [[ "${1:-}" == "--local-core" ]]; then
  local_core=true
  shift
fi
if [[ $# -ne 0 ]]; then
  echo "usage: $0 [--local-core]" >&2
  exit 2
fi

go install golang.org/x/vuln/cmd/govulncheck@latest
GOVULN=$(go env GOPATH)/bin/govulncheck
export GOWORK=off

echo "== govulncheck (root) =="
"$GOVULN" ./...

# Every released module resolves against its own go.mod rather than a local
# go.work, including the root scan above.
while IFS= read -r modfile; do
  dir=$(dirname "$modfile")
  echo "== govulncheck ($dir) =="
  if [[ "$local_core" == true && "$dir" == contrib/* ]]; then
    scan_dir=$(mktemp -d)
    cp -R "$dir/." "$scan_dir"
    ( cd "$scan_dir" && go mod edit -replace github.com/go-kruda/kruda="$ROOT" )
    exit_code=0
    ( cd "$scan_dir" && "$GOVULN" ./... ) || exit_code=$?
    rm -rf "$scan_dir"
    if ((exit_code != 0)); then
      exit "$exit_code"
    fi
  else
    ( cd "$dir" && "$GOVULN" ./... )
  fi
done < <(git ls-files 'cmd/kruda/go.mod' 'contrib/*/go.mod')
