# v1.2.6 Pipelined I/O Diagnostic Evidence

Date: 2026-06-07
Host: tiger
Commit under test: `b57fbeee13ada58148b218500ed87696362b373e`
Scope: HTTP/1.1 pipelined diagnostic workload only. This is not a default
fair-handler benchmark claim and not a broad real-world API throughput claim.

## Command

```bash
cd /home/tiger/kruda-v126-pipeline-20260607/bench/reproducible
PATH="$HOME/.cargo/bin:$PATH" \
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR="$PWD/results/pipeline-v126-20260607T0208Z" \
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
PIPELINE_DURATION=5s \
PIPELINE_WARMUP=2s \
PIPELINE_PROFILES="baseline-c128-d1:128:1 pipeline-c128-d8:128:8 pipeline-c256-d8:256:8" \
KRUDA_PORT=3281 \
ACTIX_PORT=3283 \
./pipeline.sh plaintext-handler json-static json-serialize
```

## Environment

- Result directory:
  `/home/tiger/kruda-v126-pipeline-20260607/bench/reproducible/results/pipeline-v126-20260607T0208Z`
- Workload: `http1_pipelined_diagnostic`
- `bench_enable_db=0`
- `bench_enable_pprof=0`
- `kruda_go_tags=kruda_stdjson`
- `gomaxprocs=8`
- `kruda_workers=4`
- `kruda_read_buf_size=default`
- `bench_rounds=3`
- `pipeline_duration=5s`
- `pipeline_warmup=2s`
- `pipeline_timeout=5s`
- `pipeline_profiles=baseline-c128-d1:128:1 pipeline-c128-d8:128:8 pipeline-c256-d8:256:8`
- `frameworks=kruda actix`
- `routes=plaintext-handler json-static json-serialize`
- CPU: `13th Gen Intel(R) Core(TM) i5-13500`, 8 logical CPUs
- Toolchain: `go version go1.25.10 linux/amd64`

## Median Results

All measured rows had zero socket errors and zero non-2xx responses.

| Profile | Route | Kruda median RPS | Actix median RPS | Kruda RPS delta | Kruda p99 ms | Actix p99 ms | Kruda p99 delta |
|---|---|---:|---:|---:|---:|---:|---:|
| `baseline-c128-d1` | `plaintext-handler` | 665024.32 | 587007.57 | +13.29% | 1.355 | 2.254 | -39.88% |
| `baseline-c128-d1` | `json-static` | 638794.55 | 581794.56 | +9.80% | 1.280 | 2.279 | -43.84% |
| `baseline-c128-d1` | `json-serialize` | 621279.05 | 575838.51 | +7.89% | 1.340 | 2.239 | -40.15% |
| `pipeline-c128-d8` | `plaintext-handler` | 2862776.91 | 2600192.01 | +10.10% | 1.880 | 2.881 | -34.74% |
| `pipeline-c128-d8` | `json-static` | 2840551.65 | 2661386.77 | +6.73% | 1.883 | 2.885 | -34.73% |
| `pipeline-c128-d8` | `json-serialize` | 2631438.61 | 2553475.43 | +3.05% | 1.727 | 2.967 | -41.79% |
| `pipeline-c256-d8` | `plaintext-handler` | 2974658.45 | 2591340.85 | +14.79% | 3.413 | 4.322 | -21.03% |
| `pipeline-c256-d8` | `json-static` | 2976859.05 | 2675088.87 | +11.28% | 3.454 | 4.017 | -14.02% |
| `pipeline-c256-d8` | `json-serialize` | 2735180.87 | 2614658.31 | +4.61% | 3.470 | 3.967 | -12.53% |

## Decision

Accepted as diagnostic evidence only. The fresh v1.2.6 run shows that the
HTTP/1.1 pipelined harness remains healthy on current `main`: Kruda and Actix
completed every measured row with zero socket errors and zero non-2xx responses.
Kruda had higher median RPS and lower p99 than Actix in all nine
profile/route rows, but the margins are workload-specific and must remain
labeled as HTTP/1.1 pipelined diagnostics.

Do not implement extra inline pipelined response batching from this result. The
existing syscall evidence in `2026-06-02-wing-response-batching-syscall-evidence.md`
already showed depth-8 `requests_per_write_send` near 8.00 for Kruda, so this
fresh throughput diagnostic does not create a new syscall-count target.
