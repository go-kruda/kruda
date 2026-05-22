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

## Profiles

The script runs both profiles for every framework and route:

| Profile | wrk command shape |
|---------|-------------------|
| `latency` | `wrk --latency -t4 -c128 -d15s` |
| `throughput` | `wrk --latency -t4 -c256 -d15s` |

Each framework/route/profile combination gets one warmup run and five measured rounds.

## Output

Results are written to `bench/reproducible/results/<timestamp>/`:

| File | Purpose |
|------|---------|
| `environment.txt` | CPU, OS, toolchain, route, profile, and worker metadata |
| `summary.csv` | Machine-readable per-round RPS, p50, p90, p99, max latency, socket errors, and non-2xx responses |
| `summary.md` | Markdown table for PR evidence |
| `raw/*.txt` | Raw `wrk --latency` output and server logs |

## Claim Rule

Use "faster than Actix" only when Kruda median RPS is at least 3% higher than Actix and p99 is no worse than 10% above Actix with zero socket errors and zero non-2xx responses.

When those conditions are not met, use "same ballpark as Actix." Do not make RPS-only claims.
