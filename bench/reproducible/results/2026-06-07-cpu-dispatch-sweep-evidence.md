# Kruda CPU Dispatch Sweep Evidence

Date: 2026-06-07
Host: tiger Linux dev server (`tiger-linux`)
Commit: `6675e61` (`bench: add CPU dispatch sweep control`)
Scope: Kruda-only candidate discovery for CPU-bound dispatch modes; not cross-runtime benchmark claim evidence.

## Command

```bash
cd /home/tiger/kruda-perf-cpu-dispatch-20260607T1315Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/cpu-dispatch-json-serialize-20260607T1315Z \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
KRUDA_PORT=3251 \
./sweep-kruda-cpu-dispatch.sh json-serialize
```

## Environment

- CPU: 13th Gen Intel Core i5-13500, 8 online CPUs, KVM
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.25.10 linux/amd64`
- wrk: `debian/4.1.0-4build2 [epoll]`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- `KRUDA_GO_TAGS=kruda_stdjson`
- `BENCH_ROUNDS=3`
- `BENCH_DURATION=8s`
- `KRUDA_POOL_SIZE=64` for the `pool` case only
- Result directory: `/home/tiger/kruda-perf-cpu-dispatch-20260607T1315Z/bench/reproducible/results/cpu-dispatch-json-serialize-20260607T1315Z`

## Median Results

| Mode | Route | Profile | Median RPS | Median p99 | Socket errors | Non-2xx |
|---|---|---|---:|---:|---:|---:|
| `inline` | `json-serialize` | latency | 845,469.85 | 0.421 ms | 0 | 0 |
| `inline` | `json-serialize` | throughput | 814,214.31 | 0.970 ms | 0 | 0 |
| `takeover` | `json-serialize` | latency | 309,446.60 | 173.950 ms | 0 | 0 |
| `takeover` | `json-serialize` | throughput | 308,470.91 | 293.230 ms | 0 | 0 |
| `pool` | `json-serialize` | latency | 13,802.21 | 0.279 ms | 0 | 0 |
| `pool` | `json-serialize` | throughput | 52,187.77 | 0.477 ms | 0 | 0 |
| `spawn` | `json-serialize` | latency | 8,861.70 | 0.416 ms | 0 | 0 |
| `spawn` | `json-serialize` | throughput | 21,168.10 | 0.860 ms | 0 | 0 |

## Decision

Rejected as a CPU-bound performance candidate. Inline remains the correct
default for the benchmark app's CPU-bound JSON route. `takeover`, `pool`, and
`spawn` all avoided socket errors and non-2xx responses, but each lost
material throughput versus `inline` on the weakest CPU-bound route. Do not
pursue CPU dispatch mode changes as the next broad fair-handler +20% path
without new evidence that first beats this same-runner inline baseline.
