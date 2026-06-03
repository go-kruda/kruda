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
