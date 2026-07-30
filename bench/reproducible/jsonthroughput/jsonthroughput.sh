#!/usr/bin/env bash
# Does swapping the JSON engine change req/sec, and on which payload shapes?
#
# The engine benchmarks show Sonic ~35% faster to encode and ~6× faster to decode
# than encoding/json. That is the cost of one function call, not of one request.
# This measures whether it reaches throughput, on the three shapes those
# benchmarks cover.
#
# The two arms are the same binary built with and without kruda_stdjson, run
# round-robin with the order reversed on even rounds, because this box is shared.
# The noise band is the spread of one arm across its own rounds.
set -uo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(cd -- "$SCRIPT_DIR/../../.." && pwd)"
ROUNDS="${ROUNDS:-5}"
DURATION="${DURATION:-10s}"
WARMUP="${WARMUP:-3}"
WORKERS="${WORKERS:-8}"
OUT="${OUT:-$SCRIPT_DIR/results.csv}"

export GOWORK=off PATH=/snap/bin:$PATH

cd "$SCRIPT_DIR/app"
go mod edit -replace github.com/go-kruda/kruda="$CORE_DIR"
go mod tidy >/dev/null 2>&1
go build -o /tmp/jt-sonic .
go build -tags kruda_stdjson -o /tmp/jt-stdjson .

# A POST body matching the /large payload, so /decode is measured on the same shape.
curl -s -o /tmp/jt-body.json "http://127.0.0.1:1/" 2>/dev/null || true
PORT=3999 /tmp/jt-sonic >/dev/null 2>&1 &
seed=$!
for _ in $(seq 1 200); do curl -fsS -o /tmp/jt-body.json http://127.0.0.1:3999/large 2>/dev/null && break; sleep 0.05; done
kill -9 $seed 2>/dev/null; wait $seed 2>/dev/null
echo "POST body: $(wc -c < /tmp/jt-body.json) bytes"

cat > /tmp/jt-post.lua <<'LUA'
local f = io.open("/tmp/jt-body.json", "rb")
local body = f:read("*all")
f:close()
wrk.method = "POST"
wrk.body = body
wrk.headers["Content-Type"] = "application/json"
LUA

echo "round,order,engine,route,rps,p99_ms" > "$OUT"
port=3700

run() {
  local round=$1 order=$2 eng=$3 route=$4
  port=$((port + 1))
  local extra=() url="http://127.0.0.1:$port/$route"
  [ "$route" = "decode" ] && extra=(-s /tmp/jt-post.lua)

  KRUDA_WORKERS=$WORKERS GOMAXPROCS=$WORKERS PORT=$port "/tmp/jt-$eng" >/dev/null 2>&1 &
  local pid=$!
  for _ in $(seq 1 200); do curl -fsS -o /dev/null "http://127.0.0.1:$port/ready" 2>/dev/null && break; sleep 0.05; done

  wrk -t4 -c256 -d${WARMUP}s "${extra[@]}" "$url" >/dev/null 2>&1
  local out; out=$(wrk --latency -t4 -c256 -d"$DURATION" "${extra[@]}" "$url" 2>&1)
  kill -9 $pid 2>/dev/null; wait $pid 2>/dev/null

  local rps p99
  rps=$(awk '/Requests\/sec/{print $2}' <<<"$out")
  p99=$(awk '/^ *99%/{print $2}' <<<"$out")
  echo "$round,$order,$eng,$route,${rps:-NA},${p99:-NA}" >> "$OUT"
  printf "  r%s %s %-8s %-7s %12s rps  p99=%s\n" "$round" "$order" "$eng" "$route" "${rps:-NA}" "${p99:-NA}"
}

for round in $(seq 1 "$ROUNDS"); do
  if [ $((round % 2)) -eq 1 ]; then order=AB; arms="stdjson sonic"; else order=BA; arms="sonic stdjson"; fi
  echo "== round $round ($order) =="
  for route in small large decode; do
    for eng in $arms; do run "$round" "$order" "$eng" "$route"; done
  done
done

echo DONE > "$SCRIPT_DIR/done.marker"
echo "results: $OUT"
