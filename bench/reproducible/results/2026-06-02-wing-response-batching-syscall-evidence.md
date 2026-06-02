# Wing Pipelined Response Batching Syscall Evidence

Date: 2026-06-02

Scope: intrusive `strace -c` diagnostic evidence for HTTP/1.1 pipelined
CPU-bound handler routes. This is not throughput claim evidence because strace
materially changes RPS and tail latency.

## Candidate Question

After PR #87, the next I/O architecture hypothesis was whether Wing still paid
roughly one write/send syscall per response under HTTP/1.1 pipelined request
batches, and whether an additional response batching runtime change was needed.

Code inspection showed that Wing's `Inline` dispatch already appends every
complete pipelined response in the connection read buffer into `conn.sendBuf`
before `directSend` writes it. This run measures that behavior on tiger.

## Harness

This branch adds `bench/reproducible/pipeline-syscall.sh`.

The harness:

- builds Kruda, Actix, and `pipeline-client` from their own directories
- starts one server process at a time and attaches `strace -f -c`
- drives the server with HTTP/1.1 pipelined requests
- writes `summary.csv`, `summary.md`, raw pipeline output, raw strace output,
  status files, server logs, and environment metadata
- reports `requests_per_write_send`, the main diagnostic ratio for response
  batching

For a depth-8 pipelined workload, a value near 8 means the server is writing
close to one response batch per client-side pipeline write. A value near 1
means it is still paying roughly one write/send syscall per response.

## Tiger Run

Result directory:
`/home/tiger/kruda-wing-response-batching-dac20c0/bench/reproducible/results/pipeline-syscall-full-20260602T155443Z`

Command shape:

```bash
RESULT_DIR="$DIR/bench/reproducible/results/pipeline-syscall-full-$(date -u +%Y%m%dT%H%M%SZ)" \
BENCH_FRAMEWORKS="kruda actix" \
PIPELINE_PROFILES="baseline-c128-d1:128:1 pipeline-c128-d8:128:8 pipeline-c256-d8:256:8" \
PIPELINE_DURATION=5s \
PIPELINE_WARMUP=2s \
PROFILE_SUDO=1 \
KRUDA_PORT=4310 \
ACTIX_PORT=4313 \
./pipeline-syscall.sh plaintext-handler json-static json-serialize
```

Environment summary:

- CPU: 13th Gen Intel Core i5-13500, 8 online CPUs
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.26.3 linux/amd64`
- Rust: `rustc 1.93.1`
- strace: `6.8`
- `ptrace_scope=1`
- `PROFILE_SUDO=1`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- Kruda tags: `kruda_stdjson`
- Routes: `plaintext-handler`, `json-static`, `json-serialize`
- All measured pipeline runs: zero socket errors and zero non-2xx responses
- All strace runs ended with status `130`, expected because the harness stops
  strace with `SIGINT` after the measured pipeline run

## Result

Kruda write/send syscall ratio:

| Profile | Route | Requests | write/send calls | requests/write-send |
|---|---|---:|---:|---:|
| `baseline-c128-d1` | `plaintext-handler` | 319,701 | 319,707 | 1.00 |
| `baseline-c128-d1` | `json-static` | 324,313 | 324,321 | 1.00 |
| `baseline-c128-d1` | `json-serialize` | 321,679 | 321,685 | 1.00 |
| `pipeline-c128-d8` | `plaintext-handler` | 2,265,056 | 283,177 | 8.00 |
| `pipeline-c128-d8` | `json-static` | 2,279,768 | 285,002 | 8.00 |
| `pipeline-c128-d8` | `json-serialize` | 2,213,720 | 276,763 | 8.00 |
| `pipeline-c256-d8` | `plaintext-handler` | 2,347,960 | 293,514 | 8.00 |
| `pipeline-c256-d8` | `json-static` | 2,341,400 | 292,699 | 8.00 |
| `pipeline-c256-d8` | `json-serialize` | 2,250,152 | 281,295 | 8.00 |

Actix also showed the same expected diagnostic shape:

| Profile | Route | requests/write-send |
|---|---|---:|
| `baseline-c128-d1` | `plaintext-handler` | 1.00 |
| `baseline-c128-d1` | `json-static` | 1.00 |
| `baseline-c128-d1` | `json-serialize` | 1.00 |
| `pipeline-c128-d8` | `plaintext-handler` | 8.00 |
| `pipeline-c128-d8` | `json-static` | 8.00 |
| `pipeline-c128-d8` | `json-serialize` | 8.00 |
| `pipeline-c256-d8` | `plaintext-handler` | 7.99 |
| `pipeline-c256-d8` | `json-static` | 8.00 |
| `pipeline-c256-d8` | `json-serialize` | 8.00 |

## Decision

Do not implement an additional Wing inline pipelined response batching runtime
change.

The diagnostic evidence shows that Wing already reaches the expected depth-8
coalescing ratio on the fair CPU-bound handler routes. A new runtime change
that only tries to batch inline pipelined responses would duplicate existing
behavior and risks disturbing response ordering, partial-write handling, or tail
latency without a credible syscall-count target.

The next I/O architecture work should target a different measured gap, such as
normal non-pipelined one-response-per-write workloads, Go runtime/futex pressure
under traced Wing runs, event-loop wake/re-arm behavior, or a carefully labeled
optional profile with an explicit latency tradeoff.
