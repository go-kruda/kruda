#!/bin/bash
# Paired A/B for the 2026-11-20 allocation fixes. Run from the repo root of the
# candidate checkout. BASE is the baseline revision; the script exports it to a
# temporary tree and copies in the benchmark files, which do not exist there.
#   bench/reproducible/results/2026-11-20-alloc-fixes/run-ab.sh [BASE] [OUT]
set -eu
BASE=${1:-fba66a1}
OUT=${2:-$(mktemp -d)}
mkdir -p "$OUT"
OUT=$(cd "$OUT" && pwd) # the benchmark subshells change directory
NEW=$(pwd)
{
  echo "base: $(git rev-parse "$BASE")"
  echo "candidate: $(git rev-parse HEAD)$(git diff --quiet HEAD || echo ' +uncommitted changes')"
  echo "go: $(go version)"
  echo "host: $(uname -sm)"
} > "$OUT/run-info.txt"
TREE=$(mktemp -d)
git archive "$BASE" | tar -x -C "$TREE"
cp perf_alloc_bench_test.go wing_perf_alloc_bench_test.go wing_perf_loopback_bench_test.go "$TREE/"
cp contrib/session/memory_store_bench_test.go "$TREE/contrib/session/"
CORE='^(BenchmarkPerf(Validate|NetHTTP|JSONPool|WingBody|WingHEAD)|BenchmarkDI_UseTransient$|BenchmarkCompiledValidation|BenchmarkTypedHandlerValidation$|BenchmarkCPUParsePOST$|BenchmarkCPUParseGET$|BenchmarkCPUResponsePlaintext$|BenchmarkCPUResponseJSON$)'
WING='^(BenchmarkPerfLoopback|BenchmarkCPUParseGET$|BenchmarkCPUParsePOST$|BenchmarkCPUResponseJSON$|BenchmarkCPUResponsePlaintext$|BenchmarkLoopbackPlaintext$|BenchmarkLoopbackJSON$)'
: > "$OUT/core-base.txt"; : > "$OUT/core-new.txt"; : > "$OUT/wing-base.txt"; : > "$OUT/wing-new.txt"
: > "$OUT/session-base.txt"; : > "$OUT/session-new.txt"
for i in 1 2 3 4 5 6; do
  for side in base new; do
    dir=$TREE; [ "$side" = new ] && dir=$NEW
    (cd "$dir" && go test -tags kruda_stdjson -run '^$' -bench "$CORE" -benchmem -count=1 . >> "$OUT/core-$side.txt")
    (cd "$dir" && go test -tags kruda_stdjson -run '^$' -bench "$WING" -benchtime=2000x -benchmem -count=1 . >> "$OUT/wing-$side.txt")
    (cd "$dir/contrib/session" && GOWORK=off go test -run '^$' -bench MemoryStoreGetParallel -benchmem -cpu 1,8 -count=1 . >> "$OUT/session-$side.txt")
  done
done
for k in core wing session; do benchstat "$OUT/$k-base.txt" "$OUT/$k-new.txt" > "$OUT/$k-benchstat.txt"; done
echo "results in $OUT"
