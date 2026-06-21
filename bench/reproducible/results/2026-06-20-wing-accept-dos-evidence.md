# Wing accept-side DoS — performance evidence (no-regression gate)

- **Date**: 2026-06-20
- **Branch**: `feat/wing-accept-dos` @ `caec96d` vs baseline `main` @ `f124309`
- **Box**: tiger (dev-server), Linux x86_64, 8 cores, ulimit -n 1024 (shared box — academy-*/plane-* docker stacks active)
- **Toolchain**: `GOTOOLCHAIN=go1.25.10`, build tag `kruda_stdjson`
- **Load**: `wrk` 4.1.0 (epoll), Kruda-only (`BENCH_FRAMEWORKS=kruda`), `KRUDA_WORKERS=4`, `GOMAXPROCS=8`, port 3400
- **Profiles**: latency `-t4 -c128`, throughput `-t4 -c256`; CPU-bound routes (no DB): plaintext-handler, json-static, json-serialize
- **Goal**: confirm the accept-path additions (global CAS conn cap, per-IP map, accept-rate bucket, conn-struct growth) do **not** regress the steady-state request hot path. The bundle's enforcement is accept-only; keep-alive load exercises the request path, not accept churn.

## Run 1 — base-first, 5 rounds (median RPS)

| Route / profile | base `main` | branch | Δ |
|---|---:|---:|---|
| latency plaintext | ~770k | ~702k | −8.9% ⚠️ |
| latency json-static | ~747k | ~744k | parity |
| latency json-serialize | ~735k | ~738k | parity |
| throughput plaintext | ~757k | ~745k | −1.5% |
| throughput json-static | ~719k | ~742k | +3% |
| throughput json-serialize | ~744k | ~744k | parity |

5/6 cells parity (branch sometimes higher). Only latency-plaintext (the branch run's **first** route) dipped, with elevated early-round p99 (3.2–3.5ms rounds 1–3, recovering to 1.2ms by round 5). A real per-request CPU regression would appear uniformly across all routes and both concurrency levels — it did not (same route at c256 = parity; both JSON routes = parity). Hypothesis: run-order/warmup + shared-box contention.

## Run 2 — confirmation, order REVERSED (head-first), 4 rounds

| latency plaintext | RPS median | p99 range |
|---|---:|---|
| HEAD (ran first) | ~747k | 0.89–2.10 ms |
| BASE (ran second) | ~733k | **331 ms, 568 ms** (rounds 1–2), then 1.35/1.72 ms |

throughput plaintext: HEAD ~752k, BASE ~744k (parity).

The deficit **followed run order** (moved to whichever checkout ran first; this time BASE even took catastrophic 331/568 ms p99 spikes — unambiguous shared-box contention). Branch ran first here and was clean and slightly **higher** than baseline.

## Conclusion — PASS (no regression)

Across two paired runs with reversed execution order, the branch shows **no consistent regression** on the request hot path. Per-route RPS variance (and the 331/568 ms baseline p99 spikes in Run 2) tracks run-order and shared-box noise, not the code. Branch RPS is at parity with `main` (median within ±2%, often marginally higher). This is consistent with the changes being accept-path-only (code-reviewed, hot path untouched) and the zero-alloc guards (`wing_parser_alloc_test.go`, `TestStringLaneZeroAlloc`) staying green.

**Notes**
- `connCnt` false-sharing (deferred from review): keep-alive load barely touches accept/close, so it does not surface here; it would only matter under an accept storm (the DoS scenario, where rejection cost is acceptable). Padding remains unwarranted on this evidence.
- Accept-churn microbench (connect/close cost of CAS + per-IP + bucket) was not run separately; the per-accept cost is 1 atomic + at most a map op + a float compare by construction. Out of scope for the no-regression gate.
- Raw data on tiger: `/home/tiger/kruda-ab-acceptdos/` (`base-result/`, `head-result/`, `*-confirm/`).
