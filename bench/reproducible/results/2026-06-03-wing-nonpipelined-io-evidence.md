# Wing Non-Pipelined I/O Direction Evidence

Date: 2026-06-03

Scope: focused tiger diagnostics on current `main` after PR #88. This note is
direction evidence for the next Wing performance track. It is not public
benchmark claim evidence because several runs are short diagnostics and the
strace rows are intrusive.

## Environment

- Host: tiger Linux dev server (`tiger-linux`)
- Source commit: `ef3fb05` (`bench: add pipelined syscall diagnostic harness`)
- Remote checkout:
  `/home/tiger/kruda-wing-nonpipelined-io-ef3fb05`
- CPU: 13th Gen Intel Core i5-13500, 8 online CPUs, KVM
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.26.3 linux/amd64`
- Rust: `rustc 1.93.1`
- wrk: `debian/4.1.0-4build2 [epoll]`
- Default runtime settings unless noted: `GOMAXPROCS=8`,
  `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`,
  `KRUDA_GO_TAGS=kruda_stdjson`

## Non-Pipelined Syscall Diagnostic

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/syscall-full-20260603T072534Z`

Command shape:

```bash
PROFILE_DURATION=10s \
WARMUP_DURATION=3s \
WRK_CONNECTIONS=256 \
PROFILE_SUDO=1 \
PERF_EVENTS=task-clock,context-switches,cpu-migrations,page-faults \
KRUDA_PORT=4520 \
ACTIX_PORT=4523 \
./syscall-profile.sh plaintext-handler json-static json-serialize
```

Comparable perf-stat wrk pass:

| Route | Kruda RPS | Kruda p99 | Kruda context switches | Actix RPS | Actix p99 | Actix context switches | RPS delta | Errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `plaintext-handler` | 853,895.69 | 1.09 ms | 576,629 | 747,634.68 | 3.26 ms | 849,978 | +14.21% | 0 | 0 |
| `json-static` | 831,518.59 | 1.85 ms | 382,773 | 709,909.55 | 2.53 ms | 1,175,676 | +17.13% | 0 | 0 |
| `json-serialize` | 829,329.53 | 1.32 ms | 347,831 | 742,536.00 | 4.15 ms | 414,931 | +11.69% | 0 | 0 |

Intrusive strace shape:

| Framework | Route | read/recv | write/send | epoll wait | epoll_ctl | futex |
|---|---|---:|---:|---:|---:|---:|
| Kruda | `plaintext-handler` | 686,652 | 683,866 | 440 | 1,278 | 14,166 |
| Kruda | `json-static` | 694,552 | 691,015 | 366 | 1,199 | 13,726 |
| Kruda | `json-serialize` | 691,318 | 688,313 | 349 | 1,246 | 13,873 |
| Actix | `plaintext-handler` | 906,231 | 906,295 | 28,765 | 467 | |
| Actix | `json-static` | 873,992 | 874,089 | 27,559 | 471 | |
| Actix | `json-serialize` | 902,015 | 902,100 | 28,525 | 496 | |

Interpretation:

- Kruda already wins the short comparable perf pass on all three fair
  CPU-bound handler routes, with materially better p99 latency.
- Kruda is still below a broad +20% RPS target on every route in this run.
- The strace pass shows Kruda near one read syscall and one write syscall per
  request, which is the expected shape for non-pipelined HTTP/1.1 keep-alive.
- The strace pass is too intrusive for RPS or latency conclusions. It produced
  Kruda socket errors on two routes, so use it only for syscall shape.
- Event-loop wait count is not the visible gap: Kruda epoll wait calls are far
  lower than Actix. This matches the earlier rejected idle-spin evidence.

