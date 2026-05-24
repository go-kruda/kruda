# Wing JSON Inline Evidence

This evidence was collected on `tiger-linux` (`dev-server`) with an Intel Core i5-13500 VM, 8 online CPUs, Linux 6.8.0-111-generic, Go 1.26.2, Rust 1.93.1, and wrk 4.1.0.

## Scope

Directory: `20260524Tphase3-json-final`

- Routes: `/json-static`, `/json-serialize`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Rounds: one warmup plus five measured rounds per framework, route, and profile
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `BENCH_ENABLE_DB=0`
- Errors: zero socket errors and zero non-2xx responses in all measured rounds

This is handler-path evidence for Wing JSON routes. It is separate from Wing static bypass route options and does not benchmark DB or fortunes routes.

## Median Results

| Profile | Route | Kruda RPS | Kruda p99 ms | Fiber RPS | Fiber p99 ms | Actix RPS | Actix p99 ms | Kruda vs Actix RPS | Kruda vs Actix p99 |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|
| latency | json-static | 807664.24 | 0.860 | 627507.92 | 2.990 | 690469.34 | 2.810 | +16.97% | -69.40% |
| latency | json-serialize | 790803.39 | 0.821 | 610264.70 | 3.150 | 689458.20 | 2.860 | +14.70% | -71.29% |
| throughput | json-static | 811740.53 | 1.000 | 639107.47 | 3.450 | 712263.97 | 3.260 | +13.97% | -69.33% |
| throughput | json-serialize | 791812.23 | 1.030 | 624483.98 | 3.720 | 706101.18 | 3.220 | +12.14% | -68.01% |

## Claim Gate

For these two Wing JSON handler routes, this run satisfies the public "faster than Actix" gate: Kruda median RPS is at least 3% higher than Actix, p99 latency is not worse than 10% above Actix, and all measured rounds have zero socket errors and zero non-2xx responses.

The claim is limited to these CPU-bound Wing JSON handler routes and this benchmark profile.
