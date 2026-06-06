# Clean CPU/RAM Resource Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`
Commit under test: `805b0d5`

## Scope

This run records CPU and RSS evidence after the readiness guard fix. It uses the
same CPU-bound handler routes as the reproducible cross-runtime benchmark and
non-default high ports to avoid tiger service collisions.

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/clean-resource-20260604T160505Z`

Raw `wrk` logs, pidstat logs, `resource-summary.csv`,
`resource-aggregated.csv`, and environment metadata are stored in that result
directory on tiger.

## Method

Command shape:

```bash
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_PORT=3210 \
FIBER_PORT=3212 \
ACTIX_PORT=3213 \
./resource.sh plaintext-handler json-static json-serialize
```

Resource columns:

- CPU percentage is process CPU across cores.
- RSS is resident memory from `pidstat`.
- RPS/core is RPS divided by average CPU cores consumed.

## Summary

| Profile | Framework | Route | RPS | p99 ms | Avg CPU % | Max RSS MB | RPS/core | Errors |
|---|---|---|---:|---:|---:|---:|---:|---:|
| latency | kruda | `plaintext-handler` | 828,851.98 | 0.727 | 385.41 | 16.25 | 215,057 | 0 |
| latency | kruda | `json-static` | 796,923.79 | 0.496 | 386.67 | 17.75 | 206,099 | 0 |
| latency | kruda | `json-serialize` | 829,753.22 | 0.454 | 387.81 | 19.00 | 213,959 | 0 |
| throughput | kruda | `plaintext-handler` | 826,693.71 | 0.692 | 387.13 | 20.38 | 213,544 | 0 |
| throughput | kruda | `json-static` | 832,430.88 | 0.767 | 385.87 | 21.75 | 215,728 | 0 |
| throughput | kruda | `json-serialize` | 828,632.90 | 0.716 | 387.94 | 21.38 | 213,598 | 0 |
| latency | fiber | `plaintext-handler` | 641,758.92 | 2.490 | 409.60 | 11.88 | 156,679 | 0 |
| latency | fiber | `json-static` | 634,531.59 | 2.720 | 407.52 | 12.00 | 155,706 | 0 |
| latency | fiber | `json-serialize` | 609,036.86 | 2.920 | 414.32 | 17.38 | 146,997 | 0 |
| throughput | fiber | `plaintext-handler` | 647,843.65 | 3.480 | 412.37 | 17.69 | 157,103 | 0 |
| throughput | fiber | `json-static` | 646,294.17 | 3.370 | 410.06 | 17.69 | 157,610 | 0 |
| throughput | fiber | `json-serialize` | 627,159.76 | 3.620 | 414.18 | 22.14 | 151,422 | 0 |
| latency | actix | `plaintext-handler` | 712,201.01 | 3.060 | 397.20 | 7.95 | 179,305 | 0 |
| latency | actix | `json-static` | 693,051.73 | 2.890 | 385.07 | 8.07 | 179,981 | 0 |
| latency | actix | `json-serialize` | 704,911.81 | 2.940 | 391.75 | 8.07 | 179,939 | 0 |
| throughput | actix | `plaintext-handler` | 728,631.13 | 3.120 | 389.00 | 10.32 | 187,309 | 0 |
| throughput | actix | `json-static` | 730,945.89 | 3.250 | 398.93 | 10.45 | 183,227 | 0 |
| throughput | actix | `json-serialize` | 726,368.87 | 3.180 | 391.45 | 10.45 | 185,559 | 0 |

## Interpretation

- Kruda has the highest RPS/core across every measured route and profile.
- Kruda CPU consumption is roughly in the same band as Actix and lower than
  Fiber in this run, while producing higher RPS and lower p99 latency.
- Kruda RSS is higher than Actix, especially in throughput profiles, but is in
  the same general band as Fiber and lower than Fiber for throughput
  `json-serialize`.
- This supports the current public claim shape: faster-than-Actix on the
  measured CPU-bound handler routes, with the resource caveat that Actix still
  has lower RSS on this host.
