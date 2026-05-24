# Benchmark Reproduction

This directory contains the source code and harness for Kruda, Fiber, and Actix CPU-bound benchmark comparisons.

The default benchmark does not require PostgreSQL. It measures framework overhead for high-performance, low-latency, high-throughput CPU-bound routes and records throughput, latency percentiles, socket errors, and non-2xx responses.

## Default Routes

| Route | Workload |
|------|----------|
| `/plaintext-handler` | Normal handler path returning plaintext |
| `/json-static` | Normal handler path returning constant JSON bytes, no serialization |
| `/json-serialize` | Normal handler path performing real JSON serialization |

DB, queries, fortunes, and updates are out of scope for the default comparison. Enable them explicitly with `BENCH_ENABLE_DB=1` when you want to study database-heavy workloads.

## Directory Structure

```
bench/reproducible/
├── kruda/          # Kruda using Wing transport
├── fiber/          # Fiber v2 using fasthttp
├── actix/          # Actix Web 4
├── bench.sh        # Automated benchmark script
├── resource.sh     # CPU/RAM resource benchmark script
└── README.md
```

## Prerequisites

```bash
go version
rustc --version
cargo --version
wrk --version
curl --version
```

The harness builds all three benchmark apps from their own directories.

The resource harness also requires `pidstat` from sysstat:

```bash
pidstat -V
```

## Run

```bash
cd bench/reproducible
./bench.sh
```

Run one route:

```bash
./bench.sh json-static
```

Run opt-in DB routes:

```bash
BENCH_ENABLE_DB=1 ./bench.sh db fortunes
```

The DB mode expects a TechEmpower-style PostgreSQL database and uses `DATABASE_URL` when set.

Kruda's benchmark pprof server is excluded from the default CPU-only benchmark binary so the runtime comparison does not include a diagnostic HTTP server that the Fiber and Actix apps do not run. The harness adds the `bench_pprof` Go build tag only when `BENCH_ENABLE_PPROF=1`:

```bash
BENCH_ENABLE_PPROF=1 ./bench.sh json-serialize
```

## Profiles

The script runs both profiles for every framework and route:

| Profile | wrk command shape |
|---------|-------------------|
| `latency` | `wrk --latency -t4 -c128 -d15s` |
| `throughput` | `wrk --latency -t4 -c256 -d15s` |

The harness sets `GOMAXPROCS=8` and `KRUDA_WORKERS=4` by default. The Kruda worker count matches the `wrk -t4` CPU-bound profiles and is recorded in every `environment.txt`. Override `KRUDA_WORKERS` explicitly when studying worker scaling.

Each framework/route/profile combination gets one warmup run and five measured rounds.

## Resource Run

Use `resource.sh` when the evidence needs CPU and RAM in addition to throughput and tail latency:

```bash
cd bench/reproducible
./resource.sh
```

Run one route:

```bash
./resource.sh json-serialize
```

The resource harness uses the same route and profile defaults as `bench.sh`. Each framework/route/profile combination gets one warmup run and one measured run while `pidstat` records process CPU, RSS, and context switch rates. CPU percentage is process CPU across cores, so `400%` means roughly four busy cores. The default `RESOURCE_MIN_CPU_SAMPLE=50` excludes low-CPU startup/shutdown tail samples from average CPU; set it to `0` to include every sample.

Use `BENCH_ENABLE_PPROF=1` with `resource.sh` only when the experiment intentionally measures profiling overhead.

## Output

Results are written to `bench/reproducible/results/<timestamp>/`:

| File | Purpose |
|------|---------|
| `environment.txt` | CPU, OS, toolchain, route, profile, and worker metadata |
| `summary.csv` | Machine-readable per-round RPS, p50, p90, p99, max latency, socket errors, and non-2xx responses |
| `summary.md` | Markdown table for PR evidence |
| `raw/*.txt` | Raw `wrk --latency` output and server logs |

Resource results are written to `bench/reproducible/results/resource-<timestamp>/`:

| File | Purpose |
|------|---------|
| `environment.txt` | CPU, memory, OS, toolchain, route, profile, worker, and `pidstat` metadata |
| `resource-summary.csv` | Machine-readable wrk and pidstat output, including CPU, RSS, context switches, and RPS/core |
| `resource-aggregated.csv` | Smaller table for cross-framework comparison |
| `summary.md` | Markdown table for PR evidence |
| `raw/*.txt` | Raw `wrk --latency`, `pidstat`, and server logs |

## Claim Rule

Use "faster than Actix" only when Kruda median RPS is at least 3% higher than Actix and p99 is no worse than 10% above Actix with zero socket errors and zero non-2xx responses.

When those conditions are not met, use "same ballpark as Actix." Do not make RPS-only claims.

## Current Evidence

The committed evidence below satisfies the "faster than Actix" gate for CPU-bound Wing handler routes under the throughput profile:

| Route | Evidence directory | Kruda vs Actix median RPS | Kruda vs Actix p99 |
|------|--------------------|---------------------------:|-------------------:|
| `/plaintext-handler` | `results/20260523T123854Z-plaintext-final-k4/` | +10.68% | -76.99% |
| `/json-static` | `results/20260524Tphase3-json-final/` | +13.97% | -69.33% |
| `/json-serialize` | `results/20260524Tphase3-json-final/` | +12.14% | -68.01% |

These are normal handler-path routes. Static bypass route options are intentionally separate from fair handler-path benchmark claims.

Current resource evidence for the same CPU-bound handler routes is in `results/resource-20260524Tphase6-no-pprof-build-rerun/`. It uses `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, and a default Kruda benchmark binary without the `bench_pprof` build tag to match the harness `wrk -t4` CPU-bound profiles. The run shows zero socket errors and zero non-2xx responses, with Kruda throughput and p99 ahead of Actix while RSS remains higher than Actix.
