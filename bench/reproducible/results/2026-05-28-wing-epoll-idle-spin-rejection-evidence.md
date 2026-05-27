# Wing Epoll Idle Spin Rejection Evidence

This note records a Phase 5 Linux epoll idle-spin prototype that was tested and
rejected. The prototype was opt-in only and was reverted before this evidence
was prepared, so the final branch has no runtime behavior change.

## Scope

- Host: tiger development server
- Branch under test: `perf/wing-phase5-epoll-wait-prototype`
- Base commit: `7877248` (`bench: record rejected Wing CPU affinity prototype`)
- Prototype commit tested: `d46d9c1` added `KRUDA_WING_EPOLL_IDLE_SPINS`
- Route: `GET /plaintext-handler`
- Profiles:
  - latency: `-t4 -c128 -d10s`
  - throughput: `-t4 -c256 -d10s`
- Rounds: 3 measured rounds plus warmup
- Runtime: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- Ports: `KRUDA_PORT=13100`, `FIBER_PORT=13102`, `ACTIX_PORT=13103`

## Evidence Directories

- `phase5-epoll-idle-default-smoke-20260527T205813Z/`: default idle spin
  policy, equivalent to 64 empty non-blocking waits before blocking.
- `phase5-epoll-idle-0-smoke-20260527T210231Z/`:
  `KRUDA_WING_EPOLL_IDLE_SPINS=0`.
- `phase5-epoll-idle--1-smoke-20260527T210647Z/`:
  `KRUDA_WING_EPOLL_IDLE_SPINS=-1`.
- `phase5-epoll-idle-256-smoke-20260527T211130Z/`:
  `KRUDA_WING_EPOLL_IDLE_SPINS=256`.

## Median Kruda Results

| Candidate | Profile | Kruda median RPS | Delta vs default | Kruda median p99 | Delta vs default | Socket errors | Non-2xx |
|---|---|---:|---:|---:|---:|---:|---:|
| default | latency | 833482.85 | baseline | 0.589 ms | baseline | 0 | 0 |
| idle spins 0 | latency | 790854.40 | -5.11% | 1.240 ms | +110.53% | 0 | 0 |
| idle spins -1 | latency | 790447.16 | -5.16% | 1.380 ms | +134.30% | 0 | 0 |
| idle spins 256 | latency | 815311.02 | -2.18% | 0.980 ms | +66.38% | 0 | 0 |
| default | throughput | 806429.69 | baseline | 0.960 ms | baseline | 0 | 0 |
| idle spins 0 | throughput | 797753.37 | -1.08% | 1.220 ms | +27.08% | 0 | 0 |
| idle spins -1 | throughput | 794591.89 | -1.47% | 1.500 ms | +56.25% | 0 | 0 |
| idle spins 256 | throughput | 810179.70 | +0.47% | 0.841 ms | -12.40% | 0 | 0 |

## Decision

Reject the epoll idle-spin candidate.

Lower spin limits (`0` and `-1`) regressed both RPS and p99. A higher spin
limit (`256`) slightly improved throughput-profile RPS and p99, but it
regressed latency-profile RPS and p99 heavily. It does not clear the balanced
CPU-bound performance gate, and it is not a path toward a broad 20% fair-handler
advantage over Actix.

No runtime code from this candidate should be merged. Keep future Phase 5 Track
A work focused on a different kernel or event-loop hypothesis with one variable
at a time.
