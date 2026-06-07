# Epoll Conn Pointer Table Rejection Evidence

Date: 2026-06-07 UTC
Host: tiger Linux dev server (`tiger-linux`)
Base commit: `f49f067` (`bench: record main DB follow-up evidence`)
Scope: Linux Wing epoll event hot path candidate discovery. No runtime change is
accepted from this pass.

## Candidate

The candidate replaced Linux Wing's `map[int32]unsafe.Pointer` connection
pointer lookup with an fd-indexed `[]unsafe.Pointer` table:

- `RegisterConn` grew the table and stored `connPtrs[fd] = *conn`.
- `SubmitClose` and `Detach` cleared the fd slot.
- `waitWithTimeout` looked up `connPtrs[fd]` by slice index instead of a hash
  map lookup.

This was a deliberately narrow experiment. It did not change epoll event data,
the parser, request lifecycle, response ordering, direct send behavior,
timeouts, or dispatch semantics.

## Candidate Verification

Temporary candidate worktree:
`/home/tiger/kruda-connptr-candidate-20260607T154643`

Targeted Linux unit test for table grow/clear:

```bash
cd /home/tiger/kruda-connptr-candidate-20260607T154643
/usr/bin/env GOTOOLCHAIN=go1.25.10 GOWORK=off \
  GOCACHE=/tmp/kruda-go-build-cache-connptr \
  GOMODCACHE=/tmp/kruda-go-mod-cache-connptr \
  go test -count=1 -run TestEpollEngineConnPtrTableGrowAndClear -tags kruda_stdjson
```

Result: `PASS`.

Full Linux suite with CI guard thresholds:

```bash
cd /home/tiger/kruda-connptr-candidate-20260607T154643
/usr/bin/env CI=1 GOTOOLCHAIN=go1.25.10 GOWORK=off \
  GOCACHE=/tmp/kruda-go-build-cache-connptr \
  GOMODCACHE=/tmp/kruda-go-mod-cache-connptr \
  go test -count=1 -tags kruda_stdjson ./...
```

Result: all packages passed. A non-CI full-suite run hit the noisy local
`TestPlaintextPerformanceGuard` floor once at `47,456 req/s` versus the
non-CI `50,000 req/s` threshold, then passed under the CI threshold used for
remote validation.

## Benchmark Command

Baseline worktree:
`/home/tiger/kruda-connptr-baseline-20260607T155020`

Candidate worktree:
`/home/tiger/kruda-connptr-candidate-20260607T154643`

Baseline:

```bash
cd /home/tiger/kruda-connptr-baseline-20260607T155020/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/connptr-baseline-20260607T155020 \
BENCH_FRAMEWORKS=kruda \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
BENCH_WARMUP=2s \
KRUDA_PORT=3311 \
./bench.sh plaintext-handler json-static json-serialize
```

Candidate:

```bash
cd /home/tiger/kruda-connptr-candidate-20260607T154643/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/connptr-candidate-20260607T155020 \
BENCH_FRAMEWORKS=kruda \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
BENCH_WARMUP=2s \
KRUDA_PORT=3321 \
./bench.sh plaintext-handler json-static json-serialize
```

## Benchmark Environment

Concrete values from both result directories:

```text
base_git_commit=f49f067
candidate_git_commit=f49f067
candidate_git_tracked_dirty=1
bench_enable_db=0
bench_enable_pprof=0
bench_kruda_cpu_dispatch=inline
kruda_go_tags=kruda_stdjson
gomaxprocs=8
kruda_workers=4
kruda_read_buf_size=default
bench_rounds=3
bench_duration=8s
frameworks=kruda
routes=plaintext-handler json-static json-serialize
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-117-generic x86_64
Go=go1.25.10 linux/amd64
wrk=debian/4.1.0-4build2
```

## Median Results

All measured rows had zero socket errors and zero non-2xx responses.

| Profile | Route | Main median RPS | Candidate median RPS | Candidate RPS delta | Main p99 ms | Candidate p99 ms | Candidate p99 delta |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `json-serialize` | 824,147.74 | 811,545.70 | -1.53% | 0.488 | 0.569 | +16.60% |
| latency | `json-static` | 825,451.99 | 827,004.14 | +0.19% | 0.800 | 0.565 | -29.38% |
| latency | `plaintext-handler` | 796,401.07 | 820,580.83 | +3.04% | 0.572 | 0.647 | +13.11% |
| throughput | `json-serialize` | 832,740.45 | 835,855.80 | +0.37% | 0.726 | 0.691 | -4.82% |
| throughput | `json-static` | 830,568.39 | 818,201.10 | -1.49% | 0.773 | 0.960 | +24.19% |
| throughput | `plaintext-handler` | 828,426.08 | 815,115.11 | -1.61% | 0.771 | 0.860 | +11.54% |

## Decision

Reject the fd-indexed conn pointer table candidate.

The candidate passed functional Linux tests, but it did not produce a balanced
CPU-bound improvement. RPS moved between `-1.61%` and `+3.04%`, and p99 latency
regressed in four of six median rows. The only meaningful RPS gain was
`plaintext-handler` latency at `+3.04%`, paired with a `+13.11%` p99 regression.

Do not replace the current Linux `connPtrs` map with an fd-indexed slice as a
performance change unless future evidence first shows a larger, balanced
same-runner win. This result also does not change the prior broad +20% decision:
the current CPU-bound gap is still not explained by ordinary epoll event
pointer lookup overhead.
