# CPU/RAM Resource Evidence

This note summarizes the tiger resource benchmark in `resource-20260524Tphase4-current/`.

## Scope

- Host: tiger `dev-server`
- CPU: 8 vCPU, 13th Gen Intel(R) Core(TM) i5-13500 under KVM
- OS: Linux 6.8.0-111-generic
- Toolchain: Go 1.26.2, Rust 1.93.1, wrk 4.1.0, sysstat 12.6.1
- Command shape: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=8`
- Ports: Kruda `3100`, Fiber `3102`, Actix `3103`
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- DB routes are disabled.

Command:

```bash
cd /home/tiger/kruda-resource-phase4/bench/reproducible
RESULT_DIR=/home/tiger/kruda-resource-phase4/bench/reproducible/results/resource-20260524Tphase4-current \
  GOMAXPROCS=8 \
  KRUDA_WORKERS=8 \
  KRUDA_PORT=3100 \
  FIBER_PORT=3102 \
  ACTIX_PORT=3103 \
  ./resource.sh
```

## Validation

- Socket errors: zero across every row.
- Non-2xx responses: zero across every row.
- No Kruda/Fiber/Actix benchmark processes were left running after the harness exited.

## Throughput Profile

| Route | Kruda RPS | Actix RPS | Kruda vs Actix RPS | Kruda p99 ms | Actix p99 ms | Kruda vs Actix p99 | Kruda avg CPU % | Actix avg CPU % | Kruda max RSS MB | Actix max RSS MB | Kruda RPS/core | Actix RPS/core |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `/plaintext-handler` | 813896.06 | 734983.54 | +10.74% | 3.620 | 3.110 | +16.40% | 419.24 | 381.03 | 23.33 | 10.45 | 194136 | 192894 |
| `/json-static` | 808414.35 | 736223.04 | +9.81% | 3.530 | 3.160 | +11.71% | 418.67 | 387.35 | 25.33 | 10.57 | 193091 | 190067 |
| `/json-serialize` | 796820.54 | 735724.38 | +8.30% | 4.110 | 3.180 | +29.25% | 419.18 | 401.80 | 26.71 | 10.57 | 190090 | 183107 |

## Average Resource Summary

| Profile | Framework | Avg RPS | Avg p99 ms | Avg CPU % | Max RSS MB | Avg RPS/core |
|---|---|---:|---:|---:|---:|---:|
| latency | Kruda | 790047.13 | 3.583 | 420.15 | 24.47 | 188147 |
| latency | Fiber | 637216.65 | 2.840 | 410.13 | 17.88 | 155435 |
| latency | Actix | 709698.57 | 2.853 | 389.26 | 8.07 | 182380 |
| throughput | Kruda | 806376.98 | 3.753 | 419.03 | 26.71 | 192439 |
| throughput | Fiber | 654503.83 | 3.347 | 409.93 | 22.89 | 159693 |
| throughput | Actix | 735643.65 | 3.150 | 390.06 | 10.57 | 188689 |

## Change From Previous Resource Evidence

Compared with the older Kruda resource run in `resource-20260523T021154Z/`, the throughput-profile average changed as follows:

| Metric | Change |
|---|---:|
| Avg RPS | +15.91% |
| Avg p99 | -31.34% |
| Avg CPU % | +0.39% |
| Max RSS | -27.83% |
| Avg RPS/core | +15.48% |

## Interpretation

This resource run is single-sample evidence for CPU/RAM behavior, not the five-round public claim gate. It shows Kruda throughput and RPS/core ahead of Actix for these CPU-bound routes, with zero errors and zero non-2xx responses. It also shows Kruda still using more RSS than Actix, and its p99 in this single resource pass is above Actix, so this file should not be used by itself to make a broader "faster than Actix" claim.
