# Wing Single-Handler Evidence

This evidence covers the Wing single-handler fast dispatch change for normal Kruda handler-path routes. It is not Wing static bypass evidence.

## Environment

The end-to-end benchmark was collected on `tiger-linux` (`dev-server`) with an Intel Core i5-13500 VM, 8 online CPUs, Linux 6.8.0-111-generic, Go 1.26.2, Rust 1.93.1, and wrk 4.1.0.

All runs used `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`, `BENCH_ENABLE_PPROF=0`, one warmup round, and five measured rounds per framework/profile/route.

Profiles:

| Profile | wrk command profile |
|---------|---------------------|
| latency | `wrk --latency -t4 -c128 -d15s` |
| throughput | `wrk --latency -t4 -c256 -d15s` |

## Throughput Profile Gate

The throughput-profile medians below satisfy the public "faster than Actix" gate for CPU-bound Wing handler routes: Kruda median RPS is at least 3% higher than Actix, Kruda p99 is not worse than 10% above Actix, and socket errors plus non-2xx responses are zero.

| Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda median p99 | Actix median p99 | Kruda vs Actix p99 | Socket errors | Non-2xx |
|------|-----------------:|-----------------:|-------------------:|-----------------:|-----------------:|-------------------:|--------------:|--------:|
| `/plaintext-handler` | 823554.65 | 734767.90 | +12.08% | 0.980 ms | 3.230 ms | -69.66% | 0 | 0 |
| `/json-static` | 814008.78 | 738144.14 | +10.28% | 0.930 ms | 3.260 ms | -71.47% | 0 | 0 |
| `/json-serialize` | 806376.39 | 721754.24 | +11.72% | 1.130 ms | 3.140 ms | -64.01% | 0 | 0 |

## Latency Profile Cross-Check

The latency profile also stayed ahead of Actix on median RPS and p99 for the same routes, with zero socket errors and zero non-2xx responses.

| Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda median p99 | Actix median p99 | Kruda vs Actix p99 |
|------|-----------------:|-----------------:|-------------------:|-----------------:|-----------------:|-------------------:|
| `/plaintext-handler` | 820637.73 | 705493.21 | +16.32% | 0.764 ms | 2.970 ms | -74.28% |
| `/json-static` | 822538.37 | 715197.80 | +15.01% | 0.843 ms | 3.120 ms | -72.98% |
| `/json-serialize` | 793913.03 | 705878.46 | +12.47% | 1.480 ms | 2.810 ms | -47.33% |

## Raw Evidence

- `results/20260524Tphase11-single-handler-plaintext-readbuf4k-p3650/`
- `results/20260524Tphase11-single-handler-json-static-readbuf4k-p3660/`
- `results/20260524Tphase11-single-handler-json-serialize-readbuf4k-p3670/`

Each directory contains `environment.txt`, raw `wrk --latency` output, `summary.csv`, and `summary.md`.

## Microbenchmarks

Before/after microbenchmarks compared `origin/main` against this branch on an Apple M3 with `benchstat`.

Main hot-path run:

- `BenchmarkCPUFullCycle`: 293.4 ns/op to 256.0 ns/op, 12.75% faster.
- `BenchmarkCPUHandlerInline`: 275.6 ns/op to 256.5 ns/op, 6.93% faster.
- `BenchmarkCPUHandlerPlaintextFeather`: 226.5 ns/op to 198.6 ns/op, 12.32% faster.
- No `B/op` or `allocs/op` increase on the measured CPU-bound hot paths.

The initial five-sample run showed a noisy `BenchmarkJSON` ns/op movement. A targeted ten-sample rerun showed no significant `BenchmarkJSON` regression and no `B/op` or `allocs/op` increase:

- `BenchmarkJSON`: 296.5 ns/op to 298.1 ns/op, not significant.
- `BenchmarkCPUHandlerJSONSerializeFeather`: 372.1 ns/op to 358.7 ns/op, 3.63% faster.
- No `B/op` or `allocs/op` increase on the rerun.

Microbenchmark outputs:

- `results/20260524Tphase11-single-handler-microbench.txt`
- `results/20260524Tphase11-single-handler-json-rerun-microbench.txt`

