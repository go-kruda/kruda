# v1.2.6 Read Buffer 2048 Evidence

Date: 2026-06-07
Host: tiger
Commit under test: `6e69e25dd9c7e8726e62ba346278ce2afce13ea7`
Scope: short-header CPU-bound resource profile only; not a broad throughput
claim and not a framework default change.

## Command

Same-runner baseline:

```bash
cd /home/tiger/kruda-v126-readbuf2048-20260607/bench/reproducible
PATH="$HOME/.cargo/bin:$PATH" \
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR="$PWD/results/resource-v126-readbuf4096-20260607T0119Z" \
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_READ_BUF_SIZE=4096 \
KRUDA_PORT=3261 \
FIBER_PORT=3262 \
ACTIX_PORT=3263 \
./resource.sh plaintext-handler json-static json-serialize
```

Candidate:

```bash
cd /home/tiger/kruda-v126-readbuf2048-20260607/bench/reproducible
PATH="$HOME/.cargo/bin:$PATH" \
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR="$PWD/results/resource-v126-readbuf2048-20260607T0142Z" \
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_READ_BUF_SIZE=2048 \
KRUDA_PORT=3271 \
FIBER_PORT=3272 \
ACTIX_PORT=3273 \
./resource.sh plaintext-handler json-static json-serialize
```

## Environment

Both runs used:

- `bench_enable_db=0`
- `bench_enable_pprof=0`
- `kruda_go_tags=kruda_stdjson`
- `gomaxprocs=8`
- `kruda_workers=4`
- `bench_duration=15s`
- `resource_interval=1`
- `resource_min_cpu_sample=50`
- `frameworks=kruda fiber actix`
- `routes=plaintext-handler json-static json-serialize`
- `profiles=latency:-t4 -c128 -d15s throughput:-t4 -c256 -d15s`
- CPU: `13th Gen Intel(R) Core(TM) i5-13500`, 8 logical CPUs
- Toolchain: `go version go1.25.10 linux/amd64`

Result directories:

- 4096 baseline:
  `/home/tiger/kruda-v126-readbuf2048-20260607/bench/reproducible/results/resource-v126-readbuf4096-20260607T0119Z`
- 2048 candidate:
  `/home/tiger/kruda-v126-readbuf2048-20260607/bench/reproducible/results/resource-v126-readbuf2048-20260607T0142Z`

## 2048 Cross-runtime Rows

| Profile | Framework | Route | RPS | p99 ms | Max RSS MB | Socket errors | Non-2xx |
|---|---|---|---:|---:|---:|---:|---:|
| latency | kruda | plaintext-handler | 816367.44 | 1.390 | 14.50 | 0 | 0 |
| latency | kruda | json-static | 811965.77 | 1.030 | 15.62 | 0 | 0 |
| latency | kruda | json-serialize | 784925.40 | 0.621 | 16.12 | 0 | 0 |
| throughput | kruda | plaintext-handler | 802698.88 | 1.210 | 17.88 | 0 | 0 |
| throughput | kruda | json-static | 813541.34 | 1.110 | 18.38 | 0 | 0 |
| throughput | kruda | json-serialize | 788570.12 | 1.090 | 18.75 | 0 | 0 |
| latency | fiber | plaintext-handler | 627107.59 | 2.850 | 11.75 | 0 | 0 |
| latency | fiber | json-static | 617026.59 | 2.890 | 11.88 | 0 | 0 |
| latency | fiber | json-serialize | 598172.70 | 3.140 | 17.44 | 0 | 0 |
| throughput | fiber | plaintext-handler | 639321.50 | 3.580 | 17.28 | 0 | 0 |
| throughput | fiber | json-static | 629266.20 | 3.510 | 17.28 | 0 | 0 |
| throughput | fiber | json-serialize | 616093.25 | 3.930 | 21.75 | 0 | 0 |
| latency | actix | plaintext-handler | 692088.47 | 2.720 | 7.82 | 0 | 0 |
| latency | actix | json-static | 689879.46 | 2.740 | 7.95 | 0 | 0 |
| latency | actix | json-serialize | 691364.32 | 2.860 | 8.07 | 0 | 0 |
| throughput | actix | plaintext-handler | 722103.99 | 3.230 | 10.45 | 0 | 0 |
| throughput | actix | json-static | 721257.42 | 3.620 | 10.57 | 0 | 0 |
| throughput | actix | json-serialize | 707787.46 | 3.110 | 10.57 | 0 | 0 |

## 2048 vs 4096 Kruda Same-runner Comparison

| Profile | Route | RPS delta | p99 delta | Max RSS delta |
|---|---|---:|---:|---:|
| latency | plaintext-handler | -0.90% | +109.02% | -5.72% |
| latency | json-static | -0.68% | +25.76% | -4.64% |
| latency | json-serialize | -4.37% | +8.76% | -9.84% |
| throughput | plaintext-handler | -2.71% | +51.06% | -4.64% |
| throughput | json-static | -1.99% | +21.98% | -5.74% |
| throughput | json-serialize | -4.27% | +34.40% | -8.00% |

## Decision

Accepted as optional profile evidence only. `KRUDA_READ_BUF_SIZE=2048`
preserved zero socket errors and zero non-2xx responses while reducing Kruda max
RSS by 4.64-9.84% versus the same-runner `4096` baseline. It fails the strict
default-change gate because every measured route/profile row regressed p99, and
RPS fell by 0.68-4.37%. Keep the framework default unchanged and keep `4096` as
the balanced throughput/p99 evidence profile.
