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
- Minimal difference for zero-alloc benchmarks (StaticGET with Netpoll transport)

## Hardware

Record your hardware specs when publishing results for reproducibility:

```
OS:       macOS / Linux (uname -a)
CPU:      (e.g., Apple M3, AMD Ryzen 9 7950X)
RAM:      (e.g., 16 GB)
Go:       (go version)
GOMAXPROCS: (default = number of CPU cores)
```

## Notes

- This is a separate Go module (`bench/go.mod`) — it does not affect the root Kruda module
- Fiber's `app.Test()` has higher overhead than direct `ServeHTTP` calls, so Fiber numbers include internal request/response conversion cost
- For raw framework overhead without httptest allocations, see `bench_test.go` in the root module (uses zero-alloc mock transport adapters)
