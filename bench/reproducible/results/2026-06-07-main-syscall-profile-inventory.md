# Main Syscall/Profile Inventory After Ratio Columns

Date: 2026-06-07
Host: tiger Linux dev server (`tiger-linux`)
Commit: `4473724` (`bench: add non-pipelined syscall ratios`)
Scope: Evidence-only follow-up for the CPU-bound non-pipelined fair-handler
path. No runtime candidate is accepted by this pass.

## Why This Was Run

The non-pipelined syscall profile now reports request counts,
requests-per-syscall ratios, and per-1k-request syscall ratios. This run uses
those columns on current `main` before taking another runtime change, so the
next candidate must be justified by fresh syscall shape and pprof evidence.

## Syscall-Ratio Inventory Command

```bash
cd /home/tiger/kruda-main-syscall-ratio-inventory-20260607T135847Z/bench/reproducible
export PATH="$HOME/.cargo/bin:/usr/local/go/bin:$PATH"
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/main-syscall-ratio-inventory-20260607T135906Z \
PROFILE_DURATION=8s \
WARMUP_DURATION=2s \
WRK_CONNECTIONS=256 \
PROFILE_SUDO=1 \
PERF_EVENTS=task-clock,context-switches,cpu-migrations,page-faults \
KRUDA_PORT=3271 \
ACTIX_PORT=3273 \
./syscall-profile.sh plaintext-handler json-static json-serialize
```

## Syscall-Ratio Environment

- Result directory:
  `/home/tiger/kruda-main-syscall-ratio-inventory-20260607T135847Z/bench/reproducible/results/main-syscall-ratio-inventory-20260607T135906Z`
