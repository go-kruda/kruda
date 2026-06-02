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
├── profile-kruda.sh # Kruda-only pprof capture helper
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

## Kruda JSON Encoder Mode

The harness builds Kruda with `KRUDA_GO_TAGS=kruda_stdjson` by default so
portable CPU-bound evidence does not depend on CGO availability. For JSON
handler profile investigation, set `KRUDA_GO_TAGS=default` or an empty
`KRUDA_GO_TAGS` value to build Kruda without explicit Go build tags. On
CGO-enabled systems, that selects the default Sonic encoder path.

Compare the JSON serialization route with stdlib JSON:

```bash
KRUDA_GO_TAGS=kruda_stdjson RESULT_DIR=results/json-stdjson-$(date -u +%Y%m%dT%H%M%SZ) ./bench.sh json-serialize
```

Compare the same route with Kruda's default build tags:

```bash
KRUDA_GO_TAGS=default RESULT_DIR=results/json-default-$(date -u +%Y%m%dT%H%M%SZ) ./bench.sh json-serialize
```

For Kruda-only harness checks or dispatch sweeps, keep the default cross-runtime
behavior unchanged and set `BENCH_FRAMEWORKS` explicitly:

```bash
BENCH_FRAMEWORKS=kruda ./bench.sh json-serialize
```

Use the same `KRUDA_GO_TAGS` values with `resource.sh`, `profile-kruda.sh`, or
`syscall-profile.sh` when a JSON profile candidate needs resource or diagnostic
evidence. The generated `environment.txt` records the effective
`kruda_go_tags` value for each run.

Run opt-in DB routes:

```bash
BENCH_ENABLE_DB=1 ./bench.sh db fortunes
```

The DB mode expects a TechEmpower-style PostgreSQL database and uses `DATABASE_URL` when set.
For Kruda-only DB dispatch experiments, set `BENCH_KRUDA_DB_DISPATCH` to
`takeover`, `pool`, `spawn`, or `inline`. This affects only the Kruda
benchmark app and is meant for workload-profile evidence, not CPU-bound public
benchmark claims. Pool dispatch also requires explicit `KRUDA_POOL_SIZE`
evidence because the default pool size follows the Wing worker count:

```bash
BENCH_ENABLE_DB=1 BENCH_KRUDA_DB_DISPATCH=pool KRUDA_POOL_SIZE=64 ./bench.sh db queries fortunes updates
```

For repeatable Kruda-only DB dispatch sweeps, use:

```bash
./sweep-kruda-db-dispatch.sh db queries fortunes updates
```

The sweep runs Kruda only, stores per-run `bench.sh` output under
`results/kruda-db-dispatch-sweep-<timestamp>/runs/`, and writes aggregate
median RPS/p99/error summaries to `dispatch-summary.csv` and `summary.md`.

Kruda's benchmark pprof server is excluded from the default CPU-only benchmark binary so the runtime comparison does not include a diagnostic HTTP server that the Fiber and Actix apps do not run. The harness adds the `bench_pprof` Go build tag only when `BENCH_ENABLE_PPROF=1`:

```bash
BENCH_ENABLE_PPROF=1 ./bench.sh json-serialize
```

For Kruda-only candidate discovery, use `profile-kruda.sh` instead of the cross-runtime benchmark harness. It builds Kruda with `bench_pprof`, runs only CPU-bound Kruda routes, drives `wrk --latency`, captures Go CPU profiles, and writes `go tool pprof -top` reports under `results/profile-<timestamp>/`:

```bash
./profile-kruda.sh
./profile-kruda.sh json-serialize
BENCH_ENABLE_DB=1 BENCH_KRUDA_DB_DISPATCH=pool KRUDA_POOL_SIZE=64 ./profile-kruda.sh db
```

These profiles are diagnostic evidence for choosing the next candidate. They are not cross-runtime benchmark claim evidence because the pprof server and profiler overhead are enabled only for Kruda.

## Profiles

The script runs both profiles for every framework and route:

| Profile | wrk command shape |
|---------|-------------------|
| `latency` | `wrk --latency -t4 -c128 -d15s` |
| `throughput` | `wrk --latency -t4 -c256 -d15s` |

The harness sets `GOMAXPROCS=8` and `KRUDA_WORKERS=4` by default. The Kruda worker count matches the `wrk -t4` CPU-bound profiles and is recorded in every `environment.txt`. Override `KRUDA_WORKERS` explicitly when studying worker scaling.

Memory footprint experiments may set `KRUDA_READ_BUF_SIZE`, for example `KRUDA_READ_BUF_SIZE=4096`, to reduce Wing's per-connection read buffer for short-header CPU-only workloads. This is not the framework default and must be labeled as a workload-specific memory profile; smaller buffers can reject requests whose request line and headers do not fit the configured size.

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

The committed tiger evidence captured at commit `984f0d6` satisfies the "faster than Actix" gate for CPU-bound Wing handler routes under both the latency and throughput profiles. Throughput-profile medians:

| Route | Evidence directory | Kruda vs Actix median RPS | Kruda vs Actix p99 |
|------|--------------------|---------------------------:|-------------------:|
| `/plaintext-handler` | `results/main-984f0d6-20260524T171346Z/` | +12.11% | -77.06% |
| `/json-static` | `results/main-984f0d6-20260524T171346Z/` | +13.50% | -74.24% |
| `/json-serialize` | `results/main-984f0d6-20260524T171346Z/` | +12.89% | -72.87% |

These are normal handler-path routes. Static bypass route options are intentionally separate from fair handler-path benchmark claims.

The corresponding resource evidence is in `results/resource-main-984f0d6-20260524T174429Z/`, with a summary note in `results/2026-05-25-main-984f0d6-tiger-evidence.md`. It uses `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, and a default Kruda benchmark binary without the `bench_pprof` build tag to match the harness `wrk -t4` CPU-bound profiles. The run shows zero socket errors and zero non-2xx responses, with Kruda throughput, p99, and RPS/core ahead of Actix while RSS remains higher than Actix.

The optional short-header read-buffer resource profile is in `results/resource-20260524Tphase7-readbuf4k/`, with a summary note in `results/2026-05-24-read-buffer-size-evidence.md`. It uses `KRUDA_READ_BUF_SIZE=4096`; compared with the phase 6 baseline, Kruda max RSS dropped by 10.77%, 17.61%, and 10.85% on the throughput routes. Actix still has lower RSS, so this is RSS reduction evidence, not a memory-footprint win claim.

The lazy Wing peer-address lookup evidence is in `results/resource-20260524Tphase8-lazy-remote-addr-final-readbuf4k/`, with a summary note in `results/2026-05-24-lazy-remote-addr-evidence.md`. It shows the same CPU-bound handler routes with zero socket errors and zero non-2xx responses, and a short `strace` check confirms that routes which do not call `RemoteAddr()` no longer pay eager `getpeername` syscalls on accept.
