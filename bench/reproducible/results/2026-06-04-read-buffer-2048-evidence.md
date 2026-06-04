# Read Buffer 2048 Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`
Commit under test: `7c54694`

## Scope

This evidence checks whether a smaller Wing read buffer is a useful
workload-specific memory profile for short-header CPU-bound routes. It does not
change Kruda's framework default. Smaller read buffers can reject requests whose
request line and headers do not fit the configured buffer, so this remains an
explicit benchmark/runtime tuning knob, not a general default.

## Directional Sweep

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/readbuf-sweep-20260604T164728Z`

Command shape:

```bash
BENCH_FRAMEWORKS=kruda \
BENCH_DURATION=8s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_READ_BUF_SIZE=<1024|2048|4096> \
./resource.sh plaintext-handler json-static json-serialize
```

Directional summary:

- `1024` produced the lowest RSS, but p99 was less stable on `json-static` and
  throughput `plaintext-handler`.
- `2048` looked like the most balanced candidate in the sweep: lower RSS than
  `4096`, competitive RPS/core, and no errors.
- `4096` remained strong on some throughput rows, especially
  `json-serialize`, but used more RSS.

Selected rows:

| Read buffer | Profile | Route | RPS | p99 ms | Max RSS MB | RPS/core | Errors |
|---:|---|---|---:|---:|---:|---:|---:|
| 1024 | throughput | `json-serialize` | 811,117.83 | 0.701 | 18.00 | 210,407 | 0 |
| 2048 | throughput | `json-serialize` | 818,073.08 | 0.742 | 18.25 | 214,503 | 0 |
| 4096 | throughput | `json-serialize` | 834,088.56 | 0.761 | 20.38 | 217,636 | 0 |

## Cross-Runtime Resource Pass

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/resource-readbuf2048-20260604T165355Z`

Command shape:

```bash
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_READ_BUF_SIZE=2048 \
KRUDA_PORT=3241 \
FIBER_PORT=3242 \
ACTIX_PORT=3243 \
./resource.sh plaintext-handler json-static json-serialize
```

| Profile | Route | Kruda RPS | Actix RPS | Kruda p99 | Actix p99 | Kruda RSS | Actix RSS | Errors |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 823,294.73 | 675,087.29 | 0.870 | 3.270 | 14.00 | 7.82 | 0 |
| latency | `json-static` | 835,733.44 | 632,128.83 | 0.840 | 3.980 | 15.50 | 7.82 | 0 |
| latency | `json-serialize` | 802,325.97 | 535,287.60 | 2.410 | 4.550 | 16.50 | 7.95 | 0 |
| throughput | `plaintext-handler` | 833,025.07 | 720,352.52 | 0.816 | 3.430 | 17.62 | 10.32 | 0 |
| throughput | `json-static` | 833,497.52 | 733,921.77 | 0.837 | 3.340 | 18.12 | 10.32 | 0 |
| throughput | `json-serialize` | 820,297.69 | 723,568.66 | 1.090 | 3.280 | 18.75 | 10.45 | 0 |

Compared with the clean `4096` resource evidence from
`2026-06-04-clean-resource-evidence.md`, the `2048` profile reduced Kruda max
RSS by roughly 12-17% across the six measured route/profile rows. It still uses
more RSS than Actix.

## Decision

Keep `KRUDA_READ_BUF_SIZE=4096` as the current published benchmark profile until
there is repeated evidence. `2048` is a credible follow-up candidate for an
optional short-header memory profile because it lowered RSS materially while
preserving zero errors and competitive throughput. Repeat the cross-runtime run
before updating docs or public benchmark recommendations, because the
`latency/json-serialize` row had a p99 spike in this single resource pass.