- Git commit: `4473724`
- Git tracked dirty: `0`
- CPU: `13th Gen Intel(R) Core(TM) i5-13500`, 8 logical CPUs
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.25.10 linux/amd64`
- Rust: `rustc 1.93.1`
- Cargo: `cargo 1.93.1`
- wrk: `debian/4.1.0-4build2 [epoll]`
- perf: `6.8.12`
- strace: `6.8`
- `profile_duration=8s`
- `warmup_duration=2s`
- `wrk_threads=4`
- `wrk_connections=256`
- `profile_sudo=1`
- `perf_event_paranoid=4`
- `ptrace_scope=1`
- `gomaxprocs=8`
- `kruda_workers=4`
- `actix_workers=default`
- `kruda_read_buf_size=4096`
- `kruda_go_tags=kruda_stdjson`
- `routes=plaintext-handler json-static json-serialize`

## Comparable Perf Pass

These rows are the normal wrk + perf pass from the syscall harness. They are
short diagnostic measurements, not publication benchmark medians.

All rows had zero socket errors and zero non-2xx responses.

| Route | Kruda RPS | Kruda p99 | Actix RPS | Actix p99 | Kruda delta |
|---|---:|---:|---:|---:|---:|
| `plaintext-handler` | 849,719.93 | 0.98ms | 756,725.03 | 3.39ms | +12.29% |
| `json-static` | 833,086.33 | 795.00us | 750,793.13 | 3.31ms | +10.96% |
| `json-serialize` | 814,744.06 | 1.91ms | 718,027.98 | 2.77ms | +13.47% |

## Syscall Shape

Strace rows are intrusive diagnostic evidence. Use syscall counts and ratios,
not straced RPS or latency, for conclusions. `strace_status=130` is expected
because the harness stops strace with SIGINT after wrk completes.

| Framework | Route | Requests | read/recv | write/send | req/read | req/write | epoll wait/1k req | epoll_ctl/1k req | futex/1k req | Socket errors |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| actix | `plaintext-handler` | 733,232 | 733,679 | 733,727 | 1.00 | 1.00 | 31.62 | 0.61 |  | 0 |
| actix | `json-static` | 728,141 | 728,649 | 728,642 | 1.00 | 1.00 | 31.68 | 0.71 |  | 0 |
| actix | `json-serialize` | 719,462 | 719,966 | 719,964 | 1.00 | 1.00 | 31.61 | 0.71 |  | 0 |
| kruda | `plaintext-handler` | 549,021 | 551,884 | 549,274 | 0.99 | 1.00 | 0.83 | 1.34 | 21.63 | 191 |
| kruda | `json-static` | 546,891 | 550,552 | 547,143 | 0.99 | 1.00 | 0.74 | 1.37 | 20.78 | 0 |
| kruda | `json-serialize` | 544,880 | 547,872 | 545,128 | 0.99 | 1.00 | 0.87 | 1.41 | 21.75 | 0 |

## Kruda Pprof Command

```bash
cd /home/tiger/kruda-main-syscall-ratio-inventory-20260607T135847Z/bench/reproducible
export PATH="$HOME/.cargo/bin:/usr/local/go/bin:$PATH"
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/main-profile-syscall-ratio-followup-20260607T140251Z \
BENCH_DURATION=10 \
WARMUP_DURATION=2 \
THREADS=4 \
CONNECTIONS=256 \
PORT=3281 \
PPROF_PORT=6081 \
./profile-kruda.sh plaintext-handler json-static json-serialize
```

Environment:

- Git commit: `4473724`
- Git tracked dirty: `0`
- CPU/OS/toolchain: same tiger host, Linux `6.8.0-117-generic`,
  `go1.25.10 linux/amd64`
- `kruda_go_tags=kruda_stdjson bench_pprof`
- `gomaxprocs=8`
- `kruda_workers=4`
- `kruda_read_buf_size=4096`
- `profile_seconds=10`
- `threads=4`
- `connections=256`

## Pprof Summary

| Route | `Syscall6` flat | `runtime.futex` flat | `time.runtimeNow` flat | Parser cumulative | Route/response cumulative | `directSend` cumulative | `epollWait` cumulative |
|---|---:|---:|---:|---:|---:|---:|---:|
| `plaintext-handler` | 84.08% | 3.60% | 1.68% | 2.61% | 1.24% `serveRoute` | 69.89% | 3.73% |
| `json-static` | 83.44% | 4.06% | 1.71% | 2.53% | 1.23% `serveRoute` | 68.25% | 3.96% |
| `json-serialize` | 84.98% | 1.46% | 1.46% | 2.47% | 3.29% `Ctx.JSON` | 70.26% | 3.52% |

The pprof run's Kruda wrk sidecar stayed in the same throughput band:

| Route | Requests/sec | p99 | Socket errors | Non-2xx |
|---|---:|---:|---:|---:|
| `plaintext-handler` | 856,284.68 | 1.15ms | 0 | 0 |
| `json-static` | 823,479.62 | 748.00us | 0 | 0 |
| `json-serialize` | 795,823.66 | 1.41ms | 0 | 0 |

## Local Microbenchmark Check

```bash
/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
GOWORK=off \
GOCACHE=/private/tmp/kruda-go-build-cache \
GOMODCACHE=/private/tmp/kruda-go-mod-cache \
go test -run '^$' -bench 'BenchmarkCPU(Response|Handler)' -benchmem -tags kruda_stdjson
```

Local output on Apple M3:

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkCPUResponseJSON-8` | 39.87 | 160 | 1 |
| `BenchmarkCPUResponsePlaintext-8` | 22.82 | 0 | 0 |
| `BenchmarkCPUHandlerPlaintextFeather-8` | 194.7 | 8 | 1 |
| `BenchmarkCPUHandlerJSONStaticFeather-8` | 207.1 | 160 | 1 |
| `BenchmarkCPUHandlerJSONSerializeFeather-8` | 274.4 | 160 | 1 |

This microbenchmark is useful for checking user-space path size, but it does
not explain the non-pipelined tiger gap because the remote pprof is dominated
by socket syscalls.

## Decision

Do not take a runtime change from this pass.

- Non-pipelined `req/write` is already approximately 1.00 for both Kruda and
  Actix. A broad fair-handler +20% path cannot come from ordinary response
  batching unless the workload changes to pipelining or an explicitly labeled
  workload-specific profile.
- Kruda's epoll wait rate is already far lower than Actix's in this diagnostic
  run. Epoll-wait-only and idle-spin directions remain unsupported.
- `runtime.futex` is visible on plaintext/static, but only about 3-4% flat CPU.
  It is not large enough to support a broad +20% claim by itself, and previous
  `GOMAXPROCS`, worker-count, and CPU-affinity sweeps did not produce a broad
  balanced win.
- Parser, JSON serialization, and response assembly are too small in pprof to
  justify another broad runtime candidate without new evidence.

The next credible +20% investigation should change the workload boundary or
target syscall count directly, for example:

- workload-specific HTTP/1.1 pipelined diagnostics, which already show a
  different request-per-write shape;
- DB/async workloads where Wing dispatch feathers change actual waiting work;
- a narrowly scoped response-emission prototype only if it first demonstrates
  fewer write/read syscalls per completed request without bypassing normal
  handler, middleware, lifecycle, header, and safety behavior.
