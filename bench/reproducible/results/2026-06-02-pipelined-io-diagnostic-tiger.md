# Pipelined I/O Diagnostic Evidence

This note records a tiger Linux diagnostic run for the HTTP/1.1 pipelined
benchmark harness added on branch `perf/wing-pipelined-io-profile`.

This is diagnostic evidence for I/O architecture decisions. It is not a
replacement for the default `wrk --latency` fair handler-path benchmark claim.
HTTP/1.1 pipelining must be labeled explicitly when used in public wording.

## Environment

- Host: `dev-server`
- CPU: 13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs
- OS: Linux `6.8.0-117-generic` x86_64
- Commit: `d270381`
- Go: `go1.26.3 linux/amd64`
- Rust: `rustc 1.93.1`
- Cargo: `cargo 1.93.1`
- Result directory: `/home/tiger/kruda-wing-pipeline-io-d270381/bench/reproducible/results/pipeline-diagnostic-20260602T134636Z/`
- Raw logs: `/home/tiger/kruda-wing-pipeline-io-d270381/bench/reproducible/results/pipeline-diagnostic-20260602T134636Z/raw/`

## Command

```bash
DIR="$HOME/kruda-wing-pipeline-io-d270381"
cd "$DIR"

BENCH_FRAMEWORKS="kruda fiber actix" \
BENCH_ROUNDS=3 \
PIPELINE_DURATION=5s \
PIPELINE_WARMUP=2s \
PIPELINE_PROFILES="baseline-c128-d1:128:1 pipeline-c128-d8:128:8 pipeline-c256-d8:256:8" \
KRUDA_PORT=3921 \
FIBER_PORT=3922 \
ACTIX_PORT=3923 \
RESULT_DIR="$DIR/bench/reproducible/results/pipeline-diagnostic-$(date -u +%Y%m%dT%H%M%SZ)" \
./bench/reproducible/pipeline.sh plaintext-handler json-static json-serialize
```

## Kruda vs Actix Median Summary

All rows had zero socket errors and zero non-2xx responses.

| Profile | Route | Kruda median RPS | Actix median RPS | Kruda RPS delta | Kruda p99 ms | Actix p99 ms | Kruda p99 delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `baseline-c128-d1` | `json-serialize` | 652,351.23 | 620,710.37 | +5.10% | 1.172 | 2.087 | -43.84% |
| `baseline-c128-d1` | `json-static` | 666,219.02 | 617,313.77 | +7.92% | 1.191 | 2.088 | -42.96% |
| `baseline-c128-d1` | `plaintext-handler` | 669,746.20 | 618,462.71 | +8.29% | 1.282 | 2.144 | -40.21% |
| `pipeline-c128-d8` | `json-serialize` | 2,674,306.18 | 2,792,210.05 | -4.22% | 1.486 | 2.720 | -45.37% |
| `pipeline-c128-d8` | `json-static` | 3,046,756.83 | 2,846,599.09 | +7.03% | 1.679 | 2.616 | -35.82% |
| `pipeline-c128-d8` | `plaintext-handler` | 2,990,920.56 | 2,867,565.42 | +4.30% | 1.713 | 2.499 | -31.45% |
| `pipeline-c256-d8` | `json-serialize` | 2,760,753.71 | 2,847,653.69 | -3.05% | 3.036 | 3.645 | -16.71% |
| `pipeline-c256-d8` | `json-static` | 3,134,569.50 | 2,933,410.98 | +6.86% | 3.192 | 3.730 | -14.42% |
| `pipeline-c256-d8` | `plaintext-handler` | 3,133,652.83 | 2,910,802.83 | +7.66% | 3.279 | 3.958 | -17.16% |

## Interpretation

- The new harness is working across Kruda, Fiber, and Actix on Linux.
- Pipelining strongly increases absolute RPS for all runtimes, so it is a useful
  diagnostic profile for I/O batching behavior.
- Kruda beats Actix on plaintext and static JSON in these diagnostic profiles,
  but the margin is roughly 4-8%, not a 20% blanket win.
- Kruda p99 is lower than Actix in every measured row.
- Kruda trails Actix on JSON serialization under the pipelined profiles, so the
  next performance candidate should focus on the JSON serialization/response
  path before expecting a broad pipelined benchmark win.
