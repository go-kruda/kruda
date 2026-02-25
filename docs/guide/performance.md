# Performance

Kruda is designed for high performance with zero-allocation hot paths, pooled contexts, and pluggable transports.

## Transport Selection

Kruda supports two transports:

| Transport | Platform | Best For |
|-----------|----------|----------|
| Netpoll | Linux only | Maximum throughput, low latency |
| net/http | All platforms | Compatibility, HTTP/2, TLS |

Auto-selection logic:
1. Linux + Netpoll available → Netpoll
2. Windows/macOS → net/http
3. Explicit config overrides auto-selection

Netpoll uses Linux epoll for non-blocking I/O, providing significantly higher throughput for high-concurrency workloads.

## Context Pool Tuning

Kruda pools `Ctx` objects using `sync.Pool` to avoid allocation on every request:

- Contexts are acquired from the pool at request start
- Internal maps are cleared with `clear()` (no reallocation)
- Contexts are returned to the pool after the response is sent

This is automatic — no configuration needed. The pool self-tunes based on GC pressure.

## Middleware Ordering

Middleware order affects performance. Place frequently-short-circuiting middleware first:

```go
// Good — rate limiter rejects early, before expensive work
app.Use(RateLimiter)
app.Use(middleware.Logger())
app.Use(middleware.Recovery())
app.Use(AuthMiddleware)

// Less optimal — logger runs on every request including rate-limited ones
app.Use(middleware.Logger())
app.Use(RateLimiter)
app.Use(middleware.Recovery())
app.Use(AuthMiddleware)
```

Handler chains are pre-built at registration time — there's no per-request chain construction overhead.

## Router Performance

The radix tree router provides:

- O(1) child lookup via indices string
- Static routes matched in constant time
- Parameter routes with minimal allocation
- Pre-compiled handler chains

Route registration order doesn't affect lookup performance.

## JSON Performance

| Engine | CGO Required | Performance |
|--------|-------------|-------------|
| Sonic | Yes | ~2-3x faster than encoding/json (SIMD) |
| encoding/json | No | Standard Go performance |

Sonic is used by default when CGO is available. Falls back to `encoding/json` transparently.

Force stdlib JSON:

```bash
go build -tags kruda_stdjson ./...
```

## Zero-Allocation Hot Path

Kruda minimizes allocations on the request hot path:

- Context reuse via `sync.Pool`
- Pre-built middleware chains (no slice allocation per request)
- `unsafe.String` / `unsafe.Slice` for zero-copy byte↔string conversion
- Router lookup without allocation for static routes

## Benchmark Results

Run benchmarks locally:

```bash
go test -bench=. -benchmem ./...
```

Key benchmarks:

| Benchmark | Description |
|-----------|-------------|
| `BenchmarkPlaintext` | Raw handler dispatch |
| `BenchmarkJSON` | JSON serialization response |
| `BenchmarkRouterStatic` | Static route lookup |
| `BenchmarkRouterParam` | Parameterized route lookup |
| `BenchmarkMiddleware1` | Single middleware chain |
| `BenchmarkMiddleware5` | Five middleware chain |
| `BenchmarkTypedHandler` | C[T] typed handler |

Compare against baseline:

```bash
go test -bench=. -benchmem -count=5 ./... > new.txt
benchstat bench/baseline.txt new.txt
```

## Production Tips

1. Use Netpoll on Linux for maximum throughput
2. Set appropriate timeouts to prevent resource exhaustion:
   ```go
   kruda.New(
       kruda.WithReadTimeout(30 * time.Second),
       kruda.WithWriteTimeout(30 * time.Second),
       kruda.WithBodyLimit(4 * 1024 * 1024),
   )
   ```
3. Place rate limiting and auth middleware early in the chain
4. Use `kruda_stdjson` build tag if CGO is problematic in your environment
5. Disable dev mode in production (`WithDevMode(false)` — the default)
