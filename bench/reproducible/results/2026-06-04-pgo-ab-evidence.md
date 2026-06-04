# Kruda Reproducible PGO A/B Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`

## Readiness Guard Fix

The first PGO A/B attempt at
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/pgo-ab-20260604T143751Z`
is invalid evidence. Port `3000` was already occupied on tiger, the Kruda
benchmark binary logged `bind: address already in use`, and the old readiness
check accepted a different service that already answered
`/plaintext-handler`.

Commit `e7fda49` (`bench: reject occupied readiness ports`) fixes this class of
harness error by:

- rejecting readiness URLs that already respond before the benchmark server is
  started,
- re-checking the benchmark server PID after readiness succeeds, and
- applying the same guard to reproducible bench, resource, pipeline, syscall,
  and pprof helpers.

Validation:

```bash
bash -n bench/reproducible/bench.sh
bash -n bench/reproducible/pipeline.sh
bash -n bench/reproducible/resource.sh
bash -n bench/reproducible/pipeline-syscall.sh
bash -n bench/reproducible/syscall-profile.sh
bash -n bench/reproducible/profile-kruda.sh
```

All syntax checks passed locally.

## PGO A/B

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/pgo-ab-20260604T145130Z`

Method:

- trained a temporary `default.pgo` from mixed load over
  `/plaintext-handler`, `/json-static`, and `/json-serialize`,
- ran Kruda-only reproducible `bench.sh` control without `default.pgo`,
- ran the same benchmark after building with `default.pgo`,
- used `KRUDA_PORT=3180` and `PPROF_PORT=6180` to avoid tiger port conflicts,
- removed `bench/reproducible/kruda/default.pgo` after the run.

Summary:

| Route | Profile | Control median RPS | PGO median RPS | Delta | Control median p99 ms | PGO median p99 ms | Errors |
|---|---|---:|---:|---:|---:|---:|---:|
| `json-serialize` | latency | 814,204.86 | 815,637.28 | +0.18% | 0.485 | 0.740 | 0 |
| `json-serialize` | throughput | 817,410.87 | 816,903.12 | -0.06% | 0.767 | 0.910 | 0 |
| `json-static` | latency | 802,589.56 | 809,584.99 | +0.87% | 0.806 | 0.847 | 0 |
| `json-static` | throughput | 808,344.71 | 825,898.77 | +2.17% | 0.756 | 0.842 | 0 |
| `plaintext-handler` | latency | 816,585.96 | 784,645.10 | -3.91% | 0.750 | 0.870 | 0 |
| `plaintext-handler` | throughput | 818,785.17 | 817,200.99 | -0.19% | 0.890 | 0.880 | 0 |

Interpretation:

- PGO did not produce a broad CPU-bound handler-path win on this branch and
  host.
- The best throughput gain was `json-static` throughput at +2.17%, but the
  same profile had worse p99 and plaintext latency regressed by -3.91%.
- PGO should not be the next Actix-win path unless a narrower route-specific
  profile produces repeatable p99-neutral gains.
