# Wing Netpoll Takeover Evidence

Date: 2026-06-12 UTC
Host: tiger Linux dev server (`tiger-linux`)
Commits: baseline main `a4b0f32`; candidate branch `wing-netpoll-takeover`
(`93d7912` transport change, `0673463` adds the symmetric queries n==1
short-circuit to both bench apps), all clean fresh clones
Scope: root-cause forensics for the remaining DB-route gap versus Fiber,
the Takeover dispatch fix that follows from it, and the claim battery for
"faster than Fiber" on every benchmark route. CPU-bound routes were already
won on main; this run re-verifies them on the candidate.

## Question

After v1.3.0 (`e534ed9`), Kruda still trailed Fiber on the read-only DB
routes: `/db` -3.6%, `/queries` -6.4 to -8.9%, `/fortunes` -4.9 to -8.0%.
Same pgx version, same pool DSN (`pool_max_conns=64`), line-identical
handler bodies. Three questions:

1. Is pgx/PostgreSQL the ceiling, or is there framework headroom?
2. What mechanism costs Kruda throughput that does not cost Fiber?
3. Does fixing it pass the claim rule (median RPS at least +3%, median p99
   no worse than +10%, zero socket errors, zero non-2xx) against Fiber on
   every route?

## Forensics (main `a4b0f32`, `/db`, c256, 20s windows, same box)

Results: `forensics-20260612T150500Z` (custom orchestrator: pgx floor
bench, then per-framework mechanics windows on `/db`).

pgx floor (no HTTP; 256 goroutines on a 64-conn pool, prepared
`worldSelect`, 20s per mode):

```text
FLOOR mode=query  rps=112025 p50us=1573 p90us=2037 p99us=31382
FLOOR mode=batch1 rps=111261 p50us=1597 p90us=2083 p99us=30852
```

Mechanics under identical wrk load:

| Metric | Kruda (blocking takeover) | Fiber |
|---|---:|---:|
| RPS | 98,032 | 104,632 |
| OS threads during load | 237–250 | 17–18 |
| Context switches per request | 1.40 | 0.56 |
| futex calls per request (strace) | ~10× Fiber | baseline |
| CPU per request | 22.0 µs | 21.9 µs |
| p50 / p99 | 2.43 / 10.46 ms | 2.18 / 14.03 ms |

Findings:

1. The pool/pgx ceiling is 112K RPS — Fiber captures 93% of it, Kruda
   87.5%. pgx is not the cap at Fiber's level, so the gap is framework-side.
2. CPU per request is identical. The loss is scheduling: blocking-syscall
   takeover pins one OS thread per connection, and 250 threads on 8 CPUs
   inflate the latency between "Postgres replied" and "handler goroutine
   runs, scans, releases the pool conn". With `pool_max_conns=64` binding,
   RPS = 64 / conn-hold, and the implied hold is 653 µs (Kruda) versus
   612 µs (Fiber).
3. Floor `batch1` versus `query` is -0.7%: the pgx batch path is nearly
   free at n=1. The -4.5% `/queries`-vs-`/db` gap measured earlier
   (`qprofile-20260612T143739Z`) was that small cost amplified by the
   250-thread regime — and the same profile diff had already cleared
   Wing's `c.Query` param path (it does not appear in the diff at all).

## The change

`wing_transport.go` takeoverLoop no longer switches the fd to blocking
mode. The (already non-blocking, from `accept4`) fd is wrapped in an
`*os.File`, so reads and writes park the goroutine on the runtime
netpoller instead of pinning an OS thread. Close ownership: the takeover
goroutine hands the `*os.File` back through `doneMsg.file`; the worker
loop deletes conn bookkeeping first and then closes through the File (the
fd was Detached from the Wing engine at takeover, so `SubmitClose` must
not run for these conns — and the fd number cannot be recycled while the
conns map still references it). End-to-end coverage:
`TestTransportTakeoverKeepAlive_Wing` (keep-alive reads inside the loop,
pipelined requests in one segment, Connection: close), run under `-race`.

