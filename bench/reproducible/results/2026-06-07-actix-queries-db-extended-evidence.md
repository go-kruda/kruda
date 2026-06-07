# Actix Queries Route And Extended DB Evidence

Date: 2026-06-07 UTC
Host: tiger Linux dev server (`tiger-linux`)
Branch commit: `4d543cd` (`bench: add Actix queries route`)
Base commit: `94044cf` (`bench: reject PGO candidate`)
Scope: benchmark harness correctness plus scoped DB workload evidence for
`/queries` and `/updates`. This is not a broad CPU-bound fair-handler claim.

## Root Cause

A first current-main cross-runtime run attempted to measure `/queries` and
`/updates` at `94044cf`:

```bash
cd /home/tiger/kruda-db-extended-20260607T164812Z/bench/reproducible
BENCH_ENABLE_DB=1 \
BENCH_KRUDA_DB_DISPATCH=takeover \
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
GOTOOLCHAIN=go1.25.10 \
KRUDA_PORT=3411 \
ACTIX_PORT=3413 \
RESULT_DIR=results/main-db-extended-20260607T164822Z \
./bench.sh queries updates
```

The `/updates` rows were valid, but the Actix `/queries` rows were invalid:

| Profile | Framework | Route | Median RPS | Median p99 ms | Socket errors | Non-2xx |
|---|---|---|---:|---:|---:|---:|
| latency | actix | `queries` | 718,033.88 | 3.060 | 0 | 17,448,808 |
| throughput | actix | `queries` | 731,667.04 | 3.000 | 0 | 17,587,153 |

Root cause: the Actix benchmark app registered `/db`, `/fortunes`, and
`/updates`, but not `/queries`, so the Actix `/queries` run was benchmarking
404 responses. No performance claim should use those invalid rows.

## Candidate

The harness fix adds an Actix `/queries` handler and registers it when
`BENCH_ENABLE_DB=1`. The handler follows the existing Actix `/updates` read
phase style: parse `q`, clamp it to 1..500, get a pool client, read random
`world` rows, and serialize the `World` list as JSON.

## Corrected Cross-runtime Command

Temporary checkout:
`/home/tiger/kruda-actix-queries-20260607T165605Z`

```bash
cd /home/tiger/kruda-actix-queries-20260607T165605Z/bench/reproducible
BENCH_ENABLE_DB=1 \
BENCH_KRUDA_DB_DISPATCH=takeover \
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
GOTOOLCHAIN=go1.25.10 \
KRUDA_PORT=3421 \
ACTIX_PORT=3423 \
RESULT_DIR=results/actix-queries-fixed-db-extended-20260607T165620Z \
./bench.sh queries updates
```

## Corrected Environment

Concrete values from
`results/actix-queries-fixed-db-extended-20260607T165620Z/environment.txt`:

```text
timestamp_utc=20260607T165620Z
git_commit=4d543cd
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
frameworks=kruda actix
routes=queries updates
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-117-generic x86_64
Go=go1.25.10 linux/amd64
Rust=rustc 1.93.1
Cargo=cargo 1.93.1
wrk=debian/4.1.0-4build2
```

All corrected cross-runtime measured rows had zero socket errors and zero
non-2xx responses.

## Corrected Median Results

| Profile | Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda p99 ms | Actix p99 ms | Kruda vs Actix p99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `queries` | 98,571.82 | 36,355.38 | +171.13% | 4.190 | 30.020 | -86.04% |
| throughput | `queries` | 94,289.09 | 34,698.58 | +171.74% | 5.690 | 34.570 | -83.54% |
| latency | `updates` | 2,830.34 | 2,576.64 | +9.85% | 280.850 | 327.670 | -14.29% |
| throughput | `updates` | 2,705.75 | 2,812.15 | -3.78% | 450.410 | 430.170 | +4.71% |

## Updates Dispatch Sweep

Because `/updates` did not clear the throughput gate, a Kruda-only dispatch
sweep checked whether a different DB dispatch mode should be used for the
write-heavy route:

```bash
cd /home/tiger/kruda-actix-queries-20260607T165605Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/updates-dispatch-sweep-20260607T170202Z \
BENCH_KRUDA_DB_DISPATCH_MODES="takeover inline pool spawn" \
BENCH_ROUNDS=2 \
BENCH_DURATION=6s \
KRUDA_PORT=3431 \
KRUDA_POOL_SIZE=64 \
./sweep-kruda-db-dispatch.sh updates
```

All non-inline measured rows had zero socket errors and zero non-2xx responses.
Inline was invalid for this workload because it produced socket errors.

| Mode | Profile | Median RPS | Median p99 ms | Max socket errors | Max non-2xx |
|---|---|---:|---:|---:|---:|
| takeover | latency | 2,211.56 | 608.095 | 0 | 0 |
| takeover | throughput | 2,463.87 | 485.990 | 0 | 0 |
| inline | latency | 130.24 | 1,895.000 | 41 | 0 |
| inline | throughput | 120.62 | 1,975.000 | 305 | 0 |
| pool | latency | 186.13 | 85.410 | 0 | 0 |
| pool | throughput | 290.06 | 312.040 | 0 | 0 |
| spawn | latency | 179.85 | 95.575 | 0 | 0 |
| spawn | throughput | 380.38 | 208.265 | 0 | 0 |

## Decision

Keep the Actix `/queries` benchmark harness fix. It removes an invalid 404
comparison and makes the DB route set match the documented benchmark routes.

Accept the corrected `/queries` result as scoped DB workload evidence: Kruda
cleared the +20% throughput gate versus Actix with much lower p99 latency and
zero measured errors.

Do not claim `/updates` as a Kruda +20% win. In the corrected cross-runtime run,
Kruda slightly trailed Actix in the throughput profile and had higher p99 there.
Do not change the Kruda DB dispatch mode for `/updates` from this pass:
`takeover` remained the only viable throughput mode in the sweep, while inline
errored and pool/spawn completed far fewer requests.