## Default Sonic Control

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/default-sonic-json-smoke-20260603T072940Z`

Command shape:

```bash
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
BENCH_DURATION=10s \
KRUDA_GO_TAGS=default \
KRUDA_PORT=4530 \
ACTIX_PORT=4533 \
./bench.sh json-serialize
```

The Kruda server log reported `[kruda] JSON encoder: sonic`.

| Profile | Kruda median RPS | Kruda median p99 | Actix median RPS | Actix median p99 | RPS delta | Errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|
| latency | 801,031.42 | 1.22 ms | 700,169.01 | 2.84 ms | +14.41% | 0 | 0 |
| throughput | 807,496.44 | 1.05 ms | 718,690.58 | 2.86 ms | +12.36% | 0 | 0 |

Decision: do not treat the default Sonic build as the next broad +20% path.
It keeps Kruda ahead with better p99, but it does not materially change the
margin versus the stdjson diagnostic.

## Worker Count Sweep

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/worker-json-serialize-sweep-20260603T073418Z`

Command shape:

```bash
for W in 4 5 6 8; do
  RESULT_DIR="$BASE/w$W" \
  BENCH_FRAMEWORKS=kruda \
  BENCH_ROUNDS=3 \
  BENCH_DURATION=8s \
  KRUDA_WORKERS="$W" \
  ./bench.sh json-serialize
done
```

| Workers | Profile | Kruda median RPS | Kruda median p99 | Errors | Non-2xx |
|---:|---|---:|---:|---:|---:|
| 4 | latency | 797,156.64 | 0.486 ms | 0 | 0 |
| 4 | throughput | 822,529.27 | 0.950 ms | 0 | 0 |
| 5 | latency | 806,394.16 | 2.600 ms | 0 | 0 |
| 5 | throughput | 812,363.49 | 2.910 ms | 0 | 0 |
| 6 | latency | 794,319.64 | 3.130 ms | 0 | 0 |
| 6 | throughput | 801,190.79 | 3.500 ms | 0 | 0 |
| 8 | latency | 776,584.06 | 3.470 ms | 0 | 0 |
| 8 | throughput | 788,376.14 | 3.840 ms | 0 | 0 |

Decision: keep the benchmark worker profile at `KRUDA_WORKERS=4`. Higher worker
counts hurt tail latency heavily and do not provide a stable throughput win on
the weakest route.

## Kruda pprof

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/profile-json-serialize-20260603T073952Z`

Route: `json-serialize`, `-t4 -c256`, 15 second CPU profile.

Top observations:

| Symbol | Flat | Cumulative |
|---|---:|---:|
| `internal/runtime/syscall/linux.Syscall6` | 82.28% | 82.28% |
| `github.com/go-kruda/kruda.parseHTTPRequestInternal` | 0.88% | 2.85% |
| `encoding/json.(*Encoder).Encode` | 0.39% | 3.41% |
| `github.com/go-kruda/kruda.(*Ctx).JSON` | 0.14% | 4.08% |
| `github.com/go-kruda/kruda.(*epollEngine).waitWithTimeout` | 0.12% | 4.11% |

Interpretation:

- The current fair handler route is dominated by Linux socket syscalls.
- Parser, router, JSON serialization, response assembly, and epoll waiting are
  not large enough individually to explain a broad +20% fair-handler win.
- Further parser-only, JSON-only, worker-count, CPU-affinity, or idle-spin work
  is unlikely to be the best path unless new evidence changes this profile.

## io_uring Feasibility Probe

Probe commit: `e88e5d8` (`bench: add io_uring feasibility probe`)

Command:

```bash
cd bench/reproducible/uring-probe
GOWORK=off go run . -entries 64 -nops 10000
```

Result:

```text
io_uring_probe=ok
entries=64
cq_entries=128
nops=10000
elapsed_ms=1.532
nop_per_sec=6526942.24
```

Interpretation:

- The tiger kernel allows `io_uring_setup` (`io_uring_disabled=0`) and Go can
  submit/complete basic NOP operations through raw io_uring syscalls.
- This only proves kernel/API feasibility. It does not prove that a Wing
  network transport should use io_uring or that it will outperform the current
  epoll path.
- The next io_uring step must be a network-facing prototype with normal handler
  semantics and a kill switch, not a direct replacement of the default Wing
  transport.

## io_uring HTTP Ceiling Probe

Probe commit: `f892fbd` (`bench: simplify uring HTTP probe shutdown`)

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/uring-http-ceiling-fixed-20260603T171421Z`

Command shape:

