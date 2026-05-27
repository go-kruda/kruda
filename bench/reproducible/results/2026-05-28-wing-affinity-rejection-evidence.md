# Wing CPU Affinity Rejection Evidence

This note records a Phase 5 Linux worker CPU-affinity prototype that was tested
and rejected. The prototype was opt-in only and was reverted before this
evidence was prepared, so the final branch has no runtime behavior change.

## Scope

- Host: tiger development server
- Branch under test: `perf/wing-phase5-affinity-prototype`
- Base commit: `25bdf09` (`bench: add Phase 5 v1.2.4 baseline evidence`)
- Prototype commits tested:
  - `989355f` added `KRUDA_WING_CPU_AFFINITY=1` with worker thread affinity plus
    `SO_INCOMING_CPU` listener hints.
  - `f940797` isolated the candidate to worker thread affinity only.
- Route: `GET /plaintext-handler`
- Profiles:
  - latency: `-t4 -c128 -d10s`
  - throughput: `-t4 -c256 -d10s`
- Rounds: 3 measured rounds plus warmup
- Runtime: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`

## Evidence Directories

- `phase5-affinity-off-smoke-20260527T201500Z/`: no affinity baseline,
  `KRUDA_WING_CPU_AFFINITY=0`.
- `phase5-affinity-on-smoke-20260527T202100Z/`: worker thread affinity plus
  `SO_INCOMING_CPU`, `KRUDA_WING_CPU_AFFINITY=1`.
- `phase5-affinity-thread-only-smoke-20260527T202800Z/`: worker thread affinity
  only, `KRUDA_WING_CPU_AFFINITY=1`.

## Median Results

| Candidate | Profile | Kruda median RPS | Delta vs no affinity | Kruda median p99 | Delta vs no affinity | Socket errors | Non-2xx |
|---|---|---:|---:|---:|---:|---:|---:|
| no affinity | latency | 799717.86 | baseline | 0.774 ms | baseline | 0 | 0 |
| affinity + `SO_INCOMING_CPU` | latency | 756540.39 | -5.40% | 0.664 ms | -14.21% | 0 | 0 |
| thread affinity only | latency | 831611.22 | +3.99% | 1.210 ms | +56.33% | 0 | 0 |
| no affinity | throughput | 823526.65 | baseline | 0.786 ms | baseline | 0 | 0 |
| affinity + `SO_INCOMING_CPU` | throughput | 796379.86 | -3.30% | 0.970 ms | +23.41% | 0 | 0 |
| thread affinity only | throughput | 807598.17 | -1.93% | 1.390 ms | +76.84% | 0 | 0 |

## Decision

Reject the CPU-affinity candidate.

The combined affinity plus `SO_INCOMING_CPU` candidate regressed RPS in both
profiles. The thread-only candidate improved latency-profile RPS, but it
regressed p99 heavily and regressed throughput-profile RPS and p99. It does not
clear the balanced CPU-bound performance gate, and it is not a path toward a
broad 20% fair-handler advantage over Actix.

No runtime code from this candidate should be merged. Keep future Phase 5 Track
A work focused on a different kernel/event-loop hypothesis with a one-variable
benchmark design.
