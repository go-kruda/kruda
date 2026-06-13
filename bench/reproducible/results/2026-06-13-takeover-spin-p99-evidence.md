# Takeover Adaptive-Spin p99 Evidence (P1)

Date: 2026-06-13 UTC
Host: tiger Linux dev server (`tiger-linux`), 13th Gen Intel i5-13500, 8 online
CPUs, KVM; Linux 6.8; Go 1.25.10; wrk 4.1.0 (epoll); academy-postgres 5432,
`hello_world`, `pool_max_conns=64`.
Commits: baseline main `9a0c9f8`; candidate branch `perf-p1-takeover-spin`
(`3468291`). Clean fresh clones, paired in one session (no drift).
Scope: perf-track wave **P1** — reclaim the `/queries` throughput p99 wake-hop,
the one benchmark cell that failed the claim rule after the v1.3.0 netpoll
takeover, without losing the throughput/thread-count win.

## Question

The v1.3.0 netpoll takeover (`590f0f9`) parks the Takeover keep-alive read on the
runtime netpoller (`wing_transport.go` `takeoverLoop`, `f.Read`). Under c256
scheduling contention the netpoller→goroutine wake hop inflates db/queries
throughput p99; the 5-round default-build baseline left `/queries` throughput p99
at +11.33% versus Fiber — over the +10% rule. Can a small spin reclaim the tail
without burning the throughput win?

## The change

`wing_transport.go`: before the parking `f.Read`, the keep-alive loop now makes up
to `takeoverSpinReads` (8) **non-blocking raw reads** of the connection fd. A
buffered next request is consumed immediately, skipping the netpoller wake hop;
when nothing is available the loop falls back to `f.Read` (park), preserving the
low takeover thread count under idle. The fd is owned solely by the takeover
goroutine, so the raw read cannot race the `*os.File` (verified under `-race`,
`TestTransportTakeover*` ×3). The spin path is reached **only by Takeover dispatch
(DB/Render routes)**; Inline CPU routes never execute it.

## Results — paired A/B, 5 rounds, default build, zero errors

### Spin effect (baseline main vs candidate branch, same session)

| Profile | Route | main RPS | branch RPS | ΔRPS | main p99 ms | branch p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `db` | 106,010.80 | 109,203.72 | +3.01% | 13.430 | 10.000 | −25.5% |
| throughput | `db` | 104,101.40 | 106,165.47 | +1.98% | 15.030 | 10.440 | −30.5% |
| latency | `queries` | 105,284.42 | 107,355.32 | +1.97% | 14.970 | 9.670 | −35.4% |
| throughput | `queries` | 104,052.54 | 104,323.34 | +0.26% | 15.320 | 11.360 | −25.8% |

This candidate-first session carries some position variance (the plaintext
control below — which never spins — also shifted), so the **drift-free**
attribution is the same-session gate (next table) plus the firmup-delta noted in
*Why the spin wins*: the `/queries` throughput kruda-vs-Fiber p99 delta moved from
**+11.33%** (firmup, no spin) to **−14.8%** (this session, spin) with Fiber's p99
stable across both runs.

### Gate — candidate branch vs Fiber (same session)

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 | rule |
|---|---|---:|---:|---:|---:|---:|---:|:--:|
| latency | `db` | 109,203.72 | 103,849.03 | +5.15% | 10.000 | 13.080 | −23.5% | pass |
| throughput | `db` | 106,165.47 | 101,946.90 | +4.14% | 10.440 | 14.300 | −27.0% | pass |
| latency | `queries` | 107,355.32 | 103,013.69 | +4.21% | 9.670 | 12.580 | −23.1% | pass |
| throughput | `queries` | 104,323.34 | 101,735.23 | +2.54% | 11.360 | 13.340 | −14.8% | pass |

**The failing cell is fixed and then some:** `/queries` throughput p99 went from
+11.33% over Fiber (baseline) to **−14.8% under Fiber**. Every db/queries cell now
beats Fiber on both RPS and p99.

### Confirm — fortunes (Takeover) + plaintext (Inline control), 3 rounds

Candidate branch vs Fiber, same session:

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `fortunes` | 101,638.27 | 94,927.21 | +7.07% | 3.860 | 3.330 | +15.9% |
| throughput | `fortunes` | 101,450.55 | 94,950.52 | +6.85% | 4.850 | 5.030 | −3.6% |
| latency | `plaintext-handler` | 814,266.18 | 622,457.01 | +30.8% | 0.545 | 2.870 | −81% |
| throughput | `plaintext-handler` | 825,597.23 | 645,059.69 | +28.0% | 0.696 | 3.400 | −80% |

- **`/fortunes` (Takeover, spins): no regression.** Still a clear RPS win (+6.9–7.1%
  vs Fiber). Throughput p99 within rule (−3.6%); the latency-p99 +15.9% is 0.53 ms
  at a ~3.8 ms tail — noise-sensitive at 3 rounds, and **not spin-caused**: baseline
  main `/fortunes` latency p99 (3.700) is already ~+11% over Fiber, and the 5-round
  firmup measured it at +6.49% (passing). To be re-confirmed at 5 rounds in the
  consolidated validation; the spin leaves it essentially unchanged.
- **`/plaintext-handler` (Inline control, never spins): unaffected by the spin** —
  still +28–31% RPS / p99 −80% vs Fiber, confirming the spin path is reached only by
  Takeover dispatch.

## Why the spin wins (the prior was wrong)

The pre-test prior was that wrk keep-alive (not pipelined) would leave the next
request un-buffered, so a spin would mostly miss. The data refutes it. The DB
routes are **DB-bound** (~28% CPU at saturation — the box waits on Postgres), so
the spin consumes otherwise-idle CPU to read the reply the instant it lands rather
than paying the netpoller wake hop while the scheduler is contended. The spin is
near-free precisely because the cores are idle on these routes; it converts idle
wait into a much shorter tail at no throughput cost. CPU-bound routes are
unaffected (they use Inline dispatch, never the takeover spin).

## Decision

**Ship** `takeoverSpinReads = 8`. The one cell that failed the claim rule
(`/queries` throughput p99, +11.33% over Fiber) now **beats** Fiber (−14.8%), and
every db/queries cell passes the rule on both profiles with better p99 and held-or-
improved RPS. `/fortunes` (also Takeover) is not regressed; CPU routes (Inline) are
untouched by construction and unchanged in measurement. The win is mechanistically
sound: DB-bound routes leave the cores ~70% idle, so a bounded non-blocking spin
trades that idle CPU for a shorter tail at no throughput cost.

Re-confirm `/fortunes` latency p99 and all claimed cells in the consolidated 5-round
validation before the v1.3.1 tag (plan Task R.2). No public claim wording changes
yet — that happens at release if the consolidated run holds.
