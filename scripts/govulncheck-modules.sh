#!/usr/bin/env bash
# Run govulncheck against the root module and every nested contrib go.mod.
# The root `govulncheck ./...` does not descend into nested modules, so each
# standalone module (e.g. contrib/observability, which pins the OTel SDK) must
# be scanned in its own module context to catch a regression to a vulnerable dep.
set -euo pipefail
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

go install golang.org/x/vuln/cmd/govulncheck@latest
GOVULN=$(go env GOPATH)/bin/govulncheck

echo "== govulncheck (root) =="
"$GOVULN" ./...

# Nested contrib modules are excluded from the root go.work, so each scan runs
# with GOWORK=off to resolve against the module's own go.mod (and its
# `replace github.com/go-kruda/kruda => ../..`).
while IFS= read -r modfile; do
  dir=$(dirname "$modfile")
  echo "== govulncheck ($dir) =="
  ( cd "$dir" && GOWORK=off "$GOVULN" ./... )
done < <(find contrib -mindepth 2 -maxdepth 2 -name go.mod -print | sort)
