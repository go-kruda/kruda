# Main 984f0d6 Tiger CPU-Bound Evidence

This note summarizes the tiger benchmark evidence collected from `main` commit `984f0d6` after the Wing common-header fast path was merged.

## Scope

- Host: tiger `dev-server`
- CPU: 8 vCPU, 13th Gen Intel(R) Core(TM) i5-13500 under KVM
- OS: Linux 6.8.0-111-generic
- Toolchain: Go 1.26.2, Rust 1.93.1, wrk 4.1.0, sysstat 12.6.1
- Workload: CPU-bound normal handler routes only
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- Benchmark profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `BENCH_ENABLE_DB=0`, `BENCH_ENABLE_PPROF=0`
- Transport path: Wing on Linux for Kruda, same-host loopback for all frameworks
- Non-goals: TLS, HTTP/2, database routes, fortunes, and production network behavior

Raw evidence:

- `main-984f0d6-20260524T171346Z/`: five measured rounds plus warmup for each framework, route, and profile
- `resource-main-984f0d6-20260524T174429Z/`: CPU, RSS, and RPS/core evidence from `pidstat`

## Validation

- Socket errors: zero across every measured row.
- Non-2xx responses: zero across every measured row.
- The benchmark uses normal handler routes. Wing static bypass route options are intentionally separate from fair handler-path claims.

## Median Benchmark Results

| Profile | Route | Kruda RPS | Kruda p99 ms | Fiber RPS | Fiber p99 ms | Actix RPS | Actix p99 ms | Kruda vs Actix RPS | Kruda vs Actix p99 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| latency | `/plaintext-handler` | 803656.06 | 0.589 | 627949.14 | 2.870 | 696448.06 | 2.980 | +15.39% | -80.23% |
| latency | `/json-static` | 809703.13 | 0.580 | 618381.40 | 2.880 | 690561.46 | 2.760 | +17.25% | -78.99% |
| latency | `/json-serialize` | 794063.71 | 0.577 | 600289.11 | 3.080 | 685303.65 | 2.750 | +15.87% | -79.02% |
| throughput | `/plaintext-handler` | 809773.82 | 0.835 | 642443.17 | 3.320 | 722328.75 | 3.640 | +12.11% | -77.06% |
| throughput | `/json-static` | 808953.90 | 0.832 | 634054.54 | 3.400 | 712763.16 | 3.230 | +13.50% | -74.24% |
| throughput | `/json-serialize` | 798032.15 | 0.860 | 620706.73 | 3.330 | 706941.96 | 3.170 | +12.89% | -72.87% |

## Claim Gate

This evidence satisfies the public "faster than Actix" gate for the listed CPU-bound Wing handler routes under both benchmark profiles:

- Kruda median RPS is at least 3% higher than Actix.
- Kruda p99 latency is not worse than 10% above Actix.
- Socket errors and non-2xx responses are zero.

The claim is limited to this CPU-bound same-host loopback evidence. Do not generalize it to database workloads, TLS, HTTP/2, or production network behavior.

## Resource Evidence

| Profile | Route | Kruda RPS/core | Actix RPS/core | Kruda avg CPU % | Actix avg CPU % | Kruda max RSS MB | Actix max RSS MB |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `/plaintext-handler` | 216621 | 181527 | 384.87 | 381.93 | 15.88 | 7.82 |
| latency | `/json-static` | 211982 | 172793 | 384.13 | 409.19 | 17.50 | 7.95 |
| latency | `/json-serialize` | 205437 | 174440 | 386.94 | 396.33 | 22.26 | 7.95 |
| throughput | `/plaintext-handler` | 212850 | 181238 | 386.13 | 397.78 | 20.92 | 10.07 |
| throughput | `/json-static` | 206747 | 184283 | 387.39 | 381.54 | 21.92 | 10.20 |
| throughput | `/json-serialize` | 206567 | 177261 | 386.13 | 395.07 | 24.25 | 10.20 |

Kruda uses CPU in the same range as Actix and has higher RPS/core in this run. Actix still has lower RSS, so this evidence should not be described as a memory-footprint win.
