# Main Read-only DB Follow-up Evidence

Date: 2026-06-07 UTC
Host: tiger Linux dev server (`tiger-linux`)
Commit: `2991641` (`bench: record main syscall profile inventory`)
Scope: read-only DB workloads and Kruda DB dispatch candidate discovery. This is
not a broad CPU-bound fair-handler claim.

## Why This Was Run

The current non-pipelined CPU-bound syscall/profile inventory did not justify a
runtime change toward a broad +20% fair-handler win. This follow-up refreshes
the workload-specific DB path on current `main` and then checks whether a DB
dispatch mode change is supported by same-host evidence.

## Cross-runtime DB Command

```bash
cd /home/tiger/kruda-main-db-followup-20260607T145022Z/bench/reproducible
BENCH_ENABLE_DB=1 \
BENCH_KRUDA_DB_DISPATCH=takeover \
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
BENCH_WARMUP=2s \
GOTOOLCHAIN=go1.25.10 \
KRUDA_PORT=3291 \
ACTIX_PORT=3293 \
RESULT_DIR="results/main-db-followup-20260607T145045Z" \
./bench.sh db fortunes
```

## Cross-runtime Environment

Concrete values from
`results/main-db-followup-20260607T145045Z/environment.txt`:

```text
timestamp_utc=20260607T145045Z
git_commit=2991641
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
routes=db fortunes
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-117-generic x86_64
Go=go1.25.10 linux/amd64
Rust=rustc 1.93.1
Cargo=cargo 1.93.1
wrk=debian/4.1.0-4build2
```

## Cross-runtime Median Results

All measured rows had zero socket errors and zero non-2xx responses. One Kruda
`/db` warmup file reported 128 timeouts before the measured rounds; the summary
rows below exclude warmups.

| Profile | Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda p99 ms | Actix p99 ms | Kruda vs Actix p99 |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `db` | 99,310.04 | 35,823.79 | +177.21% | 7.52 | 30.72 | -75.52% |
| latency | `fortunes` | 84,477.84 | 42,173.64 | +100.31% | 4.40 | 17.48 | -74.83% |
| throughput | `db` | 99,654.94 | 34,908.73 | +185.47% | 6.57 | 34.99 | -81.22% |
| throughput | `fortunes` | 83,641.25 | 41,570.02 | +101.21% | 5.84 | 20.39 | -71.36% |

## DB Dispatch Sweep Command

```bash
cd /home/tiger/kruda-main-db-followup-20260607T145022Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR="results/main-db-dispatch-sweep-20260607T145634Z" \
BENCH_KRUDA_DB_DISPATCH_MODES="takeover inline pool spawn" \
BENCH_ROUNDS=2 \
BENCH_DURATION=6s \
BENCH_WARMUP=2s \
KRUDA_PORT=3301 \
KRUDA_POOL_SIZE=64 \
./sweep-kruda-db-dispatch.sh db fortunes
```

## DB Dispatch Sweep Environment

Concrete values from
`results/main-db-dispatch-sweep-20260607T145634Z/environment.txt`:

```text
timestamp_utc=20260607T145634Z
git_commit=2991641
git_tracked_dirty=0
routes=db fortunes
modes=takeover inline pool spawn
bench_rounds=2
bench_duration=6s
kruda_port=3301
pool_size=64
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-117-generic x86_64
Go=go1.25.10 linux/amd64
wrk=debian/4.1.0-4build2
```

## DB Dispatch Sweep Results

This sweep is Kruda-only candidate discovery, not cross-runtime publication
evidence. All measured rows had zero socket errors and zero non-2xx responses.

| Mode | Route | Profile | Median RPS | Median p99 ms |
|---|---|---|---:|---:|
| `takeover` | `db` | latency | 99,853.31 | 6.230 |
| `takeover` | `db` | throughput | 96,028.77 | 7.880 |
| `inline` | `db` | latency | 43,053.71 | 3.800 |
| `inline` | `db` | throughput | 42,980.51 | 7.340 |
| `pool` | `db` | latency | 3,411.03 | 2.350 |
| `pool` | `db` | throughput | 3,069.34 | 3.970 |
| `spawn` | `db` | latency | 1,254.62 | 2.490 |
| `spawn` | `db` | throughput | 3,880.02 | 3.415 |
| `takeover` | `fortunes` | latency | 85,441.35 | 4.125 |
| `takeover` | `fortunes` | throughput | 85,092.88 | 5.540 |
| `inline` | `fortunes` | latency | 40,518.37 | 4.460 |
| `inline` | `fortunes` | throughput | 40,551.19 | 8.185 |
| `pool` | `fortunes` | latency | 1,251.82 | 3.000 |
| `pool` | `fortunes` | throughput | 4,278.80 | 4.040 |
| `spawn` | `fortunes` | latency | 958.16 | 2.910 |
| `spawn` | `fortunes` | throughput | 2,525.75 | 4.690 |

## Decision

Accepted as scoped evidence only. Current `main` still clears the +20%
throughput gate versus Actix on read-only DB routes by a wide margin, with lower
p99 latency in every cross-runtime median row and no measured socket or non-2xx
errors.

Do not take a runtime change from the dispatch sweep. `takeover` remains the
right Kruda DB dispatch mode for this benchmark app. `inline` is materially
slower on both read-only DB routes, and `pool`/`spawn` collapse throughput even
though their p99 values are sometimes lower because they are completing far
fewer requests. This does not change the prior CPU-bound decision: broad
non-pipelined plaintext/JSON work still needs a candidate that reduces syscall
cost or changes the workload boundary without bypassing normal Kruda behavior.
