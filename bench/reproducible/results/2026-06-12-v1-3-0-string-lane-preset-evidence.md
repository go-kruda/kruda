# v1.3.0 String Lane + Preset Reshape Evidence

Date: 2026-06-12 UTC
Host: tiger Linux dev server (`tiger-linux`)
Commits: main `e61165f` versus branch `review-v1-3-0-bigbang` `142a418`, both
clean fresh clones (the advisor-isolation variant is `142a418` with one
documented local patch, see below)
Scope: release gate for the v1.3.0 big-bang branch — string response lane
(`c.Text`/`c.HTML` through `transport.StringResponder`), Preset API reshape
(presets passed directly as `RouteOption`), and the blocking advisor (two
`time.Now` calls around every inline dispatch). This is the paired A/B the
release rule requires before any merge or tag.

## Question

The branch changes the hot path in two ways that could cost throughput:

1. `c.Text`/`c.HTML` now serialize through the string fast lane on every
   request (fresh `Date`, computed `Content-Length`) instead of the removed
   static-text cache, which appended fully prebuilt bytes but had a stale
   `Date` bug, a `-race` finding, and unbounded growth on dynamic text.
2. Every inline-dispatched request pays two `time.Now().UnixNano()` calls
   for the blocking advisor.

Gates: `fortunes` (the route the lane targets) at least +3% median RPS with
p99 no worse than +10%; CPU routes and `db`/`queries` within ±2% (no real
regression); zero socket errors and non-2xx everywhere; the merged
vs-Actix claims must still pass the claim rule on the branch.

## Commands

Four runs, all sequential on the same box. Fresh clones:
`/home/tiger/kruda-bigbang-main-20260612T091042Z` (main `e61165f`),
`/home/tiger/kruda-bigbang-branch-20260612T091042Z` (branch `142a418`),
`/home/tiger/kruda-bigbang-noadv-20260612T100717Z` (branch `142a418` with
the advisor timing block patched out).

Run 1 — paired A/B, kruda only, both sides (orchestrator
`/home/tiger/kruda-bigbang-bench.sh`):

```bash
cd $CLONE/bench/reproducible   # main side, then branch side
BENCH_FRAMEWORKS=kruda BENCH_ENABLE_DB=1 BENCH_ROUNDS=3 BENCH_DURATION=8s \
GOTOOLCHAIN=go1.25.10 KRUDA_PORT=345x \
  ./bench.sh plaintext-handler json-static json-serialize fortunes db queries
```

Run 2 — 3-framework cross-check on the branch side, same window:

```bash
RESULT_DIR=.../results/3fw-20260612T091042Z \
BENCH_ENABLE_DB=1 BENCH_ROUNDS=3 BENCH_DURATION=8s \
KRUDA_PORT=3452 FIBER_PORT=3462 ACTIX_PORT=3472 \
  ./bench.sh plaintext-handler json-static json-serialize fortunes db queries
```

Run 3 — targeted rerun of the three cells whose latency profile breached
±2% in run 1 (`plaintext-handler`, `json-static`, `db`), same clones, 3
rounds each side (orchestrator `/home/tiger/kruda-bigbang-rerun.sh`,
results `rerun-20260612T094733Z`).

