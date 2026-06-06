# Clean Cross-Runtime CPU-Bound Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`
Commit under test: `a7005aa`

## Scope

This run validates the reproducible CPU-bound benchmark after the readiness
guard fix in `e7fda49` (`bench: reject occupied readiness ports`). The run used
non-default high ports to avoid tiger services already bound to `:3000`.

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/clean-cross-runtime-20260604T153451Z`

Raw logs, `summary.csv`, `summary.md`, `claim-gates.md`, and environment
metadata are stored in that result directory on tiger.

## Method

Command shape:

```bash
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_ROUNDS=5 \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_PORT=3200 \
FIBER_PORT=3202 \
ACTIX_PORT=3203 \
./bench.sh plaintext-handler json-static json-serialize
```

Environment highlights:

- CPU: 13th Gen Intel Core i5-13500, 8 cores, SMT off in the guest
- OS/kernel: Linux 6.8.0-117-generic
- Go: 1.25.10 linux/amd64
- Rust: 1.93.1
- wrk: 4.1.0
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- profiles:
  - latency: `wrk --latency -t4 -c128 -d15s`
  - throughput: `wrk --latency -t4 -c256 -d15s`

## Claim Gates

Rule: faster-than-Actix requires Kruda median RPS at least 3% higher than Actix,
Kruda p99 no worse than 10% above Actix, and zero socket errors plus zero
non-2xx responses.

| Profile | Route | Kruda median RPS | Actix median RPS | RPS delta | Kruda median p99 ms | Actix median p99 ms | p99 delta | Errors | Gate |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| latency | `json-serialize` | 805,923.52 | 693,355.72 | +16.24% | 0.556 | 2.610 | -78.70% | 0 | faster-than-Actix |
| latency | `json-static` | 805,026.96 | 699,120.41 | +15.15% | 0.816 | 2.840 | -71.27% | 0 | faster-than-Actix |
| latency | `plaintext-handler` | 819,826.88 | 698,019.92 | +17.45% | 0.850 | 3.150 | -73.02% | 0 | faster-than-Actix |
| throughput | `json-serialize` | 806,171.49 | 718,018.99 | +12.28% | 0.746 | 3.080 | -75.78% | 0 | faster-than-Actix |
| throughput | `json-static` | 810,602.58 | 722,192.60 | +12.24% | 0.718 | 3.110 | -76.91% | 0 | faster-than-Actix |
| throughput | `plaintext-handler` | 813,090.51 | 729,007.30 | +11.53% | 0.818 | 3.350 | -75.58% | 0 | faster-than-Actix |

## Interpretation

- After the readiness guard fix, the clean cross-runtime CPU-bound handler-path
  evidence satisfies the public faster-than-Actix gate on every measured route
  and profile.
- This is not an io_uring, PGO, CPU-affinity, static-bypass, DB, TLS, HTTP/2,
  or production-network claim.
- The evidence supports public wording for high-performance, low-latency,
  high-throughput CPU-bound Wing handler routes, with the benchmark scope and
  environment stated explicitly.
