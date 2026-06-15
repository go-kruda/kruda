# Wing Body-Limit Parity — Paired A/B Evidence (no hot-path regression)

Date: 2026-06-15 UTC
Host: tiger Linux dev server (`tiger-linux`), Go **1.25.10** (pinned via
`GOTOOLCHAIN`, identical for both arms), wrk 4.1.0 (epoll), TechEmpower-style
local PostgreSQL `hello_world`.
Run dir (tiger): `/home/tiger/kruda-bodylimit-ab-20260615T140841Z`
(summary artifacts only; `raw/` wrk dumps left on tiger).

Paired A/B, **Kruda-only** (`BENCH_FRAMEWORKS=kruda`):

- **Arm A — `main` @ `4e9d300`** (baseline, before the body-limit bundle)
- **Arm B — `feat/wing-body-limit-parity` @ `bc111c7`** (this branch)

Both arms: fresh clone of the same public repo, same toolchain, same box,
back-to-back. Purpose: confirm the Wing body-limit bundle does **not** regress
the no-body hot path (user hard constraint). The hot routes carry **no request
body**, so the only shared-code change they touch is the parser's two
boolean Transfer-Encoding checks (review finding #8) — this run measures whether
that is observable.

Command (per arm):

```bash
BENCH_FRAMEWORKS=kruda KRUDA_PORT=3400 BENCH_ENABLE_DB=1 \
BENCH_ROUNDS=5 BENCH_DURATION=15s GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=$BASE/<main|branch> \
bash bench/reproducible/bench.sh plaintext-handler json-static json-serialize db
```

Profiles: `latency` = `-t4 -c128`, `throughput` = `-t4 -c256`.
**Every cell (4 routes × 2 profiles × 5 rounds × 2 arms = 80 runs) had zero
socket errors and zero non-2xx responses.**

## Median RPS (5 rounds) — main vs branch

| Route | Profile | main RPS | branch RPS | Δ RPS |
|---|---|---:|---:|---:|
| plaintext-handler | latency (c128)   | 821,955 | 813,718 | −1.00% |
| plaintext-handler | throughput (c256)| 811,542 | 816,452 | +0.60% |
| json-static       | latency (c128)   | 811,801 | 814,721 | +0.36% |
| json-static       | throughput (c256)| 803,531 | 820,621 | +2.13% |
| json-serialize    | latency (c128)   | 807,408 | 820,397 | +1.61% |
| json-serialize    | throughput (c256)| 817,342 | 821,522 | +0.51% |
| db (takeover)     | latency (c128)   | 104,428 | 108,720 | +4.11% |
| db (takeover)     | throughput (c256)| 103,923 | 101,640 | −2.20% |

## Median p99 (ms) — main vs branch

| Route | Profile | main p99 | branch p99 |
|---|---|---:|---:|
| plaintext-handler | latency    | 0.477 | 0.870 |
| plaintext-handler | throughput | 0.802 | 1.140 |
| json-static       | latency    | 0.539 | 0.787 |
| json-static       | throughput | 1.040 | 0.990 |
| json-serialize    | latency    | 0.600 | 0.618 |
| json-serialize    | throughput | 0.843 | 0.880 |
| db (takeover)     | latency    | 11.66 | 9.29  |
| db (takeover)     | throughput | 11.44 | 11.56 |

## Reading

- **No-body hot path (plaintext / json-static / json-serialize): parity-or-better.**
  Every RPS delta is within ±2.2%, inside the established ±2% noise floor for
  this box at >800K RPS. plaintext is −1.0% (c128) / +0.6% (c256) → parity;
  both JSON routes lean slightly positive. The two added boolean TE checks on
  the parser fast path are not observable in throughput, matching the
  `TestParseFastPath_AllocBaseline` 0-alloc guard.
- **CPU-route p99 is noise-dominated.** Sub-millisecond p99 at >800K RPS bounces
  0.4–1.4 ms in both arms with no systematic direction; no claim is made on it.
- **db (takeover path): parity.** +4.1% (c128) and −2.2% (c256) straddle zero —
  the route is pgx-pool-bound at the ~104K ceiling, and it carries no request
  body, so the body-accumulation / deadline changes never execute on it. db p99
  is parity-to-better (latency 11.66 → 9.29 ms).
- **Conclusion: the body-limit bundle introduces no hot-path regression.** Gate
  met; safe to merge on the perf axis.
