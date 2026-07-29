#!/usr/bin/env bash
# A/B/C the Wing blocking advisor across three commits. v2: robust teardown,
# free-port probing, per-run retry, and advisor-warning counts as a first-class
# result alongside throughput.
set -uo pipefail

BASE=/home/tiger/kruda-advisor-ab
export GOTOOLCHAIN=go1.25.11
export PATH=/snap/bin:$PATH

ROUNDS="${ROUNDS:-5}"
DURATION="${DURATION:-10s}"
WARMUP="${WARMUP:-3}"
WORKERS="${WORKERS:-8}"

cd "$BASE"
RESULTS="$BASE/results2.csv"
echo "round,order,variant,workload,rps,p50_ms,p99_ms,warnings" > "$RESULTS"

port_free() {
  ! ss -ltn 2>/dev/null | grep -q ":$1 "
}

next_port() {
  while :; do
    PORT_N=$((PORT_N + 1))
    [ "$PORT_N" -gt 34900 ] && PORT_N=34200
    port_free "$PORT_N" && { echo "$PORT_N"; return; }
  done
}
PORT_N=34200

to_ms() {
  case "$1" in
    *us) echo "scale=3; ${1%us}/1000" | bc ;;
    *ms) echo "${1%ms}" ;;
    *s)  echo "scale=3; ${1%s}*1000" | bc ;;
    *)   echo "0" ;;
  esac
}

attempt() {
  local v=$1 wl=$2 log=$3
  local p; p=$(next_port)
  local url="http://127.0.0.1:$p/plaintext-handler"
  local extra=()
  case "$wl" in
    interleaved) extra=(-s "$BASE/interleaved.lua"); url="http://127.0.0.1:$p/" ;;
    param)       extra=(-s "$BASE/param.lua");       url="http://127.0.0.1:$p/" ;;
  esac

  KRUDA_WORKERS=$WORKERS GOMAXPROCS=$WORKERS PORT=$p \
    "$BASE/server-$v" >"$log" 2>&1 &
  local pid=$!

  local ready=0 i
  for i in $(seq 1 40); do
    if curl -fsS -o /dev/null --max-time 1 "http://127.0.0.1:$p/plaintext-handler" 2>/dev/null; then
      ready=1; break
    fi
    kill -0 $pid 2>/dev/null || break
    sleep 0.25
  done
  if [ "$ready" -ne 1 ]; then
    kill -9 $pid 2>/dev/null; wait $pid 2>/dev/null
    return 1
  fi

  wrk -t4 -c256 -d${WARMUP}s "${extra[@]}" "$url" >/dev/null 2>&1
  local out; out=$(wrk --latency -t4 -c256 -d"$DURATION" "${extra[@]}" "$url" 2>&1)

  kill -9 $pid 2>/dev/null; wait $pid 2>/dev/null

  RPS=$(awk '/Requests\/sec/{print $2}' <<<"$out")
  P50=$(awk '/^ *50%/{print $2}' <<<"$out")
  P99=$(awk '/^ *99%/{print $2}' <<<"$out")
  [ -n "$RPS" ] || return 1
  WARN=$(grep -c "blocked the event loop" "$log" 2>/dev/null || echo 0)
  return 0
}

run_one() {
  local round=$1 order=$2 v=$3 wl=$4
  local log="$BASE/log2-$v-$round-$wl.log"
  local try
  for try in 1 2 3; do
    if attempt "$v" "$wl" "$log"; then
      echo "$round,$order,$v,$wl,$RPS,$(to_ms "${P50:-0}"),$(to_ms "${P99:-0}"),$WARN" >> "$RESULTS"
      echo "  r$round $order $v/$wl -> $RPS rps  p99=${P99:-NA}  warnings=$WARN"
      return 0
    fi
    echo "  r$round $order $v/$wl attempt $try failed, retrying" >&2
    sleep 2
  done
  echo "$round,$order,$v,$wl,NA,0,0,NA" >> "$RESULTS"
  echo "  r$round $order $v/$wl -> FAILED after 3 attempts" >&2
}

for round in $(seq 1 "$ROUNDS"); do
  if [ $((round % 2)) -eq 1 ]; then order="ABC"; vs="A B C"; else order="CBA"; vs="C B A"; fi
  echo "== round $round ($order) =="
  for wl in single interleaved param; do
    for v in $vs; do run_one "$round" "$order" "$v" "$wl"; done
  done
done

echo "DONE" > "$BASE/done2.marker"
echo "results at $RESULTS"
