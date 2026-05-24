# KRUDA_WORKERS=4 Resource Evidence

This note summarizes the tiger resource benchmark in `resource-20260524Tphase5-k4-pprof-off/`.

## Scope

- Host: tiger `dev-server`
- CPU: 8 vCPU, 13th Gen Intel(R) Core(TM) i5-13500 under KVM
- OS: Linux 6.8.0-111-generic
- Toolchain: Go 1.26.2, Rust 1.93.1, wrk 4.1.0, sysstat 12.6.1
- Command shape: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`
- Kruda benchmark pprof server: disabled (`BENCH_ENABLE_PPROF=0`)
- Ports: Kruda `3300`, Fiber `3302`, Actix `3303`
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- DB routes are disabled.

Command:

```bash
cd /home/tiger/kruda-p99-rss-phase5/bench/reproducible
RESULT_DIR=/home/tiger/kruda-p99-rss-phase5/bench/reproducible/results/resource-20260524Tphase5-k4-pprof-off \
  GOMAXPROCS=8 \
  KRUDA_WORKERS=4 \
  KRUDA_PORT=3300 \
  FIBER_PORT=3302 \
  ACTIX_PORT=3303 \
  ./resource.sh
```

## Validation

- Socket errors: zero across every row.
- Non-2xx responses: zero across every row.
- No Kruda/Fiber/Actix benchmark processes were left running after the harness exited.

## Throughput Profile

| Route | Kruda RPS | Actix RPS | Kruda vs Actix RPS | Kruda p99 ms | Actix p99 ms | Kruda vs Actix p99 | Kruda avg CPU % | Actix avg CPU % | Kruda max RSS MB | Actix max RSS MB | Kruda RPS/core | Actix RPS/core |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `/plaintext-handler` | 833728.62 | 744944.17 | +11.92% | 0.771 | 3.650 | -78.88% | 386.87 | 401.27 | 21.69 | 10.32 | 215506 | 185647 |
| `/json-static` | 836681.14 | 736451.44 | +13.61% | 0.831 | 3.100 | -73.19% | 386.27 | 386.54 | 23.19 | 10.57 | 216605 | 190524 |
| `/json-serialize` | 798468.64 | 738918.95 | +8.06% | 1.250 | 3.580 | -65.08% | 381.94 | 404.93 | 24.53 | 10.57 | 209056 | 182481 |

## Average Resource Summary

| Profile | Framework | Avg RPS | Avg p99 ms | Avg CPU % | Max RSS MB | Avg RPS/core |
|---|---|---:|---:|---:|---:|---:|
| latency | Kruda | 806322.17 | 0.775 | 386.76 | 21.10 | 208481 |
| latency | Fiber | 636178.42 | 2.667 | 405.33 | 17.76 | 157004 |
| latency | Actix | 702156.20 | 2.850 | 384.83 | 8.20 | 182492 |
| throughput | Kruda | 822959.47 | 0.951 | 385.03 | 24.53 | 213722 |
| throughput | Fiber | 645666.93 | 3.657 | 407.09 | 22.41 | 158637 |
| throughput | Actix | 740104.85 | 3.443 | 397.58 | 10.57 | 186217 |

## Change From Previous Resource Evidence

Compared with the prior `KRUDA_WORKERS=8` Kruda resource run in `resource-20260524Tphase4-current/`, the throughput-profile average changed as follows:

| Metric | Change |
|---|---:|
| Avg RPS | +2.06% |
| Avg p99 | -74.67% |
| Avg CPU % | -8.11% |
| Max RSS | -8.16% |
| Avg RPS/core | +11.06% |

## Interpretation

This resource run shows that the benchmark harness should align Kruda workers with the active `wrk -t4` CPU-bound profiles. It improves tail latency, CPU efficiency, and RSS without changing Kruda's framework defaults. Kruda still uses more RSS than Actix in this run, so memory parity remains future work.
