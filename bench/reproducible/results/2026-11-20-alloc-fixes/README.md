# Allocation fixes, 2026-11-20

Paired A/B of branch `perf/alloc-fixes` against `fba66a1` on macOS arm64
(Apple M3, 8 cores), Go 1.26.11 (the toolchain the module selects),
`-tags kruda_stdjson`. In-process benchmarks; not HTTP throughput, and not run
on Linux. `run-info.txt` records the exact base and candidate revisions of the
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
