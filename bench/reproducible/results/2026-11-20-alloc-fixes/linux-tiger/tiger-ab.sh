#!/bin/bash
# Linux A/B for perf/alloc-fixes: in-process benchmarks, then wrk HTTP.
set -eu
export PATH=$PATH:/snap/bin:$HOME/go/bin GOTOOLCHAIN=go1.26.11
W=$1; cd "$W"
git clone -q alloc.bundle new -b perf/alloc-fixes
cd new
git fetch -q ../alloc.bundle 'refs/heads/*:refs/remotes/b/*' || true
bench/reproducible/results/2026-11-20-alloc-fixes/run-ab.sh fba66a1 "$W/inproc" > "$W/inproc.log" 2>&1
# HTTP: build the bench server from each tree
mkdir -p "$W/base"; git archive fba66a1 | tar -x -C "$W/base"
for side in base new; do
  d=$W/new; [ $side = base ] && d=$W/base
  (cd "$d/bench/reproducible/kruda" && go build -tags kruda_stdjson -o "$W/srv-$side" .)
done
printf 'wrk.method="POST"\nwrk.body=%s\nwrk.headers["Content-Type"]="application/json"\n' "'{\"name\":\"tiger\",\"value\":42}'" > "$W/post.lua"
H=$W/http; mkdir -p $H
port=3471
for i in 1 2 3 4 5 6; do
  for side in base new; do
    PORT=$port GOMAXPROCS=8 KRUDA_WORKERS=4 taskset -c 0-7 "$W/srv-$side" > "$H/srv-$side-$i.log" 2>&1 &
    pid=$!; sleep 2
    for r in plaintext-handler json-serialize post; do
      if [ $r = post ]; then url=http://127.0.0.1:$port/json; s="-s $W/post.lua"; else url=http://127.0.0.1:$port/$r; s=""; fi
      taskset -c 8-15 wrk -t4 -c128 -d10s $s "$url" > "$H/$r-$side-$i.txt" 2>&1
    done
    kill $pid; wait $pid 2>/dev/null || true
    sleep 1
  done
done
cd $H; for r in plaintext-handler json-serialize post; do for side in base new; do
  printf '%s %s ' $r $side; grep -h Requests/sec $r-$side-*.txt | awk '{printf "%s ", $2}'; echo; done; done > summary.txt
touch "$W/DONE"
