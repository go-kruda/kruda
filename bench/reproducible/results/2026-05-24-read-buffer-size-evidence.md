# Wing Read Buffer Resource Evidence

This note summarizes the tiger resource benchmark in `resource-20260524Tphase7-readbuf4k/`.

## Scope

- Workload: CPU-bound normal handler routes only
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- Kruda benchmark Go tags: `kruda_stdjson`
- Kruda benchmark pprof server: excluded from the default binary and disabled at runtime (`BENCH_ENABLE_PPROF=0`)
- Host: tiger dev server, Linux `dev-server 6.8.0-111-generic`, Intel Core i5-13500, 8 vCPU

`KRUDA_READ_BUF_SIZE=4096` is an explicit workload-specific memory profile. It is not the Kruda framework default. Smaller read buffers can reject requests whose request line and headers do not fit the configured buffer.

## Throughput Profile Summary

| Route | Kruda RPS | Actix RPS | Kruda vs Actix RPS | Kruda p99 | Actix p99 | Kruda max RSS | Actix max RSS |
|------|----------:|----------:|-------------------:|----------:|----------:|--------------:|--------------:|
| `/plaintext-handler` | 848169.94 | 743301.11 | +14.11% | 0.632 ms | 3.150 ms | 18.80 MB | 10.20 MB |
| `/json-static` | 841818.81 | 747995.33 | +12.54% | 0.652 ms | 3.220 ms | 19.42 MB | 10.32 MB |
| `/json-serialize` | 796066.33 | 727273.77 | +9.46% | 0.736 ms | 2.970 ms | 20.95 MB | 10.32 MB |

All measured rows reported zero socket errors and zero non-2xx responses.

## Phase 6 To Phase 7 Kruda RSS Change

| Profile | Route | Phase 6 max RSS | Phase 7 max RSS | Change |
|---------|------|----------------:|----------------:|-------:|
| latency | `/plaintext-handler` | 16.88 MB | 15.75 MB | -6.69% |
| latency | `/json-static` | 19.00 MB | 16.62 MB | -12.53% |
| latency | `/json-serialize` | 20.85 MB | 19.88 MB | -4.65% |
| throughput | `/plaintext-handler` | 21.07 MB | 18.80 MB | -10.77% |
| throughput | `/json-static` | 23.57 MB | 19.42 MB | -17.61% |
| throughput | `/json-serialize` | 23.50 MB | 20.95 MB | -10.85% |

## Interpretation

The 4KB Wing read-buffer profile reduces Kruda RSS for short-header CPU-bound benchmark traffic while preserving zero errors and the existing throughput/p99 advantage over Actix in this run. Actix still has lower RSS, so this evidence should be described as RSS reduction and workload-specific tuning, not as a memory-footprint win over Actix.
