# Wing CPU Affinity A/B Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`

## Summary

An opt-in Wing CPU affinity prototype pinned each Linux Wing worker event-loop
thread to `workerID % runtime.NumCPU()` after `runtime.LockOSThread`. It was
exposed temporarily through `KRUDA_CPU_AFFINITY=1` and measured with the
reproducible Kruda-only CPU-bound benchmark.

The prototype was rejected and removed because it regressed median RPS across
all measured CPU-bound handler routes.

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/cpu-affinity-ab-20260604T150831Z`

## Result

| Route | Profile | Control median RPS | Affinity median RPS | Delta | Control median p99 ms | Affinity median p99 ms | Errors |
|---|---|---:|---:|---:|---:|---:|---:|
| `json-serialize` | latency | 824,744.29 | 803,798.46 | -2.54% | 0.795 | 0.654 | 0 |
| `json-serialize` | throughput | 821,418.95 | 806,983.20 | -1.76% | 0.688 | 0.890 | 0 |
| `json-static` | latency | 809,515.25 | 801,985.66 | -0.93% | 0.786 | 0.707 | 0 |
| `json-static` | throughput | 817,589.33 | 815,036.14 | -0.31% | 0.721 | 0.910 | 0 |
| `plaintext-handler` | latency | 836,726.50 | 783,584.99 | -6.35% | 0.980 | 0.816 | 0 |
| `plaintext-handler` | throughput | 824,901.00 | 792,371.96 | -3.94% | 0.870 | 0.960 | 0 |

## Interpretation

- CPU affinity did not produce a broad throughput win on tiger.
- Some latency-profile p99 values improved, but the throughput regression was
  too large and inconsistent with the balanced performance gate.
- Do not reintroduce a Wing CPU affinity option unless a new benchmark host or
  a separate-client setup shows repeatable RPS gains with p99 no worse than
  control.