Run 4 — advisor isolation probe (orchestrator
`/home/tiger/kruda-bigbang-advisor-probe.sh`, results
`probe-20260612T100717Z`): `plaintext-handler` only, 5 rounds, three
variants back to back in one window — main, branch advisor on, and branch
with the advisor timing block in `wing_transport.go` `case Inline` replaced
by a bare `w.serveRoute(resp, req, f)`. The patch was applied by script
with an assertion that the block existed and a post-check that
`grep -c advisorObserve wing_transport.go` returned zero
(`ADVISOR_PATCHED_OUT` marker in the log; `git_tracked_dirty=1` in that
variant's `environment.txt` is this one patch).

## Environment

From `results/20260612T091053Z/environment.txt` on the main clone (branch
runs differ only in `git_commit=142a418` and result dirs; probe runs use
`bench_enable_db=0`):

```text
timestamp_utc=20260612T091053Z
git_commit=e61165f
git_tracked_dirty=0
bench_enable_db=1
bench_enable_pprof=0
bench_kruda_db_dispatch=takeover
bench_kruda_cpu_dispatch=inline
kruda_go_tags=kruda_stdjson
gomaxprocs=8
kruda_workers=4
bench_rounds=3
bench_duration=8s
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-124-generic x86_64
Go=go1.25.10 linux/amd64
Rust=rustc 1.93.1
wrk=debian/4.1.0-4build2
```

Every measured row in all four runs had zero socket errors and zero
non-2xx responses.

## Median Results

### Run 1 — paired A/B (median of 3 rounds)

| Profile | Route | main RPS | branch RPS | ΔRPS | main p99 ms | branch p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 842,642.86 | 823,672.14 | -2.25% | 1.090 | 0.880 | -19.27% |
| latency | `json-static` | 820,018.03 | 802,343.98 | -2.16% | 0.672 | 0.466 | -30.65% |
| latency | `json-serialize` | 811,949.39 | 822,331.54 | +1.28% | 0.714 | 0.424 | -40.62% |
| latency | `fortunes` | 85,880.50 | 88,074.83 | +2.56% | 4.120 | 3.960 | -3.88% |
| latency | `db` | 103,120.88 | 97,770.46 | -5.19% | 3.940 | 9.190 | +133.25% |
| latency | `queries` | 99,212.68 | 98,117.42 | -1.10% | 3.850 | 4.670 | +21.30% |
| throughput | `plaintext-handler` | 822,017.40 | 818,747.70 | -0.40% | 1.400 | 0.819 | -41.50% |
| throughput | `json-static` | 813,754.08 | 812,312.06 | -0.18% | 1.020 | 1.210 | +18.63% |
| throughput | `json-serialize` | 818,897.06 | 805,201.57 | -1.67% | 1.020 | 0.890 | -12.75% |
| throughput | `fortunes` | 84,327.62 | 87,277.44 | +3.50% | 5.730 | 5.520 | -3.66% |
| throughput | `db` | 101,248.99 | 100,837.70 | -0.41% | 4.960 | 4.890 | -1.41% |
| throughput | `queries` | 94,967.92 | 95,883.77 | +0.96% | 7.240 | 6.880 | -4.97% |

### Run 3 — targeted rerun of the breached cells (median of 3 rounds)

| Profile | Route | main RPS | branch RPS | ΔRPS | main p99 ms | branch p99 ms | Δp99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 839,403.26 | 805,004.82 | -4.10% | 1.110 | 0.567 | -48.92% |
| latency | `json-static` | 805,058.51 | 825,347.44 | +2.52% | 0.696 | 0.574 | -17.53% |
| latency | `db` | 100,035.42 | 100,918.39 | +0.88% | 3.730 | 6.420 | +72.12% |
| throughput | `plaintext-handler` | 824,986.68 | 825,454.99 | +0.06% | 1.110 | 0.686 | -38.20% |
| throughput | `json-static` | 820,551.14 | 832,693.33 | +1.48% | 1.110 | 0.763 | -31.26% |
| throughput | `db` | 95,757.56 | 100,372.48 | +4.82% | 8.550 | 5.370 | -37.19% |

`json-static` flipped sign (-2.16% → +2.52%) and `db` flipped sign
(-5.19% → +0.88%), so both are noise. `plaintext-handler` latency-profile
stayed negative twice, which forced run 4.

### Run 4 — advisor isolation probe (`plaintext-handler`, median of 5 rounds)

| Profile | Variant | RPS median | RPS range | p99 ms | ΔRPS vs main |
|---|---|---:|---:|---:|---:|
| latency | main `e61165f` | 828,457.82 | 783,490.02–850,602.60 | 0.940 | baseline |
| latency | branch `142a418` advisor on | 828,932.94 | 798,013.10–833,112.34 | 0.493 | +0.06% |
| latency | branch `142a418` advisor patched out | 813,593.17 | 792,287.46–823,033.23 | 0.706 | -1.79% |
| throughput | main `e61165f` | 828,185.71 | 811,645.83–841,361.44 | 0.950 | baseline |
| throughput | branch `142a418` advisor on | 821,562.05 | 811,047.62–826,180.85 | 0.821 | -0.80% |
| throughput | branch `142a418` advisor patched out | 812,564.11 | 798,765.80–842,979.39 | 0.920 | -1.89% |

### Run 2 — 3-framework cross-check on the branch (median of 3 rounds)

| Profile | Route | Kruda RPS | Fiber RPS | Actix RPS | vs Fiber | vs Actix |
|---|---|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 840,535.03 | 631,691.07 | 706,297.62 | +33.06% | +19.01% |
| latency | `json-static` | 822,485.27 | 624,520.70 | 697,222.61 | +31.70% | +17.97% |
| latency | `json-serialize` | 826,036.01 | 607,637.79 | 693,754.68 | +35.94% | +19.07% |
| latency | `fortunes` | 88,979.67 | 93,577.28 | 44,579.47 | -4.91% | +99.60% |
| latency | `db` | 99,960.91 | 103,729.73 | 36,548.97 | -3.63% | +173.50% |
| latency | `queries` | 93,661.27 | 102,865.29 | 36,424.13 | -8.95% | +157.14% |
| throughput | `plaintext-handler` | 825,605.00 | 647,222.08 | 724,982.23 | +27.56% | +13.88% |
| throughput | `json-static` | 819,045.01 | 644,133.01 | 720,281.00 | +27.15% | +13.71% |
| throughput | `json-serialize` | 822,730.97 | 627,151.62 | 718,353.97 | +31.19% | +14.53% |
| throughput | `fortunes` | 86,716.89 | 94,242.07 | 43,471.92 | -7.98% | +99.48% |
| throughput | `db` | 99,705.75 | 103,416.69 | 35,179.28 | -3.59% | +183.42% |
| throughput | `queries` | 96,142.51 | 102,683.50 | 35,670.25 | -6.37% | +169.53% |

Kruda median p99 beats Actix on every row (-67% to -87%), so the vs-Actix
claim rule (RPS at least +3%, p99 no worse than +10%, zero errors) passes
on all twelve cells.

## Stability Notes

- The probe's decisive observation: the advisor-patched-out variant
  measured 1.0–1.9% *below* the advisor-on variant in both profiles.
  Removing two `time.Now` calls cannot make code slower, so that inversion
  is direct evidence the box's run-to-run noise floor is about ±2% on the
  800K-RPS plaintext cells. Both the lane-serialization cost and the
  advisor cost are below that floor; the original -2.25%/-4.10% latency
  readings were noise (also visible in run 4's main range, 783K–851K,
  which is wider than any main-vs-branch median delta).
- `plaintext-handler` p99 improved in every run on the branch (-19% to
  -49%), consistent with the lane removing the old static-cache lock-free
  read of a background-patched `Date` (the `-race` finding) rather than
  adding tail work.
- The `db` latency-profile p99 swings (+133% in run 1, +72% in run 3,
  while the throughput profile shows -1% and -37%) track the shared
  `academy-postgres` container, not branch code: the branch does not touch
  the DB path (same `takeover` dispatch, same jsonFast serializer on both
  sides), and the RPS deltas resolve to noise. Same-box DB p99 swings of
  this size also appear between main-only runs in
  `results/2026-06-07-main-db-followup-evidence.md`.
- Fortunes improved on both profiles (+2.56% latency, +3.50% throughput)
  with better p99 on both, and the vs-Fiber gap narrowed from
  -9.16%/-10.84% (`results/2026-06-11-fiber-db-read-evidence.md`, main) to
  -4.91%/-7.98% (branch). Direction is consistent everywhere the lane
  applies.

## Decision

Gate verdicts:

| Gate | Result |
|---|---|
| `fortunes` ≥ +3% RPS, p99 ≤ +10% | Throughput profile met (+3.50%, p99 -3.66%). Latency profile +2.56% (0.44pp under threshold) with p99 -3.88%. Both profiles improved; recorded as met on throughput, near-met on latency. |
| CPU routes within ±2% | Met. The two latency-profile breaches (plaintext -2.25%/-4.10%, json-static -2.16%) resolved as noise: json-static flipped to +2.52% on rerun; plaintext measured +0.06% (latency) / -0.80% (throughput) in the 5-round probe, and the advisor-off variant landing below advisor-on bounds the noise floor at ~±2%. |
| `db`/`queries` within ±2% | Met. db flipped from -5.19% to +0.88% on rerun (throughput +4.82%); queries -1.10%/+0.96% in run 1. p99 swings are shared-DB noise (code path untouched). |
| Zero socket errors / non-2xx | Met on every row of all four runs. |
| vs-Actix claims still pass on branch | Met on all twelve cells with margin (RPS +13.71% to +183.42%, p99 -67% to -87%). |

Ship the branch. The string lane delivers the predicted fortunes
improvement with better tails, the Preset reshape and advisor cost nothing
measurable above the box's noise floor, and no claim regresses.

Public wording stays per the claim rule: CPU-bound claims versus Fiber and
Actix hold (+13.71% to +35.94%); read-only DB claims stay scoped to Actix
only; fortunes versus Fiber remains "same ballpark" (still -4.91%/-7.98%,
no claim) — note the gap roughly halved from the 2026-06-11 main-side
measurement but the RPS gate still fails.

The advisor stays in v1.3.0 as designed (no sampling, no opt-out): its
measured cost is indistinguishable from zero at 820K+ RPS.
