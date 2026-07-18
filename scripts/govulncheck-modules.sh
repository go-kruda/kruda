#!/usr/bin/env bash
# Run govulncheck against every released module: root, cmd/kruda, and contrib.
# The root `govulncheck ./...` does not descend into nested modules, so each
# standalone module must be scanned in its own module context to catch a
# regression to a vulnerable dependency.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

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
  ( cd "$dir" && "$GOVULN" ./... )
done < <(git ls-files 'cmd/kruda/go.mod' 'contrib/*/go.mod')
