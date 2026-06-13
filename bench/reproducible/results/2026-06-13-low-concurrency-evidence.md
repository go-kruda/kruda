# Low-Concurrency Latency (P4)

Date: 2026-06-13 UTC
Host: tiger Linux dev server (`tiger-linux`), Go 1.25.10, wrk 4.1.0, default
(Sonic) build, kruda vs fiber, 3 rounds, `BENCH_LOWC=1` (adds c8/c16/c32 to the
standard c128/c256). Zero socket errors, zero non-2xx on every cell.
Scope: perf-track wave **P4** — characterize latency when the server is not
saturated (closer to many real services than c128/c256) and optimize only on
evidence.

## Question

The standard profiles are saturation (c128/c256). How does Kruda compare to
Fiber on p50/p99 at low concurrency, where real services often sit?

## Results — medians (Kruda vs Fiber)

| Conn | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| c8 | `plaintext` | 262,647 | 238,447 | +10.2% | 0.253 | 0.073 | +247% |
| c8 | `db` | 59,769 | 58,611 | +2.0% | 0.256 | 0.266 | −3.8% |
| c8 | `queries` | 58,965 | 57,882 | +1.9% | 0.265 | 0.269 | −1.5% |
| c16 | `plaintext` | 592,992 | 441,559 | +34.3% | 0.253 | 0.416 | −39.2% |
| c16 | `db` | 77,881 | 77,453 | +0.6% | 0.529 | 0.531 | −0.4% |
| c16 | `queries` | 77,721 | 76,349 | +1.8% | 0.535 | 0.560 | −4.5% |
| c32 | `plaintext` | 740,617 | 567,473 | +30.5% | 0.800 | 2.040 | −60.8% |
| c32 | `db` | 97,760 | 97,579 | +0.2% | 0.990 | 1.220 | −18.9% |
| c32 | `queries` | 97,193 | 95,289 | +2.0% | 1.130 | 1.310 | −13.7% |

## Findings

- **Kruda leads RPS at every concurrency** — `plaintext` +10% (c8) rising to +34%
  (c16) / +31% (c32); `db`/`queries` are pool-bound so they sit ~tied (+0.2 to
  +2.0%), as expected.
- **Kruda p99 is better or equal almost everywhere**, and the P1 spin keeps
  helping the DB tail even at low load (`db` c32 p99 −18.9%, `queries` −13.7%).
- **The single blemish: `plaintext` p99 at c8** — Kruda 0.253 ms vs Fiber
  0.073 ms. It is sub-millisecond and noisy (Kruda's three rounds were
  0.13/0.25/0.45 ms; Fiber's a stable 0.073), i.e. a 0.18 ms absolute gap at 8
  connections while Kruda still serves +10% more RPS. At c16 and above Kruda's
  `plaintext` p99 is already ahead (−39% / −61%).

## Decision

**Document; no optimization.** Low-concurrency latency is competitive-to-winning
across the board; Kruda leads RPS at every concurrency and leads p99 from c16 up.
The only spot Fiber is cleaner is the c8 `plaintext` tail — sub-millisecond,
noisy, and immaterial for real workloads, and chasing it would mean trading
Wing's saturation tuning (where Kruda wins by +27–34%) for a 0.18 ms gain at 8
connections. Not worth it.

Harness change only (`bench.sh` gains the opt-in `BENCH_LOWC=1` profiles); no
production code, so no RPS/p99 regression risk for this wave.
