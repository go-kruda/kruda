# Read Buffer 2048 Evidence

Date: 2026-06-05
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

## Repeat Cross-Runtime Resource Pass

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/resource-readbuf2048-repeat-20260604T181840Z`

Command shape:

```bash
BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_DURATION=15s \
KRUDA_GO_TAGS=kruda_stdjson \
KRUDA_READ_BUF_SIZE=2048 \
KRUDA_PORT=3251 \
FIBER_PORT=3252 \
ACTIX_PORT=3253 \
./resource.sh plaintext-handler json-static json-serialize
```

| Profile | Route | Kruda RPS | Actix RPS | Kruda p99 | Actix p99 | Kruda RSS | Actix RSS | Errors |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 822,189.28 | 712,321.22 | 1.090 | 3.610 | 14.38 | 7.95 | 0 |
| latency | `json-static` | 800,095.67 | 700,721.19 | 1.060 | 2.940 | 15.50 | 7.95 | 0 |
| latency | `json-serialize` | 825,286.03 | 693,787.07 | 0.660 | 3.280 | 16.50 | 7.95 | 0 |
| throughput | `plaintext-handler` | 824,356.91 | 735,412.99 | 1.130 | 4.100 | 17.88 | 10.20 | 0 |
| throughput | `json-static` | 809,993.79 | 724,586.98 | 1.140 | 3.250 | 18.25 | 10.45 | 0 |
| throughput | `json-serialize` | 821,033.81 | 713,632.10 | 1.050 | 3.570 | 18.38 | 10.45 | 0 |

The repeat pass did not reproduce the earlier `latency/json-serialize` p99
spike. It again preserved zero socket errors and zero non-2xx responses.

Compared with the clean `4096` resource evidence, the repeat `2048` pass reduced
Kruda max RSS by roughly 12-16% across the six measured route/profile rows:

| Profile | Route | 4096 RSS MB | 2048 repeat RSS MB | RSS delta |
|---|---|---:|---:|---:|
| latency | `plaintext-handler` | 16.25 | 14.38 | -11.5% |
| latency | `json-static` | 17.75 | 15.50 | -12.7% |
| latency | `json-serialize` | 19.00 | 16.50 | -13.2% |
| throughput | `plaintext-handler` | 20.38 | 17.88 | -12.3% |
| throughput | `json-static` | 21.75 | 18.25 | -16.1% |
| throughput | `json-serialize` | 21.38 | 18.38 | -14.0% |

## Decision

Keep `KRUDA_READ_BUF_SIZE=4096` as the current published throughput/p99 evidence
profile. Promote `2048` only as an optional short-header memory profile
candidate: it repeatedly reduced RSS materially while preserving zero errors,
zero non-2xx responses, and the Actix throughput/p99 gate, but it also produced
some higher Kruda p99 rows than the clean `4096` profile. Do not change Kruda's
framework default from this evidence.
