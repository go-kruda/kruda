#!/bin/bash
# Paired A/B: alternate base and new, 6 rounds, same benchmarks.
set -u
S=/private/tmp/claude-501/-Users-emetworks-Project-HOBBY-framework-kruda/2269c768-1ade-4b47-ac35-d1801d977a1c/scratchpad
OUT=$S/ab-$1; mkdir -p $OUT; rm -f $OUT/base.txt $OUT/new.txt
export GOCACHE=/tmp/kruda-ab-gocache GOMODCACHE=/tmp/kruda-ab-modcache
PAT='^(BenchmarkPerf|BenchmarkDI_UseTransient$|BenchmarkCompiledValidation|BenchmarkTypedHandlerValidation$|BenchmarkCPUParsePOST$|BenchmarkCPUParseGET$|BenchmarkCPUResponsePlaintext$|BenchmarkCPUResponseJSON$)'
for i in 1 2 3 4 5 6; do
  (cd $S/base-fba66a1 && go test -tags kruda_stdjson -run '^$' -bench "$PAT" -benchmem -count=1 . >> $OUT/base.txt 2>&1)
  (cd /Users/emetworks/Project/HOBBY/framework/kruda-phase2 && go test -tags kruda_stdjson -run '^$' -bench "$PAT" -benchmem -count=1 . >> $OUT/new.txt 2>&1)
done
echo done > $OUT/DONE