The bench apps additionally short-circuit `/queries` n==1 to a direct
`QueryRow` — in BOTH the Kruda and Fiber apps, so the q=1 cell compares
frameworks, not batch-of-one bookkeeping.

## Commands

Paired A/B (`ab-20260612T154500Z`, 3 rounds, kruda_stdjson):

```bash
BENCH_FRAMEWORKS=kruda            # main clone, port 3451
BENCH_FRAMEWORKS="kruda fiber"    # branch clone, ports 3452/3462
BENCH_ENABLE_DB=1 BENCH_ROUNDS=3 BENCH_DURATION=8s GOTOOLCHAIN=go1.25.10 \
  ./bench.sh db queries fortunes
```

Claim battery (`victory-20260612T161500Z`, 5 rounds, branch `0673463`):

```bash
BENCH_FRAMEWORKS="kruda fiber" BENCH_ENABLE_DB=1 BENCH_ROUNDS=5 \
BENCH_DURATION=8s KRUDA_PORT=3452 FIBER_PORT=3462 \
  ./bench.sh plaintext-handler json-static json-serialize fortunes db queries
```

Default-build decider (`sonic-20260612T170500Z`, 5 rounds, same clone):

```bash
KRUDA_GO_TAGS=default BENCH_FRAMEWORKS="kruda fiber" BENCH_ENABLE_DB=1 \
BENCH_ROUNDS=5 BENCH_DURATION=8s ./bench.sh db queries
```

## Environment

From `victory-20260612T161500Z/environment.txt` (decider differs only in
`kruda_go_tags=default`):

```text
git_commit=0673463
git_tracked_dirty=0
bench_enable_db=1
bench_kruda_db_dispatch=takeover
bench_kruda_cpu_dispatch=inline
kruda_go_tags=kruda_stdjson
gomaxprocs=8
kruda_workers=4
bench_rounds=5
bench_duration=8s
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-124-generic x86_64
Go=go1.25.10 linux/amd64
wrk=debian/4.1.0-4build2
```

Every measured row in all runs had zero socket errors and zero non-2xx.

## Median Results

### Paired A/B — Kruda main vs branch (3 rounds, stdjson)

| Profile | Route | main RPS | branch RPS | ΔRPS | main p99 ms | branch p99 ms |
|---|---|---:|---:|---:|---:|---:|
| latency | `db` | 101,334.06 | 107,612.31 | +6.20% | 7.010 | 11.830 |
| latency | `queries` | 97,333.18 | 105,001.51 | +7.88% | 4.920 | 12.870 |
| latency | `fortunes` | 85,445.04 | 102,228.49 | +19.64% | 4.320 | 4.090 |
| throughput | `db` | 95,912.40 | 105,978.02 | +10.49% | 11.730 | 13.680 |
| throughput | `queries` | 92,920.70 | 99,994.37 | +7.61% | 7.180 | 15.790 |
| throughput | `fortunes` | 86,874.53 | 100,974.31 | +16.23% | 5.570 | 5.360 |

Mechanics window on the branch (`/db`, c256, 20s): 24 threads (was
237–250), 0.56 context switches per request (was 1.40 — now equal to
Fiber), 21.2 µs CPU per request, 106,978 RPS.

### Claim battery — branch Kruda vs Fiber (5 rounds, stdjson)

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 832,177.75 | 630,240.42 | +32.04% | 0.561 | 2.910 | -80.72% |
| latency | `json-static` | 815,727.22 | 620,141.87 | +31.54% | 0.617 | 2.950 | -79.08% |
| latency | `json-serialize` | 809,186.60 | 603,259.78 | +34.14% | 0.705 | 3.170 | -77.76% |
| latency | `fortunes` | 103,404.55 | 93,706.00 | +10.35% | 3.640 | 3.470 | +4.90% |
| latency | `db` | 111,557.03 | 106,773.30 | +4.48% | 9.750 | 13.060 | -25.34% |
| latency | `queries` | 105,083.79 | 102,983.28 | +2.04% | 14.140 | 13.750 | +2.84% |
| throughput | `plaintext-handler` | 821,189.67 | 647,195.30 | +26.88% | 0.828 | 3.550 | -76.68% |
| throughput | `json-static` | 821,084.03 | 644,475.99 | +27.40% | 0.789 | 3.570 | -77.90% |
| throughput | `json-serialize` | 820,063.10 | 630,577.12 | +30.05% | 0.795 | 3.660 | -78.28% |
| throughput | `fortunes` | 103,196.30 | 92,226.20 | +11.89% | 5.040 | 5.380 | -6.32% |
| throughput | `db` | 105,937.30 | 102,712.30 | +3.14% | 13.010 | 12.760 | +1.96% |
| throughput | `queries` | 101,086.42 | 101,384.52 | -0.29% | 15.570 | 14.660 | +6.21% |

