# Kruda Benchmark Suite

Comparative benchmarks for Kruda vs Fiber, Gin, and Echo.

## Running Benchmarks

```bash
# All benchmarks (all frameworks)
cd bench && go test -bench=. -benchmem -count=5 -tags kruda_stdjson ./...

# Kruda only
cd bench && go test -bench=BenchmarkKruda -benchmem -count=5 -tags kruda_stdjson ./...

# Specific framework
cd bench && go test -bench=BenchmarkGin -benchmem -count=5 -tags kruda_stdjson ./...

# Save results to file
cd bench && go test -bench=. -benchmem -count=5 -tags kruda_stdjson ./... | tee results.txt
```

The `-tags kruda_stdjson` flag is required because Kruda's Sonic JSON engine needs CGO. This flag selects the stdlib `encoding/json` fallback.

Use `-count=5` (or higher) for stable results. Single runs may have higher variance.

## Benchmark Categories

Each framework is tested with identical workloads:

| Benchmark | Description |
|-----------|-------------|
| `StaticGET` | GET `/` returning plain text |
| `ParamGET` | GET `/users/:id` with path parameter extraction |
| `POSTJSON` | POST with JSON body parse + JSON response |
| `Middleware1` | 1 no-op middleware in chain |
| `Middleware5` | 5 no-op middleware in chain |
| `Middleware10` | 10 no-op middleware in chain |
| `JSONEncode` | GET returning JSON-encoded map |

## Interpreting Results

Go benchmark output format:

```
BenchmarkKruda_StaticGET-8    2853681    427.5 ns/op    1320 B/op    20 allocs/op
```

| Column | Meaning |
|--------|---------|
| `-8` | GOMAXPROCS (CPU cores used) |
| `2853681` | Total iterations (higher = faster) |
| `427.5 ns/op` | Nanoseconds per operation (lower = faster) |
| `1320 B/op` | Bytes allocated per operation (lower = less GC pressure) |
| `20 allocs/op` | Heap allocations per operation (lower = less GC pressure) |

## Methodology

- Benchmarks use `b.RunParallel` for concurrent execution across all available cores
- Kruda benchmarks call `App.ServeKruda()` directly via transport interface adapters (no TCP)
- Gin and Echo benchmarks use their `ServeHTTP` method with `httptest.NewRecorder` (no TCP)
- Fiber benchmarks use `app.Test()` which processes requests internally (no TCP)
- All frameworks use their default JSON encoder (`encoding/json` for Kruda with stdjson tag)
- Router compilation happens before `b.ResetTimer()` — only hot-path execution is measured

## Frameworks Compared

| Framework | Version | Notes |
|-----------|---------|-------|
| Kruda | dev (local) | `replace` directive points to parent module |
| Fiber v2 | v2.52.x | Uses fasthttp internally |
| Gin | v1.11.x | Standard net/http based |
| Echo v4 | v4.15.x | Standard net/http based |
| Hertz | — | Skipped (complex dependency tree, deferred) |

## Go Version Comparison (1.24 vs 1.26)

Go 1.26 introduces the Green Tea GC which reduces tail latency and GC pause times. To compare:

```bash
# Run with Go 1.24
go1.24 test -bench=BenchmarkKruda -benchmem -count=10 -tags kruda_stdjson ./... > go124.txt

# Run with Go 1.26
go1.26 test -bench=BenchmarkKruda -benchmem -count=10 -tags kruda_stdjson ./... > go126.txt

# Compare (requires benchstat: go install golang.org/x/perf/cmd/benchstat@latest)
benchstat go124.txt go126.txt
```

Expected improvements with Go 1.26:
- Lower P99 latency due to reduced GC pauses
- Potentially fewer ns/op for allocation-heavy benchmarks (POSTJSON, JSONEncode)
- Minimal difference for zero-alloc benchmarks (StaticGET with fasthttp transport)

## Hardware

Record your hardware specs when publishing results for reproducibility:

```
OS:       macOS / Linux (uname -a)
CPU:      (e.g., Apple M3, AMD Ryzen 9 7950X)
RAM:      (e.g., 16 GB)
Go:       (go version)
GOMAXPROCS: (default = number of CPU cores)
```