```bash
cd bench/reproducible/uring-http-probe
GOWORK=off go build -o uring-http-probe .
./uring-http-probe -port 4567 -workers 4 -entries 4096
wrk --latency -t4 -c128 -d10s http://127.0.0.1:4567/plaintext-handler
wrk --latency -t4 -c256 -d10s http://127.0.0.1:4567/plaintext-handler
```

The probe is a fixed-response HTTP/1.1 keep-alive server with one io_uring per
SO_REUSEPORT worker. It does not run Kruda handlers, middleware, lifecycle
hooks, CORS/security headers, panic recovery, or route lookup.

| Profile | RPS | p50 | p90 | p99 | Max | Errors |
|---|---:|---:|---:|---:|---:|---:|
| `-t4 -c128 -d10s` | 817,853.51 | 74 us | 158 us | 309 us | 15.94 ms | 0 |
| `-t4 -c256 -d10s` | 859,183.53 | 137 us | 317 us | 1.03 ms | 15.93 ms | 0 |

Interpretation:

- The network-facing io_uring path is functional on tiger and handles wrk
  keep-alive traffic without socket errors in this short run.
- The ceiling is not clearly higher than current Wing fair-handler diagnostics.
  The c256 run is close to the best recent Wing plaintext diagnostic, while the
  c128 run is lower, despite bypassing the framework contract entirely.
- This does not justify a Wing runtime rewrite or default transport change.
- A future io_uring experiment would need a more specific advantage, such as
  measurable syscall batching, SQPOLL evidence, or a profile where current Wing
  is demonstrably blocked by epoll/read/write coordination.

## SQPOLL Control

Probe commit: `f6272d3` (`bench: wake SQPOLL uring HTTP submissions`)

Command shape:

```bash
cd bench/reproducible/uring-http-probe
GOWORK=off go build -o uring-http-probe .
./uring-http-probe -port 4570 -workers 4 -entries 4096 -sqpoll
wrk --latency -t4 -c128 -d5s http://127.0.0.1:4570/plaintext-handler
```

Result:

| Profile | RPS | p50 | p90 | p99 | Max | Errors |
|---|---:|---:|---:|---:|---:|---:|
| `-t4 -c128 -d5s` | 513,031.44 | 76 us | 156 us | 2.26 ms | 27.78 ms | 0 |

Interpretation:

- SQPOLL is functional only after waking submissions with
  `IORING_ENTER_SQ_WAKEUP`.
- On this host and probe shape, SQPOLL regresses throughput and p99 materially
  versus the default io_uring HTTP probe.
- Do not pursue SQPOLL as the next Wing optimization without new evidence.

## io_uring HTTP v2 Controls

Probe commits:

- `b838614` (`bench: add uring HTTP ceiling controls`)
- `b99f8b3` (`bench: start uring accepts on ring owner`)
- `05226ff` (`bench: lock uring single issuer workers`)

Result directories:

- `/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/uring-http-v2-20260604T141539Z`
- `/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/uring-http-v2-c256-sweep-20260604T141810Z`
- `/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/uring-http-setup-sweep-20260604T142207Z`
- `/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/uring-http-taskrun-sweep-20260604T142259Z`

The v2 controls tested multishot accept, mid-drain submission flushing, and
io_uring setup flags. These are still fixed-response ceiling probes, not Kruda
runtime behavior.

Primary 10s matrix:

| Case | Profile | RPS | p50 | p90 | p99 | Max | Errors |
|---|---|---:|---:|---:|---:|---:|---:|
| control | `-t4 -c128 -d10s` | 847,676.90 | 77 us | 170 us | 663 us | 12.06 ms | 0 |
| control | `-t4 -c256 -d10s` | 835,659.87 | 141 us | 293 us | 634 us | 16.08 ms | 0 |
| multishot accept | `-t4 -c128 -d10s` | 837,854.38 | 79 us | 175 us | 357 us | 19.22 ms | 0 |
| multishot accept | `-t4 -c256 -d10s` | 805,871.99 | 145 us | 306 us | 0.96 ms | 9.96 ms | 0 |
| submit batch 16 | `-t4 -c128 -d10s` | 762,996.57 | 84 us | 180 us | 361 us | 15.24 ms | 0 |
| submit batch 16 | `-t4 -c256 -d10s` | 860,146.79 | 150 us | 330 us | 527 us | 12.36 ms | 0 |
| submit batch 32 | `-t4 -c128 -d10s` | 803,455.05 | 83 us | 202 us | 825 us | 13.96 ms | 0 |
| submit batch 32 | `-t4 -c256 -d10s` | 838,917.68 | 141 us | 299 us | 602 us | 15.42 ms | 0 |
| multishot accept + submit batch 32 | `-t4 -c128 -d10s` | 801,924.85 | 81 us | 177 us | 369 us | 10.28 ms | 0 |
| multishot accept + submit batch 32 | `-t4 -c256 -d10s` | 782,112.13 | 150 us | 318 us | 0.94 ms | 10.15 ms | 0 |

