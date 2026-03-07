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
# Run with Go 1.25
go1.25 test -bench=BenchmarkKruda -benchmem -count=10 -tags kruda_stdjson ./... > go125.txt

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

## Wing Transport Benchmark

Wing is Kruda's custom transport using epoll+eventfd (Linux) — zero external dependencies. Wing is the default on Linux. On macOS, the default is fasthttp (Wing kqueue is available via `kruda.Wing()` but not the default).

### Linux (Intel i5-13500, 8 cores, 256 connections, 10s, wrk)

| Framework | Plaintext RPS | JSON RPS |
|-----------|-------------:|----------:|
| **Kruda + Wing (epoll)** | **521,318** | **496,586** |
| Fiber Prefork (8 workers) | 268,222 | 244,058 |
| Kruda + fasthttp | 231,460 | 219,445 |
| Fiber | 229,421 | 213,405* |

*\* Fiber JSON returned non-2xx (route mismatch in bench setup)*

**Wing vs Fiber Prefork: 1.94x plaintext, 2.03x JSON**
**Wing vs fasthttp: 2.25x plaintext, 2.26x JSON**

### macOS (Apple M3, 8 cores, 100 connections, 10s, wrk)

| Framework | Plaintext RPS | JSON RPS |
|-----------|-------------:|----------:|
| **Kruda + Wing (kqueue)** | **263,485** | **267,938** |
| Kruda + fasthttp | 207,751 | 213,632 |

**Wing (kqueue) vs fasthttp: 1.27x plaintext, 1.25x JSON**

### Micro Benchmarks (go test -bench)

| Benchmark | M3 (ns/op) | i5-13500 (ns/op) | i5-14500 (ns/op) | Allocs |
|-----------|-----------|-------------------|-------------------|--------|
| ParseGET | 152 | 170 | 288 | 5 allocs, 136 B |
| ParsePOST | 195 | 237 | 373 | 5 allocs, 176 B |
| ResponseBuild (zero-copy) | 100 | 140 | — | 4 allocs, 360 B |
| ResponseBuildCopy | 42 | 58 | — | 1 alloc, 112 B |
| FullCycle (parse→respond) | 262 | 284 | 519 | 8 allocs, 304 B |
| HandlerInline (full path) | 283 | 283 | 532 | 8 allocs, 304 B |

*Full request→response cycle: ~280ns (M3/Linux), ~520ns (Windows), 8 allocations*

### Windows

Wing does not support Windows. On Windows, `kruda.Wing()` automatically falls back
slower than fasthttp due to Go's mature net package and Windows SO_REUSEADDR not
distributing connections like Linux SO_REUSEPORT. The maintenance cost and bug risk
were not justified.

### How to Run

```bash
# Micro benchmarks
cd transport/wing && go test -bench=. -benchmem -count=3 ./...

# wrk benchmark (build servers, then use wrk)
# See bench/transport-compare/ for standalone bench servers
```

*Tested: 2026-03-01, Go 1.26, wrk 4.2.0*

## Notes

- This is a separate Go module (`bench/go.mod`) — it does not affect the root Kruda module
- Fiber's `app.Test()` has higher overhead than direct `ServeHTTP` calls, so Fiber numbers include internal request/response conversion cost
- For raw framework overhead without httptest allocations, see `bench_test.go` in the root module (uses zero-alloc mock transport adapters)
