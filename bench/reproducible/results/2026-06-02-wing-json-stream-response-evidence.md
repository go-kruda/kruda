# Wing JSON Stream Response Evidence

Date: 2026-06-02

Scope: local microbenchmark evidence plus focused tiger diagnostic pipeline
evidence for the Wing handler-path JSON serialization response path. This is
not a public broad "faster than Actix" claim and not a 20% win claim.

## Candidate

Candidate commit: `d77ff70` (`perf: stream Wing stdjson responses into pooled buffers`)

When the active JSON encoder is `encoding/json`, Wing now lets `Ctx.JSON` encode
directly into a response-owned `bytes.Buffer` before using Wing's JSON fast
response builder. This avoids the intermediate `[]byte` allocation from
`encoding/json.Marshal` on the Wing JSON serialization route.

The candidate is intentionally limited to the stdjson build because Sonic's
current `MarshalToBuffer` path still calls `sonic.Marshal` and then writes the
result into a buffer, which did not improve the route in the rejected Sonic
buffer encoder experiment.

Custom `WithJSONEncoder` behavior is preserved: a custom encoder still uses the
configured encoder and does not enter the stdjson stream response path unless a
stream encoder is configured for a build where the stream path is enabled.

## Local Microbenchmark

Baseline worktree: `origin/main` at `6ad22a7`

Candidate branch: `perf/wing-json-response-path` at `d77ff70`

Command:

```bash
go test -run '^$' -tags kruda_stdjson \
  -bench 'BenchmarkCPU(ResponseJSON|HandlerJSON(Static|StaticBytes|Serialize)Feather)$' \
  -benchmem -count=10 .
```

Environment:

- Host: local Apple M3
- OS/arch: `darwin/arm64`
- Build tags: `kruda_stdjson`

Median comparison from the count=10 raw benchmark outputs:

| Benchmark | Main ns/op | Candidate ns/op | Delta | Main B/op | Candidate B/op | Main alloc/op | Candidate alloc/op |
|---|---:|---:|---:|---:|---:|---:|---:|
| `BenchmarkCPUHandlerJSONSerializeFeather` | 312.80 | 297.65 | -4.84% | 192 | 160 | 2 | 1 |
| `BenchmarkCPUHandlerJSONStaticBytesFeather` | 232.20 | 236.20 | +1.72% | 160 | 160 | 1 | 1 |
| `BenchmarkCPUHandlerJSONStaticFeather` | 224.00 | 220.85 | -1.41% | 160 | 160 | 1 | 1 |
| `BenchmarkCPUResponseJSON` | 42.86 | 43.30 | +1.01% | 160 | 160 | 1 | 1 |

Interpretation:

- The targeted JSON serialization handler path improves by 4.84%.
- The targeted path drops from 2 alloc/op and 192 B/op to 1 alloc/op and
  160 B/op.
- Static JSON helper paths stay within the 5% regression gate with no allocation
  increase.

## Tiger Diagnostic Pipeline Evidence

Result directory:
`/home/tiger/kruda-wing-json-response-path-d77ff70/bench/reproducible/results/pipeline-json-stdjson-candidate-20260602T144713Z`

Command shape:

```bash
BENCH_FRAMEWORKS="kruda actix" \
BENCH_ROUNDS=3 \
PIPELINE_DURATION=5s \
PIPELINE_WARMUP=2s \
PIPELINE_PROFILES="pipeline-c128-d8:128:8 pipeline-c256-d8:256:8" \
KRUDA_GO_TAGS=kruda_stdjson \
./bench/reproducible/pipeline.sh json-serialize
```

Environment summary:

- CPU: 13th Gen Intel Core i5-13500, 8 online CPUs
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.26.3 linux/amd64`
- Rust: `rustc 1.93.1`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- Kruda tags: `kruda_stdjson`
- Kruda server log: `[kruda] JSON encoder: encoding/json`
- Route: `json-serialize`
- Frameworks: Kruda and Actix
- Errors: zero socket errors and zero non-2xx responses in all rounds

Median result:

| Profile | Kruda median RPS | Actix median RPS | RPS delta | Kruda p99 ms | Actix p99 ms | p99 delta | Errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| `pipeline-c128-d8` | 2,931,759.93 | 2,797,118.54 | +4.81% | 1.384 | 2.385 | -41.97% | 0 | 0 |
| `pipeline-c256-d8` | 2,997,507.41 | 2,921,522.08 | +2.60% | 2.658 | 3.589 | -25.94% | 0 | 0 |

## Default/Sonic Control

Control result directory:
`/home/tiger/kruda-wing-pipeline-io-d270381/bench/reproducible/results/pipeline-json-default-20260602T143225Z`

This was run before the stdjson stream candidate with `KRUDA_GO_TAGS=default`.
The Kruda server log reported `[kruda] JSON encoder: sonic`.

Median result:

| Profile | Kruda median RPS | Actix median RPS | RPS delta | Kruda p99 ms | Actix p99 ms | Errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|
| `pipeline-c128-d8` | 2,879,487.12 | 2,902,411.98 | -0.79% | 1.415 | 2.667 | 0 | 0 |
| `pipeline-c256-d8` | 2,919,853.62 | 2,920,094.44 | -0.01% | 2.746 | 3.833 | 0 | 0 |

Interpretation:

- Default Sonic already sits very close to Actix for this diagnostic route and
  has better p99 in these runs.
- The kept runtime change is for the stdjson Wing JSON serialization path only.
- The Sonic control does not justify changing the default Sonic response path.

## Decision

Keep the stdjson Wing JSON stream response candidate.

This is a real normal handler-path improvement for `json-serialize` under
`kruda_stdjson`: it reduces one allocation in the local hot-path benchmark and
turns the focused tiger pipelined diagnostic comparison from a previous deficit
into positive RPS deltas with materially better p99 latency.

Do not claim a broad Actix win from this evidence. The `pipeline-c128-d8` profile
passes the 3% RPS and p99 gate, but `pipeline-c256-d8` is only +2.60% RPS, below
the public claim threshold. The correct wording is that this narrows/improves
the Wing JSON serialization path and remains in the same ballpark as Actix for
this diagnostic workload.
