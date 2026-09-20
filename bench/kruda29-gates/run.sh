#!/usr/bin/env bash
# KRUDA-29 Linux gates: native-Linux correctness verification.
# See README.md. Any failure aborts the run with a non-zero exit.
set -euo pipefail

cd "$(dirname "$0")/../.."

RESULTS_DIR="${RESULTS_DIR:-}"
FUZZTIME="${FUZZTIME:-30s}"
SUMMARY=""

record() {
	echo "==> $*"
	SUMMARY+="$*
"
}

record_env() {
	record "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	record "go: $(go version)"
	record "kernel: $(uname -a)"
	record "cpu: $(nproc) online"
	record "memory: $(free -m | awk '/^Mem:/ {print $2}') MiB total"
	if [ -d .git ]; then
		record "git sha: $(git rev-parse HEAD)"
		record "git clean: $([ -z "$(git status --short)" ] && echo yes || echo NO)"
	else
		record "git sha: unavailable (no .git in build context)"
	fi
	record "go.mod hash: $(sha256sum go.mod | cut -d' ' -f1)"
}

record_env

record "gate 1/6: go build ./..."
go build ./...

record "gate 2/6: go vet (default + kruda_stdjson)"
go vet ./...
go vet -tags kruda_stdjson ./...

record "gate 3/6: full race suite, default engine"
CI=1 go test -race -count=1 ./...

record "gate 4/6: full race suite, kruda_stdjson engine"
CI=1 go test -race -count=1 -tags kruda_stdjson ./...

record "gate 5/6: Linux-only Wing short-read tests (explicit)"
go test -race -count=1 -v -run 'TestWingShortRead' .

if [ "$FUZZTIME" != "0" ]; then
	record "gate 6/6: fuzz smokes (${FUZZTIME} each)"
	for target in FuzzValidateString FuzzParserDifferential FuzzBindJSON \
		FuzzParseHTTPRequest FuzzRouterPattern FuzzRouterMatch; do
		record "-- $target"
		go test -run=NONE -fuzz="$target" -fuzztime="$FUZZTIME" .
	done
else
	record "gate 6/6: fuzz smokes SKIPPED (FUZZTIME=0)"
fi

record "ALL LINUX GATES PASSED"

if [ -n "$RESULTS_DIR" ]; then
	mkdir -p "$RESULTS_DIR"
	printf '%s' "$SUMMARY" >"$RESULTS_DIR/summary.txt"
	echo "summary written to $RESULTS_DIR/summary.txt"
fi
