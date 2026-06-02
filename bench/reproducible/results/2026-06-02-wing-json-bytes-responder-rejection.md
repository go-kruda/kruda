# Wing JSON Bytes Responder Rejection Evidence

Date: 2026-06-02

Scope: local microbenchmark evidence plus a fresh tiger CPU profile check for a
narrow Wing handler-path JSON bytes candidate. This is not cross-runtime Actix
evidence and not a public performance claim.

## Candidate

The candidate moved the Wing `JSONResponder` check in
`Ctx.SendStaticWithTypeBytes` before the fasthttp static-body fast path when the
content type is Kruda's JSON content type.

Rationale:

- `SendStaticJSON` already benefits from trying Wing's `JSONResponder` before a
  guaranteed fasthttp miss on Wing.
- `SendStaticWithTypeBytes(jsonContentType, data)` reaches the same Wing JSON
  response builder but still checked the fasthttp static-body path first.

The runtime change was reverted because it did not produce a statistically
meaningful improvement.

## Added Benchmark Coverage

This PR keeps a non-public microbenchmark for the previously unmeasured static
JSON bytes helper:

- `BenchmarkCPUHandlerJSONStaticBytesFeather`

The benchmark exercises a normal Wing handler route using:

```go
c.SendStaticWithTypeBytes(jsonContentType, benchStaticJSONBody)
```

## Local Command

Baseline worktree: `origin/main` at
`1395a66a32c3e7ce71cb0dbe34b694e040d72302`, with only the new benchmark added
for measurement.

Candidate branch: `perf/wing-json-bytes-responder-first`, before the runtime
change was reverted.

```bash
/usr/bin/env PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin \
  GOTOOLCHAIN=local \
  GOWORK=off \
  GOCACHE=/private/tmp/kruda-go-build-cache-phase8 \
  GOMODCACHE=/private/tmp/kruda-go-mod-cache-phase8 \
  go test -run '^$' -tags kruda_stdjson \
    -bench 'Benchmark(CPUHandlerJSONStatic(Bytes)?Feather)$' \
    -benchmem -count=10 ./
```

Environment:

- Host: local Apple M3
- OS/arch: darwin/arm64
- Go: local toolchain reported `go1.26.1`
- Build tags: `kruda_stdjson`

## Benchstat

```text
name                                  main sec/op    candidate sec/op    delta
CPUHandlerJSONStaticFeather-8         219.7n +/- 6%  220.4n +/- 1%      ~ (p=0.468 n=10)
CPUHandlerJSONStaticBytesFeather-8    221.6n +/- 1%  221.4n +/- 2%      ~ (p=0.684 n=10)
geomean                               220.6n         220.9n             +0.15%

name                                  main B/op      candidate B/op      delta
CPUHandlerJSONStaticFeather-8         160.0 +/- 0%   160.0 +/- 0%       no change
CPUHandlerJSONStaticBytesFeather-8    160.0 +/- 0%   160.0 +/- 0%       no change

name                                  main alloc/op  candidate alloc/op  delta
CPUHandlerJSONStaticFeather-8         1.000 +/- 0%   1.000 +/- 0%       no change
CPUHandlerJSONStaticBytesFeather-8    1.000 +/- 0%   1.000 +/- 0%       no change
```

## Tiger Profile Check

Worktree:
`/home/tiger/kruda-main-1395a66-20260602` at
`1395a66a32c3e7ce71cb0dbe34b694e040d72302`.

Result directory:
`/home/tiger/kruda-main-1395a66-20260602/bench/reproducible/results/profile-20260602T092329Z`.

Command:

```bash
PORT=3310 PPROF_PORT=6310 BENCH_FRAMEWORKS=kruda BENCH_ROUNDS=1 BENCH_DURATION=10 \
  ./profile-kruda.sh plaintext-handler json-static json-serialize
```

Environment summary:

- CPU: 13th Gen Intel Core i5-13500, 8 online CPUs
- OS: Linux `6.8.0-117-generic`
- Go: `go1.26.3 linux/amd64`
- wrk: Debian `4.1.0-4build2`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- `KRUDA_READ_BUF_SIZE=4096`
- Build tags: `kruda_stdjson bench_pprof`
- wrk shape: 4 threads, 256 connections

Fresh profile summary:

| Route | RPS during profile | p99 | Dominant flat CPU |
|---|---:|---:|---:|
| `plaintext-handler` | 853,329.87 | 0.90 ms | `internal/runtime/syscall/linux.Syscall6` 83.67% |
| `json-static` | 818,812.95 | 1.16 ms | `internal/runtime/syscall/linux.Syscall6` 86.68% |
| `json-serialize` | 792,773.15 | 1.01 ms | `internal/runtime/syscall/linux.Syscall6` 83.17% |

The wrk outputs did not report socket errors or non-2xx responses. This is
diagnostic profile evidence only because pprof was enabled and only Kruda was
measured.

## Decision

Reject the Wing-first `SendStaticWithTypeBytes` JSON responder reorder.

The candidate did not improve the directly targeted microbenchmark and did not
change allocation behavior. The fresh tiger profile still shows the CPU-bound
routes are dominated by socket/syscall cost, so further fair-handler CPU-bound
runtime work should start from a new I/O architecture hypothesis or a narrowed
JSON-serialization-only goal.
