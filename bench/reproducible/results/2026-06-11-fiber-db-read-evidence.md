# Fiber-Inclusive Read-Only DB Evidence

Date: 2026-06-11 UTC
Host: tiger Linux dev server (`tiger-linux`)
Commit: `b84e19b` (`bench: add Actix queries DB route`), clean tree
Scope: first cross-runtime read-only DB run that includes Fiber alongside
Kruda and Actix. It tests whether the accepted scoped DB story (`/db`,
`/queries`, `/fortunes` versus Actix) extends to Fiber. This is not a
CPU-bound fair-handler claim and not an `/updates` claim.

## Question

The merged scoped DB evidence compared Kruda and Actix only:

- `results/2026-06-06-v126-db-evidence.md` (`/db`, `/fortunes`)
- `results/2026-06-07-actix-queries-db-extended-evidence.md` (`/queries`,
  `/updates`)

The Fiber benchmark app already implements `/db`, `/queries`, `/fortunes`,
and `/updates` with pgx, and its `/queries` handler uses the same
`pgx.Batch` single-round-trip pattern as the Kruda benchmark app. No
committed cross-runtime DB rows included Fiber, so the open question was
whether the scoped read-only DB story is broader than Actix.

## Command

Temporary checkout: `/home/tiger/kruda-fiber-db-read-20260611T144648Z`

```bash
cd /home/tiger/kruda-fiber-db-read-20260611T144648Z/bench/reproducible
BENCH_ENABLE_DB=1 \
BENCH_KRUDA_DB_DISPATCH=takeover \
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
GOTOOLCHAIN=go1.25.10 \
KRUDA_PORT=3441 \
FIBER_PORT=3442 \
ACTIX_PORT=3443 \
RESULT_DIR="$PWD/results/fiber-db-read-20260611T144648Z" \
./bench.sh db queries fortunes
```

## Environment

Concrete values from
`results/fiber-db-read-20260611T144648Z/environment.txt` on the tiger
checkout:

```text
timestamp_utc=20260611T144716Z
git_commit=b84e19b
git_tracked_dirty=0
bench_enable_db=1
bench_enable_pprof=0
bench_kruda_db_dispatch=takeover
bench_kruda_cpu_dispatch=inline
database_url_common_override=0
kruda_database_url_override=0
fiber_database_url_override=0
actix_database_url_override=0
kruda_go_tags=kruda_stdjson
gomaxprocs=8
kruda_workers=4
actix_workers=default
kruda_read_buf_size=default
kruda_pool_size=default
bench_rounds=3
bench_duration=8s
frameworks=kruda fiber actix
routes=db queries fortunes
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-124-generic x86_64
Go=go1.25.10 linux/amd64
Rust=rustc 1.93.1
Cargo=cargo 1.93.1
wrk=debian/4.1.0-4build2
```

Every measured row for all three frameworks had zero socket errors and zero
non-2xx responses.

## Median Results

Median of three measured rounds per cell:

| Profile | Route | Kruda RPS | Fiber RPS | Actix RPS | Kruda p99 ms | Fiber p99 ms | Actix p99 ms |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `db` | 104,040.33 | 104,466.44 | 36,330.16 | 3.580 | 13.490 | 29.540 |
| latency | `queries` | 98,557.42 | 104,540.29 | 36,352.56 | 4.590 | 9.440 | 30.410 |
| latency | `fortunes` | 83,467.90 | 93,618.99 | 42,741.79 | 4.470 | 3.680 | 17.640 |
| throughput | `db` | 98,318.45 | 103,708.29 | 36,945.90 | 10.240 | 12.860 | 31.280 |
| throughput | `queries` | 95,120.81 | 102,257.19 | 36,180.17 | 7.520 | 11.810 | 33.230 |
| throughput | `fortunes` | 83,020.53 | 91,392.22 | 43,626.60 | 5.870 | 5.580 | 15.990 |

## Kruda Deltas

Claim-rule gate: median RPS at least +3%, median p99 no worse than +10%,
zero socket errors, zero non-2xx.

| Profile | Route | vs Fiber RPS | vs Fiber p99 | Fiber gate | vs Actix RPS | vs Actix p99 | Actix gate |
|---|---|---:|---:|---|---:|---:|---|
| latency | `db` | -0.41% | -73.46% | not met | +186.37% | -87.88% | met |
| latency | `queries` | -5.72% | -51.38% | not met | +171.12% | -84.91% | met |
| latency | `fortunes` | -10.84% | +21.47% | not met | +95.28% | -74.66% | met |
| throughput | `db` | -5.20% | -20.37% | not met | +166.11% | -67.26% | met |
| throughput | `queries` | -6.98% | -36.33% | not met | +162.91% | -77.37% | met |
| throughput | `fortunes` | -9.16% | +5.20% | not met | +90.30% | -63.29% | met |

## Stability Notes

- Per-cell round spread was at most 4.0% RPS; most cells were under 2%.
- Latency-profile `/db` is a dead heat: Kruda rounds spanned
  103,930-104,411 RPS and Fiber rounds spanned 102,182-106,230 RPS, so the
  round ranges overlap and the -0.41% median delta is noise-level.
- The `/queries` and `/fortunes` Fiber gaps are consistent: in the
  throughput profile the highest Kruda round stayed below the lowest Fiber
  round on both routes.
- The Actix deltas reproduce the merged evidence closely from a fresh
  checkout: throughput `/db` +166.11% now versus +166.13% in
  `results/2026-06-06-v126-db-evidence.md`, and latency `/queries` +171.12%
  now versus +171.13% in
  `results/2026-06-07-actix-queries-db-extended-evidence.md`.

## Decision

Keep the scoped read-only DB win claims versus Actix. This run reproduces
them independently at `b84e19b` on all six route/profile rows with zero
errors.

Do not extend the scoped DB story to Fiber. The RPS gate fails on every
row: `/db` is same ballpark (noise-level overlap in the latency profile),
`/queries` is 5.72-6.98% behind, and `/fortunes` is 9.16-10.84% behind.
Public wording must keep "faster than Actix" DB claims scoped to Actix and
use "same ballpark as Fiber" for read-only DB RPS.

Kruda's median p99 versus Fiber is materially lower on `/db` and
`/queries` (-20.37% to -73.46%) but not on `/fortunes`. Per the claim rule,
do not turn this into an RPS-independent "faster than Fiber" claim; record
it as a tail-latency observation only.

If the `/fortunes` gap matters later, treat it as a separate Kruda-only
bottleneck investigation (the route mixes a DB read with HTML rendering),
not as a promised runtime patch.
