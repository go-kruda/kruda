# Balanced CPU-Bound Performance Design

Date: 2026-05-23
Status: Implemented in `perf/balanced-cpu-performance`

## Goal

Strengthen Kruda's public performance evidence for high-performance, low-latency, high-throughput CPU-bound routes. The benchmark story must report throughput, latency percentiles, allocation pressure, socket errors, and non-2xx responses together instead of relying on a single RPS number.

This change does not alter default framework behavior, does not add a release, and does not introduce GitHub Actions cross-runtime benchmarks.

## Workload Scope

The default reproducible benchmark covers CPU-bound HTTP/1.1 routes only:

- `GET /plaintext-handler`: normal handler path returning a plaintext body.
- `GET /json-static`: normal handler path returning constant JSON bytes without serialization.
- `GET /json-serialize`: normal handler path performing real JSON serialization.

Database routes, fortunes, and update workloads remain available only when explicitly enabled with `BENCH_ENABLE_DB=1`. They are not part of the default claim gate because driver, pool, and database configuration can dominate framework overhead.

## Fairness Rules

The default comparison runs Kruda, Fiber, and Actix from `bench/reproducible` using script-relative build paths. Each framework is started from its own directory, measured on the same host, and tested with the same `wrk --latency` profiles:

- Latency profile: `-t4 -c128 -d15s`
- Throughput profile: `-t4 -c256 -d15s`

Each framework/route/profile combination receives one warmup run and five measured rounds. Raw `wrk` output is kept with the summary so reviewers can inspect the source evidence.

## Evidence Format

Every run records:

- CPU and OS/kernel information
- Go, Rust, Cargo, and wrk versions
- `GOMAXPROCS`, `KRUDA_WORKERS`, `BENCH_ENABLE_DB`, routes, and profiles
- Per-round RPS, p50, p90, p99, max latency, socket errors, and non-2xx responses
- Raw output path for each measured round

PR evidence should attach or commit the generated result directory when external attachment is not available.

## Public Claim Gate

Kruda may say "faster than Actix" for these CPU-bound benchmark routes only when both conditions are true:

- Kruda median RPS is at least 3% higher than Actix on the same route/profile.
- Kruda p99 is no worse than 10% above Actix with zero socket errors and zero non-2xx responses.

If the RPS condition is not met, or p99/error behavior is worse, the public wording must say "same ballpark as Actix" instead. RPS-only claims are not acceptable.

## Static Wing Bypass

`WingStaticText` and `WingStaticJSON` are explicit, opt-in route options for public static hot paths such as health checks, plaintext responses, and static JSON. They install a prebuilt Wing response and bypass the handler, middleware, lifecycle hooks, cookies, CORS, and secure-header injection on Wing transports.

These options must not be used for fair normal-handler benchmark claims. Benchmark routes that compare frameworks should use handler-level static JSON when measuring "no serialization" behavior.

## Non-Goals

- No default behavior change.
- No release, version bump, or tag.
- No GitHub Actions cross-runtime benchmark.
- No DB, fortunes, TLS, HTTP/2, WebSocket, or production-network benchmark claims in this PR.
