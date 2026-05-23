# Phase 2 JSON Handler Evidence

This evidence covers the Phase 2 Wing handler-path JSON changes on the tiger Linux benchmark host.

## Scope

- Routes: `/json-static`, `/json-serialize`
- Profiles:
  - latency: `wrk --latency -t4 -c128 -d15s`
  - throughput: `wrk --latency -t4 -c256 -d15s`
- Rounds: one warmup plus five measured rounds per framework, route, and profile
- Workers: `KRUDA_WORKERS=4`
- Frameworks: Kruda, Fiber, Actix
- Result directory: `bench/reproducible/results/20260523T170148Z/`

## Summary

Kruda remains in the same ballpark as Actix for these JSON handler-path routes. It does not meet an Actix win gate on median RPS, but it shows lower p99 latency than Actix in every measured JSON profile with zero socket errors and zero non-2xx responses.

| Profile | Route | Kruda median RPS | Kruda p99 ms | Actix median RPS | Actix p99 ms | Kruda vs Actix RPS | Kruda vs Actix p99 |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| latency | json-static | 698064.31 | 2.520 | 708962.33 | 3.170 | -1.54% | -20.50% |
| latency | json-serialize | 688902.02 | 2.520 | 710238.73 | 2.970 | -3.00% | -15.15% |
| throughput | json-static | 705718.60 | 2.930 | 731266.67 | 3.440 | -3.49% | -14.83% |
| throughput | json-serialize | 695060.56 | 2.810 | 728711.40 | 3.340 | -4.62% | -15.87% |

## Microbenchmark Summary

Same-machine before/after microbenchmarks compare `origin/main` against this branch. The strongest local improvement is the handler-level static JSON path:

- `BenchmarkCPUHandlerJSONStaticFeather`: 342.7 ns/op to 296.4 ns/op, 13.51% faster.
- `BenchmarkCPUHandlerJSONStaticFeather`: 520 B/op to 488 B/op.
- `BenchmarkCPUHandlerJSONStaticFeather`: 6 allocs/op to 5 allocs/op.
- `BenchmarkCPUHandlerJSONSerializeFeather`: 380.9 ns/op to 363.4 ns/op, 4.58% faster.
- `BenchmarkCPUResponseJSON`: 102.55 ns/op to 94.20 ns/op, 8.14% faster.
- No measured allocation increase on the covered CPU hot paths.

Full microbenchmark output is included in:

- `microbench-main.txt`
- `microbench-perf.txt`
- `microbench-benchstat.txt`

## Claim Boundary

This evidence supports a narrower JSON handler-path improvement and lower p99 latency versus Actix in this run. It does not support a claim that Kruda is faster than Actix on JSON median RPS.
