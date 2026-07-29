# Wing blocking advisor — A/B/C on Linux, 2026-07-29

Does the rate-based advisor cost throughput, and does the per-worker tallying
that keeps it off the hot path earn its complexity?

## Setup

| | |
|---|---|
| Host | 8-core 13th Gen Intel i5-13500, Linux, shared box, load < 0.5 at start |
| Go | pinned `GOTOOLCHAIN=go1.25.11` |
| Build | `-tags kruda_stdjson` for all variants, so the JSON engine is identical and the advisor is the only difference |
| Server | `bench/reproducible/kruda`, `KRUDA_WORKERS=8 GOMAXPROCS=8`, inline CPU dispatch (byte-identical across all three commits) |
| Load | `wrk -t4 -c256`, 3 s warm-up, 10 s measured |
| Design | 5 rounds, variants interleaved round-robin, order reversed on even rounds so drift on a shared box cancels |

| id | commit | advisor |
|---|---|---|
| A | `2d6eeac` (main) | warns on an absolute count of 10 long requests; observes only requests over 100 µs |
| B | `730a785` | rate-based; shared atomic per request, key allocated on every route change |
| C | `feb1c1b` | rate-based; per-worker tallies, no atomic and no allocation on the hot path |

Noise band per workload is A's own spread across the 5 rounds.

## Throughput

| workload | noise band | A (main) | B | C |
|---|---|---|---|---|
| single route `/plaintext-handler` | ±2.43% | 744,425 | 745,072 (+0.09%, inside) | 738,343 (−0.82%, inside) |
| 4 routes interleaved | ±0.55% | 682,151 | 671,205 (**−1.60%, outside**) | 677,158 (−0.73%, inside) |
| param route, 200 ids | ±1.80% | 634,775 | 632,679 (−0.33%, inside) | 633,399 (−0.22%, inside) |

C vs B on the interleaved workload, paired by round: −0.69%, +0.75%, +1.91%,
+1.22%, +1.28% — mean **+0.89% for C**. Round 1 is the only round favouring B
and is the warm-up round.

## False-positive warnings — the decisive result

Total advisor warnings emitted across the 5 rounds. Every one is a claim that a
handler blocked the event loop; none of these handlers do anything but write a
constant string or a small JSON object.

| variant | single | interleaved | param |
|---|---|---|---|
| A (main) | 5 | 20 | **518** |
| B | 0 | 0 | 0 |
| C | 0 | 0 | 0 |

On the param route main warns about **~104 distinct paths per run**, at
`blocked=` values from 100 µs to 7.5 ms that are pure scheduler delay under
load. Each concrete path is its own advisor key, so every one reaches the
absolute threshold of 10 and warns once. Example from the raw logs:

```
WARN kruda: route is annotated for inline dispatch but blocked the event loop
     route="GET /users/172" blocked=2.81ms count=10
```

The warning storm costs no measurable throughput — A is still fastest or equal
everywhere — so this is purely a correctness and usability defect, and a loud
one.

## Conclusions

1. **The rate-based advisor is justified.** It removes a warning storm that main
   produces under ordinary load, and costs nothing outside the noise band on the
   single-route shape the published benchmarks use.
2. **The contended atomic in B is free on a single route** (+0.09%, inside the
   band) but costs **1.60% on multi-route traffic**, outside the band. C recovers
   most of that (−0.73%, inside the band) and beats B by 0.89% paired.
3. Multi-route is the shape real applications serve. The single-route benchmark
   is the one case where B's per-request allocation never fires.

Reproduce with `./advisor-ab.sh` (expects the three commits cloned under
`/home/tiger/kruda-advisor-ab/{A,B,C}` and built as `server-{A,B,C}`).
