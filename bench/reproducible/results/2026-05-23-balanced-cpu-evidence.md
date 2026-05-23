# Balanced CPU-Bound Evidence

This evidence was collected on `tiger-linux` (`dev-server`) with an Intel Core i5-13500 VM, 8 online CPUs, Linux 6.8.0-111-generic, Go 1.26.2, Rust 1.93.1, and wrk 4.1.0.

## CPU-Bound Handler Benchmark

Directory: `20260522T185949Z`

- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Rounds: one warmup plus five measured rounds per framework, route, and profile
- Errors: zero socket errors and zero non-2xx responses in all measured rounds

Throughput-profile median results:

| Route | Kruda RPS | Kruda p99 ms | Fiber RPS | Fiber p99 ms | Actix RPS | Actix p99 ms |
|---|---:|---:|---:|---:|---:|---:|
| plaintext-handler | 738306.04 | 4.160 | 653176.54 | 3.490 | 735755.24 | 3.190 |
| json-static | 681857.48 | 5.540 | 644191.88 | 3.460 | 731201.09 | 3.250 |
| json-serialize | 676348.74 | 5.330 | 625619.62 | 3.880 | 717438.54 | 3.020 |

Kruda is competitive on throughput and above Fiber on these CPU-bound routes, but this run does not satisfy the public "faster than Actix" gate because Kruda p99 latency is more than 10% above Actix. The correct public claim for this evidence is "same ballpark as Actix."

## CPU And Memory Sample

Directory: `resource-20260523T021154Z`

This resource pass records one measured run per framework, route, and profile with wrk output plus pidstat logs. The derived aggregation is in `resource-aggregated.csv`.

Throughput-profile averages across the three CPU-bound routes:

| Framework | Avg RPS | Avg p99 ms | Avg CPU % | Max RSS MB | Avg RPS/core |
|---|---:|---:|---:|---:|---:|
| Kruda | 695715.63 | 5.467 | 417.4 | 37.01 | 166640 |
| Fiber | 639673.29 | 3.680 | 415.1 | 21.95 | 154139 |
| Actix | 721199.94 | 3.327 | 397.1 | 10.32 | 181615 |

Kruda used more memory than Fiber and Actix in this run. CPU usage was close to Fiber and above Actix, while RPS per core was above Fiber and below Actix.

## Microbenchmark Gate

Directory: `microbench-20260523T060000Z`

The microbenchmark comparison used `origin/main` as the baseline and this PR branch as the candidate on an Apple M3 with `GOMAXPROCS=8`, `-benchmem`, and `-count=5`.

- CPU hot-path geomean: 127.8 ns/op to 125.7 ns/op, 1.65% faster
- CPU hot-path allocations: no B/op or allocs/op increase on plaintext, JSON, parser, response builder, full cycle, or feather lookup
- Static cached response path: 31.36 ns/op and 1 alloc/op to 24.76 ns/op and 0 alloc/op

The microbenchmark gate passes: no CPU hot-path regression above 5% and no allocation increase on CPU-bound hot paths.
