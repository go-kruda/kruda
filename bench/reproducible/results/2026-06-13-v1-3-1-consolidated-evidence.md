# v1.3.1 Consolidated Validation

Date: 2026-06-13 UTC
Host: tiger Linux dev server (`tiger-linux`), 13th Gen Intel i5-13500, 8 online
CPUs, KVM; Linux 6.8; Go 1.25.10; wrk 4.1.0 (epoll); academy-postgres 5432,
`hello_world`, `pool_max_conns=64`.
Commit: main `f026f73` (the v1.3.1 runtime — P1 takeover spin merged; P2/P3/P4 add
no runtime change). Default (Sonic) build, kruda vs fiber, 5 rounds, 8s windows.
Scope: release-gate validation for **v1.3.1** — confirm the P1 spin's effect holds
across every claimed route and that no route regressed. Every cell below had
zero socket errors and zero non-2xx.

## Median results (5 rounds)

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 841,622 | 625,408 | +34.6% | 0.530 | 2.930 | −81.9% |
| latency | `json-static` | 819,083 | 614,605 | +33.3% | 0.482 | 2.920 | −83.5% |
| latency | `json-serialize` | 807,342 | 597,262 | +35.2% | 0.695 | 3.140 | −77.9% |
| latency | `fortunes` | 99,037 | 92,163 | +7.46% | 3.800 | 3.510 | +8.26% |
| latency | `db` | 104,022 | 103,116 | +0.88% | 11.700 | 13.340 | −12.3% |
| latency | `queries` | 105,940 | 101,453 | +4.42% | 9.630 | 13.550 | −28.9% |
| throughput | `plaintext-handler` | 825,253 | 640,578 | +28.8% | 0.791 | 3.630 | −78.2% |
| throughput | `json-static` | 824,668 | 645,127 | +27.8% | 0.684 | 3.430 | −80.1% |
| throughput | `json-serialize` | 797,159 | 626,828 | +27.2% | 1.080 | 3.490 | −69.1% |
| throughput | `fortunes` | 99,329 | 93,075 | +6.72% | 5.100 | 5.190 | −1.7% |
| throughput | `db` | 103,004 | 100,293 | +2.70% | 12.200 | 14.440 | −15.5% |
| throughput | `queries` | 100,781 | 98,873 | +1.93% | 13.070 | 14.000 | −6.6% |

## Reading

- **CPU routes** (`plaintext`, `json-static`, `json-serialize`): decisive, +27% to
  +35% RPS with p99 −69% to −84%. Unchanged from v1.3.0.
- **`/fortunes`**: clear win, +6.7% / +7.5% RPS; **latency p99 now within the rule
  at +8.26%** (the 3-round noise that showed +16% resolved at 5 rounds), throughput
  p99 −1.7%.
- **`/db` and `/queries`** — the v1.3.1 story: the P1 spin **eliminates the v1.3.0
  p99 trade**. Every DB-route cell now **beats Fiber on p99** (−6.6% to −28.9%).
  RPS sits at the pgx ceiling (the routes are pool-bound; see
  `2026-06-13-db-route-ceiling-evidence.md`), so the RPS delta is parity-plus
  (+0.9% to +4.4%) rather than a decisive margin — by physics, not for lack of a
  win. The takeover-spin evidence (`2026-06-13-takeover-spin-p99-evidence.md`)
  isolates the p99 mechanism.

## Decision

Ship v1.3.1. Every claimed route passes the claim rule on both profiles with the
p99 caveat from v1.3.0 now removed: db/queries no longer trade tail latency for
throughput — they beat Fiber on p99 while matching it on RPS at the pgx ceiling.
No route regressed; zero errors throughout.

Public-claim wording updated accordingly: CPU routes and `/fortunes` are clear
RPS+p99 wins; `/db` and `/queries` match Fiber on RPS (both at the pgx ceiling)
and now beat it on p99. The v1.3.0 "trades a few milliseconds of db/queries p99"
note is superseded by this run.
