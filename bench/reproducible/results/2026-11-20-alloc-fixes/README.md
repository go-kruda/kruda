# Allocation fixes, 2026-11-20

Paired A/B of branch `perf/alloc-fixes` against `fba66a1` on macOS arm64
(Apple M3, 8 cores), Go 1.26.1, `-tags kruda_stdjson`. In-process benchmarks;
not HTTP throughput, and not run on Linux.

| Files | What | How |
| --- | --- | --- |
| `core-*`, `wing-*`, `session-*` | All changed paths | `run-ab.sh fba66a1 <out>`: 6 alternating rounds per side. Wing loopback benchmarks use `-benchtime=2000x`, too few iterations for the ns-scale `CPU*` rows in `wing-*`; read those rows from `hot-path/` |
| `hot-path/parse-*` | Parse GET/POST after the owned-body check moved into the body branch | 5 rounds × `-count=4` per side (20 samples) |
| `hot-path/response-*` | JSON/plaintext responses, full cycle, inline handler | same |
| `hot-path/aa-*` | Noise floor: `fba66a1` against itself | same |

The `core-*`/`wing-*` runs predate the parse change in `hot-path/parse-*`; that
change moves one check and affects parse time only, not allocations.