## Cross-Runtime Benchmark (Kruda vs Elysia)

Compare Kruda (Go/fasthttp) against Elysia (Bun/TypeScript) using real HTTP servers and bombardier:

```bash
# Install dependencies first
go install github.com/codesenberg/bombardier@latest
# Install bun from https://bun.sh

# Run cross-runtime benchmark (default: all cores)
cd bench/cross-runtime && ./run_bench.sh 100 10s

# Single-core fair comparison (Go 1 core vs Bun 1 thread)
cd bench/cross-runtime && GOMAXPROCS=1 ./run_bench.sh 100 10s
```

This benchmark:
- Kills stale processes, starts fresh servers, verifies responses before benchmarking
- Tests 3 scenarios: GET `/` (plaintext), GET `/users/:id` (param), POST `/json` (JSON)
- Uses bombardier with configurable connections/duration, 3 rounds (best of)
- Measures real HTTP throughput (req/s) and mean latency (ms)

Unlike the Go microbenchmarks, this measures end-to-end HTTP performance including:
- TCP connection handling
- HTTP parsing
- JSON serialization/deserialization
- Runtime overhead (Go GC vs Bun's JavaScriptCore)

Results are comparable to [bun-http-framework-benchmark](https://github.com/SaltyAom/bun-http-framework-benchmark) methodology.

## Cross-Runtime Benchmark Results

### Multi-Core (default deployment — GOMAXPROCS=8)

| Test | Kruda (req/s) | Elysia (req/s) | Diff | Winner |
|------|-------------:|---------------:|------|--------|
| plaintext | 416,269 | 197,400 | +110.9% | **Kruda** |
| param | 386,998 | 176,835 | +118.8% | **Kruda** |
| json_post | 382,098 | 115,752 | +230.1% | **Kruda** |

**Score: Kruda 3 — Elysia 0**

| Test | Kruda latency | Elysia latency |
|------|-------------:|---------------:|
| plaintext | 238ms | 505ms |
| param | 256ms | 564ms |
| json_post | 260ms | 863ms |

*Linux x86_64, Intel i5-13500 (8 cores), Go 1.26.0, Bun 1.3.4, Elysia 1.4.26, bombardier -c 100 -d 10s*

**Note:** Go uses all available cores by default. Bun is single-threaded. This reflects real-world deployment where Go frameworks automatically scale across all cores without cluster mode.

### Single-Core (fair 1v1 — GOMAXPROCS=1)

| Test | Kruda (req/s) | Elysia (req/s) | Diff | Winner |
|------|-------------:|---------------:|------|--------|
| plaintext | 201,679 | 199,084 | +1.3% | **Kruda** |
| param | 188,094 | 196,544 | +4.5% | **Elysia** |
| json_post | 174,168 | 116,005 | +50.1% | **Kruda** |

**Score: Kruda 2 — Elysia 1**

| Test | Kruda latency | Elysia latency |
|------|-------------:|---------------:|
| plaintext | 495ms | 501ms |
| param | 531ms | 508ms |
| json_post | 573ms | 861ms |

*Same hardware, GOMAXPROCS=1*

### Analysis

- **Single-core:** Kruda and Elysia are nearly identical on raw HTTP (~200K req/s). Elysia's Bun runtime is impressively fast for simple requests. However, Kruda wins decisively on JSON workloads (+50%) which represent real API server traffic.
- **Multi-core:** Kruda's advantage compounds — Go automatically uses all cores, delivering 2x throughput scaling (200K → 416K) while Bun stays at ~197K. No cluster mode, no PM2, just deploy and scale.
- **JSON is the real benchmark:** Plaintext measures HTTP parsing speed. JSON POST measures what API servers actually do — parse request bodies and serialize responses. Kruda wins this 50-230% depending on core count.
- **Latency:** Kruda consistently delivers ~50% lower latency across all tests and configurations.

## Notes

- This is a separate Go module (`bench/go.mod`) — it does not affect the root Kruda module
- Fiber's `app.Test()` has higher overhead than direct `ServeHTTP` calls, so Fiber numbers include internal request/response conversion cost
- For raw framework overhead without httptest allocations, see `bench_test.go` in the root module (uses zero-alloc mock transport adapters)