Short c256 sweep:

| Case | RPS | p50 | p90 | p99 | Max | Errors |
|---|---:|---:|---:|---:|---:|---:|
| control A | 859,209.75 | 140 us | 305 us | 559 us | 10.47 ms | 0 |
| control B | 833,457.83 | 140 us | 304 us | 682 us | 13.20 ms | 0 |
| submit batch 4 | 686,130.20 | 167 us | 343 us | 1.20 ms | 10.68 ms | 0 |
| submit batch 8 | 777,856.93 | 155 us | 324 us | 772 us | 18.81 ms | 0 |
| submit batch 16 | 836,152.19 | 140 us | 285 us | 476 us | 9.65 ms | 0 |
| submit batch 24 | 813,933.77 | 147 us | 318 us | 585 us | 5.66 ms | 0 |
| submit batch 64 | 847,051.93 | 139 us | 297 us | 526 us | 7.99 ms | 0 |

Task-run setup sweep:

| Case | RPS | p50 | p90 | p99 | Max | Errors |
|---|---:|---:|---:|---:|---:|---:|
| control | 888,981.82 | 138 us | 323 us | 608 us | 10.33 ms | 0 |
| coop taskrun | 861,745.30 | 136 us | 298 us | 517 us | 14.02 ms | 0 |
| coop taskrun + submit batch 16 | 873,489.63 | 131 us | 265 us | 498 us | 10.15 ms | 0 |

Rejected or incomplete controls:

- `-single-issuer` still returned `wait cqe: file exists` with multiple
  workers even after starting accepts from the loop and locking the OS thread.
  That suggests the ring creation task also has to be the issuer. Testing it
  correctly would require moving ring setup into the event-loop owner.
- `-defer-taskrun` returned `io_uring_setup: invalid argument` on this host
  without the setup constraints it expects.

Interpretation:

- Multishot accept does not produce a throughput win for this keep-alive
  workload because accepting new sockets is not the dominant cost.
- Mid-drain submission flushing is not a stable win. `submit-batch 16` improved
  one c256 run but regressed c128 and landed inside control-run variance in the
  short sweep.
- `IORING_SETUP_COOP_TASKRUN` did not beat the same-run control.
- These v2 controls still do not establish an io_uring ceiling high enough to
  justify a Wing runtime rewrite. A future io_uring attempt needs a stronger
  hypothesis than setup flags or accept resubmission reduction.

## Direction

The highest-probability path to a larger fair-handler win is not another
parser/router/JSON tweak. The remaining hard path is an opt-in I/O architecture
experiment that changes the request/response syscall shape or reduces syscall
coordination overhead while preserving the default framework contract.

Candidates should be isolated as experimental Wing profiles and must not change
default behavior:

- A Linux-only io_uring/proactor prototype that preserves the existing HTTP
  parser, middleware/lifecycle contract, timeout checks, safe-copy semantics,
  and response ordering.
- A narrow write/read syscall-shape prototype with a measurable target before
  runtime changes are kept.

Success gate for the next prototype:

- At least +5% route-level RPS over current Wing on the same host and command.
- p99 no worse than current Wing.
- Zero socket errors and zero non-2xx responses in non-strace wrk evidence.
- No correctness/security regression in HTTP parser, timeout, lifecycle,
  middleware, CORS/security-header, and pipelining tests.

If the prototype cannot reduce syscall shape or coordination cost, stop and do
not merge runtime code.