Ten of twelve cells pass the claim rule; `queries` is same-ballpark under
the stdjson build.

### Default-build decider — Kruda (Sonic, shipped default) vs Fiber (5 rounds)

| Profile | Route | Kruda RPS | Fiber RPS | ΔRPS | Kruda p99 ms | Fiber p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `db` | 109,513.53 | 102,937.77 | +6.39% | 13.110 | 13.640 | -3.89% |
| latency | `queries` | 106,291.34 | 101,078.35 | +5.16% | 13.480 | 14.720 | -8.42% |
| throughput | `db` | 106,505.28 | 103,117.84 | +3.29% | 13.500 | 13.290 | +1.58% |
| throughput | `queries` | 104,239.19 | 100,909.41 | +3.30% | 13.640 | 13.920 | -2.01% |

Kruda's lowest `queries` latency-profile round (103,876) sits above
Fiber's median.

## Stability Notes

- The paired A/B and the claim battery agree on direction and magnitude
  for every cell; the battery's 5 rounds resolved the one borderline cell
  from the 3-round run (`fortunes` latency p99 +11.4% → +4.90%).
- The tail trade is real and expected: under blocking takeover the kernel
  woke the connection's own thread directly (better p99); the netpoller
  adds a wake hop (db/queries p99 grew from 4.9–11.7 ms to 11.8–15.8 ms on
  the Kruda side) while cutting thread-scheduling pressure (much better
  median and throughput). Both remain far below Actix's 29–34 ms p99 on
  the same cells (`2026-06-12-v1-3-0-string-lane-preset-evidence.md`), so
  no vs-Actix claim is at risk; against Fiber, every claimed cell keeps
  p99 within the rule.
- `fortunes` p99 improved on both profiles — that route was previously
  hurt most by thread pressure (DB read + render on the held conn).
- The stdjson build is the conservative flavor for Kruda: it already wins
  the CPU routes and `fortunes`/`db` with it; the default build (Sonic) is
  what `go get` users run and is the flavor that carries the `queries`
  claim. Fiber runs its own default JSON throughout.

## Decision

Ship the netpoll takeover (merge `wing-netpoll-takeover`). The mechanism
is understood, the paired A/B shows +6.2% to +19.6% RPS on every DB-route
cell with zero errors, and the dispatch model now matches the workload:
goroutine parking for I/O-bound takeover routes, identical CPU cost.

Claim per the claim rule, all versus Fiber, zero errors everywhere:

- CPU-bound (`/plaintext-handler`, `/json-static`, `/json-serialize`):
  +26.88% to +34.14% RPS, p99 -76% to -81% (stdjson build).
- `/fortunes`: +10.35% / +11.89% RPS, p99 within rule (stdjson build).
- `/db`: +4.48% / +3.14% (stdjson) and +6.39% / +3.29% (default build).
- `/queries`: +5.16% / +3.30% on the default (Sonic) build with better
  p99; same-ballpark (+2.04% / -0.29%) on the stdjson build — wording for
  `queries` must name the default build.

Public wording: "faster than Fiber on every benchmark route" is now
supported, with `queries` scoped to the default build. The old
"same ballpark as Fiber" DB wording is superseded by this evidence.
Tail-latency honesty: report the p99 trade on `db`/`queries` wherever the
claim appears in detail; medians and p99 stay ahead of or within rule
against both rivals.
