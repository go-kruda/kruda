# Wing Phase 5 Candidate Evidence

This evidence records Phase 5 fair handler-path experiments on the tiger Linux benchmark server. It is intentionally evidence-only: no runtime behavior change is accepted from these candidate runs.

## Scope

- Server source: `144e1f6`, same runtime code as the current Phase 5 baseline.
- Workload: CPU-bound fair handler routes only.
- Routes: `plaintext-handler`, `json-static`, and `json-serialize`.
- Common profile: loopback `wrk --latency -t4 -c256 -d10s` with a warmup round.
- Error gate: all recorded rows in these result directories had zero socket errors and zero non-2xx responses.

## Result Directories

- `phase5-worker-sweep-raw-20260528T035902Z`
- `phase5-postsend-read-candidate-20260528T040554Z`
- `phase5-go-toolchain-compare-20260528T041141Z`
- `phase5-cpu-spear-candidate-20260528T041735Z`
- `phase5-gomaxprocs-sweep-20260528T042156Z`

## Conclusions

### Worker Sweep

`KRUDA_WORKERS=4` remains the balanced setting for this machine and profile.

`KRUDA_WORKERS=5` produced slightly higher one-round plaintext and static JSON throughput, but p99 latency regressed sharply and `json-serialize` throughput was lower:

| Workers | plaintext RPS | json-static RPS | json-serialize RPS | p99 range |
|---:|---:|---:|---:|---:|
| 4 | 802088.20 | 821874.10 | 807711.76 | 0.910-1.200 ms |
| 5 | 820561.39 | 822854.99 | 787711.01 | 2.150-3.220 ms |

`KRUDA_WORKERS=3` had lower throughput, while `6` and `8` had worse tail latency without a throughput win.

### Post-send Read Removal

Removing the speculative post-send read from `directSend` was rejected. It did not produce a balanced win in the repeated comparison:

| Route | Baseline median RPS | Candidate median RPS | Change |
|---|---:|---:|---:|
| plaintext-handler | 839060.25 | 815442.86 | -2.81% |
| json-static | 816138.36 | 813505.52 | -0.32% |
| json-serialize | 809137.90 | 780985.73 | -3.48% |

The candidate sometimes improved p99 on plaintext and serialized JSON, but the throughput regression makes it unsuitable for the balanced CPU-bound gate.

### Go Toolchain Compare

Go 1.26.3 was not a broad free win over Go 1.25.8 on this source and benchmark profile:

| Route | Go 1.25.8 median RPS | Go 1.26.3 median RPS | Change |
|---|---:|---:|---:|
| plaintext-handler | 819477.29 | 805401.13 | -1.72% |
| json-static | 800678.11 | 812893.45 | +1.53% |
| json-serialize | 798459.50 | 788012.75 | -1.31% |

This does not justify changing the published baseline or claim wording.

### CPU Spear Candidate

Changing the benchmark CPU routes from the normal handler dispatch hints to `WingQuery`/Spear takeover was rejected. It preserved zero errors, but throughput collapsed and tail latency became unacceptable:

| Route | Baseline median RPS | Candidate median RPS | Candidate median p99 |
|---|---:|---:|---:|
| plaintext-handler | 828524.05 | 313614.95 | 334.800 ms |
| json-static | 805051.08 | 307023.97 | 215.230 ms |
| json-serialize | 803765.64 | 301272.72 | 212.880 ms |

Spear remains an IO/DB-oriented scheduling feather, not a CPU-bound fair-handler throughput option.

### GOMAXPROCS Sweep

No `GOMAXPROCS` setting produced a broad balanced win with `KRUDA_WORKERS=4`.

`GOMAXPROCS=10` had the best one-round plaintext result, but it regressed `json-serialize` throughput and p99 compared with `GOMAXPROCS=8`. `GOMAXPROCS=6` was competitive, but not enough to justify changing the current benchmark recommendation.

## Decision

Keep the current fair-handler Phase 5 baseline settings:

- `KRUDA_WORKERS=4`
- `GOMAXPROCS=8`
- `KRUDA_READ_BUF_SIZE=4096`

These experiments do not support a new runtime change or a 20% Actix win claim. The evidence supports the current wording that Kruda is in the same ballpark as Actix for CPU-bound fair handler routes, with some routes ahead and others limited by tail latency or resource tradeoffs.
