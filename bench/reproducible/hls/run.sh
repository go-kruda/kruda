#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
root=$PWD
mode=${1:-release}
out=${2:-"$root/runs/$mode-$(date +%Y%m%d-%H%M%S)"}
out=$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve())' "$out")

unset HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy
unset GOGC GOMEMLIMIT GODEBUG KRUDA_TRANSPORT TIINNO_BENCH_WING_COPY_256
export NO_PROXY=127.0.0.1,localhost

cd bin
if [[ "$mode" == release ]]; then
    python3 "$root/runner/release.py"
    exit 0
fi

if [[ "$mode" == smoke ]]; then
    cpu=$(python3 -c 'import os; print(min(os.sched_getaffinity(0)))')
    roles=(--proxy-cpu "${HLS_PROXY_CPUS:-$cpu}" --origin-cpu "${HLS_ORIGIN_CPUS:-$cpu}" --load-cpu "${HLS_DRIVER_CPUS:-$cpu}")
    runner=long
    viewers=64
    rounds=1
    cycles=1
    limit=3m
else
    roles=(--proxy-cpu "${HLS_PROXY_CPUS:-12,13}" --origin-cpu "${HLS_ORIGIN_CPUS:-8,9,10,11}" --load-cpu "${HLS_DRIVER_CPUS:-4,5,6,7}")
    rounds=5
    ulimit -n 65535
    case "$mode" in
        short) runner=short; viewers=6144; cycles=2; limit=8m ;;
        long) runner=long; viewers=11776; cycles=10; limit=22m ;;
        *) echo "usage: bash run.sh release|smoke|short|long [output-directory]" >&2; exit 2 ;;
    esac
fi

mkdir -p "$(dirname "$out")"
mkdir "$out"
launch=(nice -n 5)
if [[ "$mode" != smoke ]]; then
    launch=(taskset -c "${HLS_COORDINATOR_CPU:-2}" "${launch[@]}")
fi
pairs=(--pair "wing-256:$viewers" --pair "fiber:$viewers" --pair "actix-tuned:$viewers")
"${launch[@]}" timeout --signal=TERM --kill-after=10s 90s python3 -u "$root/runner/$runner.py" \
    --preflight --rounds 1 --warm 1 --cycles 1 --output "$out/preflight" --label preflight "${roles[@]}" "${pairs[@]}"
"${launch[@]}" timeout --signal=TERM --kill-after=10s "$limit" python3 -u "$root/runner/$runner.py" \
    --rounds "$rounds" --warm 1 --cycles "$cycles" --output "$out/score" --label "$mode" "${roles[@]}" "${pairs[@]}"

if [[ "$mode" == smoke ]]; then
    python3 - "$out/score/results.jsonl" <<'PY'
import json
import sys
from pathlib import Path

rows = [json.loads(line) for line in Path(sys.argv[1]).read_text().splitlines()]
assert {row["stack"] for row in rows} == {"wing-256", "fiber", "actix-tuned"}
assert len(rows) == 3
for row in rows:
    assert row["attempts"] == row["ok"] == row["warm"]["ok"] == 64, row
    assert row["origin_hits_including_warm"] == 128, row
print("PASS: all three stacks completed 64 warm and 64 scored segments")
PY
fi
