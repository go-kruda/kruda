#!/usr/bin/env bash
# Soak gate: sustained wrk load + SSE connect/disconnect churn, asserting
# goroutine-count and RSS stability. PASS/FAIL via exit code.
#
# Usage: DURATION=600 PORT=3499 ./soak.sh
#   DURATION  seconds of load (default 600; use 3600 for the tiger gate run)
#   PORT      server port (default 3499 — tiger convention: 34xx)
set -euo pipefail

DURATION=${DURATION:-600}
PORT=${PORT:-3499}
BASE="http://127.0.0.1:${PORT}"
DIR=$(cd "$(dirname "$0")" && pwd)
OUT="${DIR}/results/soak-$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$OUT"

command -v wrk >/dev/null || { echo "FAIL: wrk is required"; exit 1; }

echo "== build =="
(cd "$DIR/soak" && GOWORK=off go build -o "$OUT/kruda-soak" .)

echo "== start server =="
SOAK_PORT=$PORT "$OUT/kruda-soak" >"$OUT/server.log" 2>&1 &
SRV=$!
# WRK/CHURN aren't assigned until the load-generation section below; under
# set -u, an EXIT trap that references them before that would itself fail
# with "unbound variable" and abort before reaching `kill`, orphaning SRV.
# Bind them empty now so the trap can always expand them safely (word
# splitting drops the empty values, so `kill` never sees a bad argument).
WRK=""
CHURN=""
trap 'kill $SRV $WRK $CHURN 2>/dev/null || true' EXIT
sleep 2

sample() { # $1 = label
  local g rss
  g=$(curl -sf "$BASE/soakstats" | sed -E 's/.*"goroutines":([0-9]+).*/\1/')
  rss=$(ps -o rss= -p $SRV | tr -d ' ')
  echo "$(date -u +%s),$1,$g,$rss" >>"$OUT/samples.csv"
  echo "sample $1: goroutines=$g rss_kb=$rss"
}

echo "== warmup 10s =="
wrk -t2 -c64 -d10s "$BASE/plaintext" >/dev/null
sleep 2
sample baseline
BASE_G=$(tail -1 "$OUT/samples.csv" | cut -d, -f3)
BASE_RSS=$(tail -1 "$OUT/samples.csv" | cut -d, -f4)

echo "== load ${DURATION}s: wrk + SSE churn =="
wrk -t2 -c64 -d"${DURATION}s" "$BASE/json" >"$OUT/wrk.log" 2>&1 &
WRK=$!
(
  end=$((SECONDS + DURATION))
  while [ $SECONDS -lt $end ]; do
    curl -sN --max-time 2 "$BASE/events" | head -c 200 >/dev/null || true
    sleep 0.1
  done
) &
CHURN=$!
while kill -0 $WRK 2>/dev/null; do
  sleep 15
  sample load || true
done
wait $CHURN 2>/dev/null || true

echo "== settle 10s =="
sleep 10
sample final
FIN_G=$(tail -1 "$OUT/samples.csv" | cut -d, -f3)
FIN_RSS=$(tail -1 "$OUT/samples.csv" | cut -d, -f4)

echo "== verdict =="
echo "goroutines: baseline=$BASE_G final=$FIN_G (limit: baseline+50)"
echo "rss_kb:     baseline=$BASE_RSS final=$FIN_RSS (limit: baseline*1.5)"
FAIL=0
[ "$FIN_G" -le $((BASE_G + 50)) ] || { echo "FAIL: goroutine growth"; FAIL=1; }
[ "$FIN_RSS" -le $((BASE_RSS * 3 / 2)) ] || { echo "FAIL: RSS growth"; FAIL=1; }
[ $FAIL -eq 0 ] && echo "SOAK PASS ($OUT)"
exit $FAIL
