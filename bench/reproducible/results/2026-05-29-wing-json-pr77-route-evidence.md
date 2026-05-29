# Wing JSON Route Evidence After PR #77

Date: 2026-05-29
Host: tiger dev server
Source commit: `891a758` (`perf: pre-size Wing JSON response buffers`)
Evidence directory: `pr77-main-default-json-ports3100-20260529T155723Z/`

## Scope

This evidence records post-PR #77 route-level benchmark results for JSON-focused CPU-bound routes.

Routes:

- `/json-static`
- `/json-serialize`

Run shape:

- `KRUDA_GO_TAGS=default`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- `BENCH_ENABLE_DB=0`
- `BENCH_ENABLE_PPROF=0`
- `wrk --latency -t4 -c128 -d15s` latency profile
- `wrk --latency -t4 -c256 -d15s` throughput profile
- 5 measured rounds per framework/profile/route after warmup
- Alternate ports `3100`, `3102`, and `3103` were used because the default benchmark ports were already occupied on the host.

## Median Summary

| Profile | Route | Framework | Median RPS | Median p99 | Socket errors | Non-2xx |
|---|---|---|---:|---:|---:|---:|
| latency | `/json-static` | Kruda | 811,113.94 | 0.847 ms | 0 | 0 |
| latency | `/json-static` | Fiber | 633,074.83 | 2.770 ms | 0 | 0 |
| latency | `/json-static` | Actix | 709,871.24 | 2.950 ms | 0 | 0 |
| latency | `/json-serialize` | Kruda | 787,339.67 | 0.860 ms | 0 | 0 |
| latency | `/json-serialize` | Fiber | 608,943.62 | 3.010 ms | 0 | 0 |
| latency | `/json-serialize` | Actix | 697,528.47 | 3.100 ms | 0 | 0 |
| throughput | `/json-static` | Kruda | 815,337.49 | 0.814 ms | 0 | 0 |
| throughput | `/json-static` | Fiber | 653,448.00 | 3.290 ms | 0 | 0 |
| throughput | `/json-static` | Actix | 734,180.75 | 3.300 ms | 0 | 0 |
| throughput | `/json-serialize` | Kruda | 800,995.52 | 1.020 ms | 0 | 0 |
| throughput | `/json-serialize` | Fiber | 635,652.78 | 3.460 ms | 0 | 0 |
| throughput | `/json-serialize` | Actix | 727,242.17 | 3.230 ms | 0 | 0 |

## Actix Gate

Using the benchmark claim gate from `bench/reproducible/README.md`, Kruda passes the JSON-route gate for this run:

| Profile | Route | Kruda vs Actix median RPS | Kruda vs Actix median p99 | Gate |
|---|---|---:|---:|---|
| latency | `/json-static` | +14.26% | -71.29% | Pass |
| latency | `/json-serialize` | +12.88% | -72.26% | Pass |
| throughput | `/json-static` | +11.05% | -75.33% | Pass |
| throughput | `/json-serialize` | +10.14% | -68.42% | Pass |

## Interpretation

This run supports a JSON-route-specific claim on the current tiger benchmark host after PR #77. It does not, by itself, isolate the route-level delta caused by PR #77 because the previous commit was not rerun in the same benchmark session.

The PR #77 microbench evidence remains the direct before/after evidence for the response-buffer change. This route evidence is a current post-merge cross-runtime check for the JSON workload profile.

Do not use this run as evidence for plaintext routes, DB routes, fortunes, or static bypass routes.
