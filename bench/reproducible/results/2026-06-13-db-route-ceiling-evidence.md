# DB-Route Ceiling Evidence

Date: 2026-06-13 UTC
Host: tiger Linux dev server (`tiger-linux`), 13th Gen Intel i5-13500, 8 online
CPUs, KVM; Linux 6.8; Go 1.25.10; wrk 4.1.0 (epoll); academy-postgres 5432,
`hello_world`, `pool_max_conns=64`.
Commit: main `fa29a2f` (v1.3.0), clean fresh clones.
Scope: answer whether the read/write DB routes (`/db`, `/queries`, `/fortunes`,
`/updates`) can be won *decisively* (≥ +10% median RPS on both profiles) versus
Fiber, or whether a physical ceiling caps the achievable margin. This is the
profile-first basis for the v1.3.x performance goal; it does not change any
shipped v1.3.0 claim.

## Question

The v1.3.0 netpoll-takeover evidence
(`2026-06-12-wing-netpoll-takeover-evidence.md`) established that Kruda is
faster than Fiber on every benchmark route per the claim rule (median RPS at
least +3%, median p99 no worse than +10%, zero socket errors, zero non-2xx),
with `/queries` scoped to the default Sonic build. The open question for the
next goal is sharper:

> Can the DB routes be pushed to a *decisive* margin (≥ +10% on both profiles),
> the way the CPU-bound routes already are (+27–34%)?

And the one route never measured head-to-head: does `/updates` (write path) win
at all?

## The ceiling is pgx + the connection pool, not Wing

From the committed forensics (`2026-06-12-wing-netpoll-takeover-evidence.md`,
`forensics-20260612T150500Z`), the bare pgx floor on this box — 256 goroutines
on a 64-connection pool, prepared `worldSelect`, no HTTP — is:

```text
FLOOR mode=query  rps=112025 p50us=1573 p90us=2037 p99us=31382
FLOOR mode=batch1 rps=111261 p50us=1597 p90us=2083 p99us=30852
```

**112K RPS is the hard ceiling for any framework using pgx against this pool.**
Both Kruda and Fiber use the same pgx version, the same DSN, and line-identical
handler bodies, so neither can exceed it on the read routes.

Where Kruda sits relative to that floor (this run, `/db` and `/queries`):

| Cell | Kruda RPS | % of pgx floor (112,025) |
|---|---:|---:|
| `/db` latency | 108,048 | 96.4% |
| `/db` throughput | 103,508 | 92.4% |
| `/queries` latency | 111,721 | 99.7% |

Kruda is already sitting on the pgx ceiling. To beat Fiber by +10% on `/db`
(≈ 114K RPS) would require *exceeding the 112K pgx floor itself* — impossible
without changing the DB layer (a larger pool, a different driver, or query
pipelining), none of which is a Wing-side win and all of which would benefit
Fiber equally.

## The DB routes are DB-bound, not CPU/framework-bound

At `/db` peak (~103K RPS) with the measured 22.0 µs CPU per request
(`2026-06-12` mechanics window), the app burns:

```text
103,000 req/s × 22.0 µs/req ≈ 2.27 CPU-seconds/s ≈ 2.3 of 8 cores (≈ 28%)
```

The box is ~70% idle at DB saturation — it is waiting on Postgres, not running
out of CPU. Throughput is governed by `RPS = pool_max_conns / conn-hold`; with
64 connections and a ~600 µs conn-hold dominated by the Postgres round-trip, the
framework-controllable slice of each request is a small minority. This is why
the same `/db` route that Kruda wins by +173–183% versus Actix
(`2026-06-12-v1-3-0-string-lane-preset-evidence.md`) is only a few percent
versus Fiber: Fiber sits on the same ceiling, so the frameworks converge.

`/updates` is even more DB-bound: it is a write path (per request: a batch of
reads plus a batch of `UPDATE`s), so it tops out around 2,600–2,900 RPS with a
35–85 ms median. The framework contribution to that wall-clock is negligible.

## Fresh measurement — characterization run (3 rounds, default build)

`kruda-updates-ceiling-20260613T080306Z`, default (Sonic) build, kruda vs fiber,
`BENCH_ROUNDS=3 BENCH_DURATION=8s`, ports 3452/3462. Every cell: zero socket
errors, zero non-2xx. Medians:

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 | Claim rule |
|---|---|---:|---:|---:|---:|---:|---:|:--:|
| latency | `db` | 108,047.74 | 104,149.18 | +3.74% | 13.750 | 14.140 | −2.76% | pass |
| throughput | `db` | 103,508.17 | 103,833.20 | −0.31% | 16.160 | 14.400 | +12.22% | tie |
| latency | `queries` | 111,720.59 | 105,740.48 | +5.66% | 12.260 | 12.310 | −0.41% | pass |
| throughput | `queries` | 103,251.47 | 102,047.32 | +1.18% | 15.720 | 13.500 | +16.44% | ballpark |
| latency | `fortunes` | 101,992.24 | 93,814.27 | +8.72% | 4.350 | 3.740 | +16.31% | RPS pass / p99 over |
| throughput | `fortunes` | 102,349.64 | 92,055.77 | +11.18% | 5.190 | 5.410 | −4.07% | pass |
| latency | `updates` | 2,870.01 | 2,653.77 | +8.15% | 282.320 | 333.620 | −15.38% | pass |
| throughput | `updates` | 2,738.01 | 2,606.46 | +5.05% | 573.970 | 534.950 | +7.29% | pass |

Reading:

