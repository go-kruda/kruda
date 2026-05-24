# CPU Benchmark Pprof Build Exclusion Evidence

This note summarizes the tiger resource benchmark in `resource-20260524Tphase6-no-pprof-build-rerun/`.

## Scope

- Host: tiger-linux
- CPU: Intel Core i5-13500, 8 logical CPUs
- Kernel: Linux 6.8.0-111-generic
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- Profiles:
  - latency: `wrk --latency -t4 -c128 -d15s`
  - throughput: `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`
- Kruda benchmark Go tags: `kruda_stdjson`
- Kruda benchmark pprof server: excluded from the default binary and disabled at runtime (`BENCH_ENABLE_PPROF=0`)

## Result

The run shows zero socket errors and zero non-2xx responses for every framework, route, and profile.

Throughput profile, Kruda vs Actix:

| Route | Kruda RPS | Actix RPS | RPS delta | Kruda p99 | Actix p99 | p99 delta | Kruda RSS | Actix RSS |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `/plaintext-handler` | 835187.06 | 752886.69 | +10.93% | 1.030ms | 3.270ms | -68.50% | 21.07MB | 10.45MB |
| `/json-static` | 825018.49 | 747466.82 | +10.38% | 1.070ms | 3.250ms | -67.08% | 23.57MB | 10.45MB |
| `/json-serialize` | 804931.49 | 737094.27 | +9.20% | 1.080ms | 3.960ms | -72.73% | 23.50MB | 10.45MB |

Compared with the previous `resource-20260524Tphase5-k4-pprof-off/` throughput profile:

| Metric | Phase 5 | Phase 6 | Delta |
|---|---:|---:|---:|
| Kruda avg RPS | 822959.47 | 821712.35 | -0.15% |
| Kruda avg p99 | 0.951ms | 1.060ms | +11.46% |
| Kruda max RSS | 24.53MB | 23.57MB | -3.91% |
| Kruda avg CPU | 385.03% | 384.69% | -0.09% |
| Kruda avg RPS/core | 213722 | 213604 | -0.06% |

The pprof build exclusion modestly reduces the observed Kruda RSS while keeping throughput effectively flat. The phase 6 p99 remains materially lower than Actix for the CPU-bound throughput routes, but Kruda RSS remains higher than Actix. This evidence supports the same throughput/p99 claim gate as the prior handler-path evidence and does not change the conclusion that Actix still has the lower resident memory footprint.
