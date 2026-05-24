# SendStaticJSON Handler-Path Evidence

This note summarizes the tiger evidence for the handler-level `SendStaticJSON`
helper. This is not a Wing static bypass route: the handler, middleware,
lifecycle hooks, cookies, CORS, and security header behavior remain part of the
normal route path.

## Scope

- Workload: CPU-bound normal handler route only
- Route: `/json-static`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- Kruda benchmark Go tags: `kruda_stdjson`
- Kruda benchmark pprof server: excluded from the default binary and disabled at runtime (`BENCH_ENABLE_PPROF=0`)
- Host: tiger dev server, Linux `dev-server 6.8.0-111-generic`, Intel Core i5-13500, 8 vCPU
- Evidence directory: `20260524Tphase9-send-static-json-readbuf4k-p3620/`

## Throughput Profile Summary

| Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda median p99 | Actix median p99 | Socket errors | Non-2xx |
|------|-----------------:|-----------------:|-------------------:|-----------------:|-----------------:|--------------:|--------:|
| `/json-static` | 820646.04 | 737305.93 | +11.30% | 1.060 ms | 3.040 ms | 0 | 0 |

## Interpretation

`SendStaticJSON` avoids the content-type comparison in the existing
`SendStaticWithTypeBytes` JSON path while preserving normal handler semantics.
The route still passes the public Actix win gate for this CPU-bound profile,
but this is not a 20% throughput win and should not be described as one.
