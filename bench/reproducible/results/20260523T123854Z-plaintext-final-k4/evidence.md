# Wing Plaintext Handler Phase 1 Evidence

This evidence records the Phase 1 fair handler-path run for `GET /plaintext-handler`.
It is not a static bypass benchmark and does not include DB or fortunes routes.

## Environment

- Host: tiger Linux benchmark server
- CPU: 13th Gen Intel Core i5-13500 under KVM, 8 vCPU
- Kernel: Linux 6.8.0-111-generic
- Go: go1.26.2 linux/amd64
- Rust: rustc 1.93.1
- wrk: 4.1.0
- Kruda: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`
- Route: `GET /plaintext-handler`
- Profiles: latency `-t4 -c128 -d15s`, throughput `-t4 -c256 -d15s`
- Rounds: one warmup plus five measured rounds per framework/profile

Command:

```bash
KRUDA_PORT=13000 FIBER_PORT=13002 ACTIX_PORT=13003 KRUDA_WORKERS=4 BENCH_ROUNDS=5 BENCH_DURATION=15s RESULT_DIR=results/20260523T123854Z-plaintext-final-k4 ./bench.sh plaintext-handler
```

## Median Summary

| Profile | Framework | Median RPS | Median p50 | Median p90 | Median p99 | Socket errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|
| latency | Kruda | 807032.72 | 0.112 ms | 0.217 ms | 0.551 ms | 0 | 0 |
| latency | Fiber | 639856.69 | 0.137 ms | 0.525 ms | 2.520 ms | 0 | 0 |
| latency | Actix | 709835.81 | 0.091 ms | 0.980 ms | 3.240 ms | 0 | 0 |
| throughput | Kruda | 812782.42 | 0.200 ms | 0.404 ms | 0.794 ms | 0 | 0 |
| throughput | Fiber | 654458.99 | 0.261 ms | 0.990 ms | 3.220 ms | 0 | 0 |
| throughput | Actix | 734378.34 | 0.169 ms | 1.410 ms | 3.450 ms | 0 | 0 |

## Claim Gate

Kruda is in the same ballpark as Actix for this fair plaintext handler workload and has lower p99 latency in this run. It does not satisfy the stretch gate for a 20% median RPS lead over Actix.

- Latency profile median RPS delta vs Actix: +13.7%
- Throughput profile median RPS delta vs Actix: +10.7%
- Kruda p99 is lower than Actix in both profiles.
- All measured rounds have zero socket errors and zero non-2xx responses.

Raw wrk output, server logs, environment metadata, `summary.csv`, and `summary.md` are included in this directory.
