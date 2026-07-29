#!/usr/bin/env bash
# Reproduce the CHANGELOG's cold-start and startup-RSS figures for the two JSON
# engines, and test the premise they rest on.
#
# The claim is that CGO_ENABLED=0 builds, which used to get encoding/json and
# now get Sonic, pay for Sonic's JIT warm-up at process start — quoted as
# "25 typed POST routes: cold start 6.5 ms → 13.5 ms, startup RSS 12.1 MB →
# 17.6 MB". Route count is swept as well as fixed at 25, because if warm-up
# does not scale with the number of typed routes then the 25-route framing is
# incidental and the simpler harness is the better one.
#
# RSS is reported at two defined points — the instant the server first answers,
# and after a 500 ms settle — because a JIT allocates during warm-up and the two
# can differ by megabytes. Say which one any published number is.
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(cd -- "$SCRIPT_DIR/../../.." && pwd)"
SPAWNS="${SPAWNS:-15}"
ROUTE_COUNTS="${ROUTE_COUNTS:-1 25}"
OUT="${OUT:-$SCRIPT_DIR/results.txt}"

export GOWORK=off

cd "$SCRIPT_DIR/app"
go mod edit -replace github.com/go-kruda/kruda="$CORE_DIR"
go mod tidy >/dev/null 2>&1
go build -o /tmp/coldstart-sonic .
go build -tags kruda_stdjson -o /tmp/coldstart-stdjson .

cd "$SCRIPT_DIR/driver"
go build -o /tmp/coldstart-driver .

{
  echo "# cold start and startup RSS, $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "# $(go version)"
  echo "# $(nproc 2>/dev/null || sysctl -n hw.ncpu) cores"
  echo "# binary sizes: sonic $(stat -c%s /tmp/coldstart-sonic 2>/dev/null || stat -f%z /tmp/coldstart-sonic) B, stdjson $(stat -c%s /tmp/coldstart-stdjson 2>/dev/null || stat -f%z /tmp/coldstart-stdjson) B"
  echo
  for n in $ROUTE_COUNTS; do
    /tmp/coldstart-driver -bin /tmp/coldstart-stdjson -routes "$n" -spawns "$SPAWNS" -port 3500 -label "encoding/json"
    /tmp/coldstart-driver -bin /tmp/coldstart-sonic   -routes "$n" -spawns "$SPAWNS" -port 3600 -label "sonic"
    echo
  done
} | tee "$OUT"

echo "results written to $OUT"