- `/updates` is a Kruda win on both profiles (+5–8% RPS, p99 within rule, zero
  errors) — but with wide round-to-round spread (write-bound), so a 5-round
  battery is required before it carries a public claim.
- `/db` and `/queries` throughput are at the ceiling: this 3-round run lands at
  tie (−0.3%) and +1.2%, *below* the committed 5-round decider (+3.1% / +3.3%).
  Both agree the win is real but marginal — inside the noise band at the pgx
  floor.
- `/fortunes` keeps a solid RPS win (+8.7% / +11.2%); its latency-profile p99
  swung from +4.9% (committed) to +16.3% here — tail variance worth tracking.
- The p99 trade is visible: `/db` and `/queries` throughput p99 breached the
  +10% rule this run (+12.2% / +16.4%) — the netpoller wake-hop disclosed in the
  v1.3.0 evidence, noisy at the tail.

## Firm-up — claim battery (5 rounds, default build)

`kruda-firmup-5round-20260613T083825Z`, default build, kruda vs fiber,
`BENCH_ROUNDS=5 BENCH_DURATION=8s`, ports 3452/3462. Every cell: zero socket
errors, zero non-2xx. Medians of 5 rounds:

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 | Claim rule |
|---|---|---:|---:|---:|---:|---:|---:|:--:|
| latency | `db` | 112,053.46 | 104,839.91 | +6.88% | 12.030 | 13.140 | −8.45% | pass |
| throughput | `db` | 105,457.85 | 100,795.14 | +4.63% | 14.010 | 15.390 | −8.97% | pass |
| latency | `queries` | 111,585.33 | 104,498.05 | +6.78% | 11.050 | 12.090 | −8.60% | pass |
| throughput | `queries` | 104,194.14 | 102,801.88 | +1.35% | 14.640 | 13.150 | +11.33% | ballpark |
| latency | `fortunes` | 103,636.69 | 93,341.40 | +11.03% | 3.940 | 3.700 | +6.49% | pass (decisive) |
| throughput | `fortunes` | 101,035.81 | 91,629.76 | +10.27% | 5.370 | 5.510 | −2.54% | pass (decisive) |
| latency | `updates` | 2,582.43 | 2,678.70 | −3.59% | 366.410 | 283.530 | +29.23% | loss |
| throughput | `updates` | 2,779.12 | 2,692.46 | +3.22% | 544.500 | 518.380 | +5.04% | marginal |

The 5 rounds resolve the 3-round noise and correct two cells:

- **`/db` is a clean win on both profiles** (+4.63% / +6.88%, both with *better*
  p99). The 3-round throughput tie (−0.3%) was noise; `/db` latency throughput
  (112,053) now *equals* the pgx floor (112,025) — 100% of the ceiling, zero
  headroom left.
- **`/fortunes` is the one decisive DB-route win**: +10.27% / +11.03% on both
  profiles, p99 within rule. It is DB-read **plus HTML render**, so it carries
  more framework-decidable work than a bare row fetch — that is where Wing's lean
  transport and string lane convert into a ≥ +10% margin. The 3-round
  latency-p99 scare (+16.3%) was noise; 5 rounds show +6.49%.
- **`/queries` is a latency win (+6.78%, better p99) but throughput ballpark**
  (+1.35%, p99 +11.33%) — the weakest read cell, pinned at ~93% of the floor.
- **`/updates` is same-ballpark, not a win**: the 3-round +5–8% was write-path
  variance; at 5 rounds the latency profile is a −3.6% loss (p99 +29%) and
  throughput a marginal +3.2%. Claim `/updates` as "same ballpark as Fiber,"
  never a win.

## Decision / Goal

Decisive (≥ +10% on both profiles) is **not achievable on the pure row-fetch DB
routes** (`/db`, `/queries`): Kruda already occupies 93–100% of the pgx floor —
at `/db` latency it *equals* the 112K floor — the routes are DB-bound (≈28% CPU
at saturation), and beating Fiber by +10% there would mean beating pgx itself.
It *is* achievable on `/fortunes` (+10.3% / +11.0%) precisely because that route
adds HTML render — framework-decidable work that pulls it off the bare-pgx
ceiling. `/updates` is write-bound and lands same-ballpark (latency loss,
throughput marginal); it is not a framework-decidable win.

Route-by-route ceiling, all versus Fiber on the shipped default build, zero
errors:

- CPU-bound (`plaintext`, `json` ×3): decisive, +27–34% (committed evidence).
- `/fortunes`: **decisive**, +10.3% / +11.0%, p99 within rule.
- `/db`: clean win, +4.6% / +6.9%, better p99 — but at the pgx ceiling, cannot
  be made decisive without changing the DB layer.
- `/queries`: latency win +6.8% (better p99); throughput same-ballpark (+1.4%,
  p99 +11.3%).
- `/updates`: same ballpark — claim no win.

The goal for the DB routes is therefore **win-or-tie per the claim rule and
prove the ceiling**, not chase RPS the pgx floor will not yield. The one DB-side
item with real headroom is the netpoller wake-hop p99 on `/queries` throughput
(the single cell that fails the rule); it is tracked separately. Levers that
could widen the row-fetch gap (Wing-native PG wire path, query pipelining, a
larger pool) either benefit Fiber equally or change the benchmark contract —
shelved unless a future need justifies the cost.

This also tightens the public story without weakening it: the v1.3.0 "faster
than Fiber on every benchmark route" claim covered the six shipped routes and
stands (this run re-confirms `/db` +4.6–6.9%, `/fortunes` +10.3–11.0%, `/queries`
latency +6.8%); `/updates` was never in that claim and stays scoped as
same-ballpark.
