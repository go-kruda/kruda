# Allocation fixes, 2026-11-20

Paired A/B of branch `perf/alloc-fixes` against `fba66a1` on macOS arm64
(Apple M3, 8 cores), Go 1.26.11 (the toolchain the module selects),
`-tags kruda_stdjson`. In-process benchmarks. Linux runs, including HTTP throughput, are in
`linux-tiger/`. `run-info.txt` records the exact base and candidate revisions of the
`core-*`, `wing-*` and `session-*` files.

| Files | What | How |
| --- | --- | --- |
| `core-*`, `wing-*`, `session-*` | All changed paths | `run-ab.sh fba66a1 <out>`: 6 alternating rounds per side. Wing loopback benchmarks use `-benchtime=2000x`, too few iterations for the ns-scale `CPU*` rows in `wing-*`; read those rows from `hot-path/` |
| `hot-path/parse-*` | Parse GET/POST after the owned-body check moved into the body branch | 5 rounds × `-count=4` per side (20 samples) |
| `hot-path/response-*` | JSON/plaintext responses, full cycle, inline handler | same |
| `hot-path/aa-*` | Noise floor: `fba66a1` against itself | same |

The `hot-path/` runs were taken at `2be3a5e`. The two later commits change
validation map labels, the body reader's EOF probe, Wing body accumulation and
stream buffers; none touches the parse or response code those runs measure.

## Linux (`linux-tiger/`)

Linux amd64, 16 cores, Go 1.26.11, candidate `bdef8ff` (same code as
`97124ac` plus docs) against `fba66a1`. `tiger-ab.sh` does all of it from a git
bundle of the branch.

| Files | What | Result |
| --- | --- | --- |
| `inproc/` | `run-ab.sh`, 6 alternating rounds | Same direction and size as macOS on every changed path |
| `http/` | `wrk -t4 -c128 -d10s` against `bench/reproducible/kruda` built from each tree, server on cores 0-7 with 4 Wing workers, wrk on 8-15, 6 alternating rounds; `summary.txt` lists req/s | Medians: plaintext 670k vs 669k, JSON 660k vs 654k, small JSON POST 511k vs 512k; spread between rounds is about 5% |
| `focus/` | Plaintext and JSON response, 20 samples, base, candidate and base again (A/A) | Plaintext +9.9% (30.0 vs 33.0 ns), A/A no difference |
| `focus3/` | Candidate with `appendStringTo` restored to the base code | Still +7.4%, so that change is not the cause |
| `focus5/` | Candidate server code with only the new and changed root test files reverted | 30.01 vs 30.07 ns, no difference: the gap is test-binary layout |
