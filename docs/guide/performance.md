# Performance

Kruda is designed for high performance with zero-allocation hot paths, pooled contexts, and pluggable transports.

## Transport Selection

Kruda defaults to **fasthttp** for maximum performance. Use `NetHTTP()` when you need HTTP/2 or TLS.

| Transport | Option | Best For |
|-----------|--------|----------|
| fasthttp | `kruda.New()` (default) | Maximum throughput, low latency |
| net/http | `kruda.New(kruda.NetHTTP())` | HTTP/2, TLS, Windows |

Auto-fallback: TLS config or Windows automatically selects net/http even without `NetHTTP()`.

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

Sonic is used automatically via build tags. Falls back to `encoding/json` transparently.

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

## Profile-Guided Optimization (PGO)

Go 1.21+ supports PGO — the compiler uses a CPU profile of your app to make better inlining, devirtualization, and layout decisions. This gives **2-7% free performance** with zero code changes.

### Quick Start

```bash
# Install the CLI (if not already)
go install github.com/go-kruda/kruda/cmd/kruda@latest

# Generate a PGO profile (auto-load mode)
kruda pgo --auto --duration 30

# Or interactive mode: you provide the load
kruda pgo --duration 60
```

This creates `default.pgo` in your main package directory. Go auto-detects it on the next `go build`.

### How It Works

1. **Profile** — Run your app under realistic load while collecting a CPU profile
2. **Build** — Go reads `default.pgo` and optimizes hot code paths
3. **Deploy** — Your binary is 2-7% faster with no code changes

### Setup

Add pprof to your main.go:

```go
import (
    "net/http"
    _ "net/http/pprof"
)

func main() {
    // Start pprof server on a separate port
    go http.ListenAndServe(":6060", nil)

    app := kruda.New()
    // ... your routes
    app.Listen(":3000")
}
```

### CLI Commands

```bash
kruda pgo                          # Interactive: you generate load manually
kruda pgo --auto                   # Auto: uses bombardier to generate load
kruda pgo --auto -d 60             # Auto with 60s profiling duration
kruda pgo --auto --endpoints /,/users/1,/api/health  # Specific endpoints
kruda pgo --auto -c 200            # 200 bombardier connections
kruda pgo -o ./profiles/v2.pgo     # Custom output path

kruda pgo info                     # Check if PGO is active
kruda pgo strip                    # Disable PGO (backs up the profile)
```

### Best Practices

1. **Representative load** — Profile under realistic traffic patterns, not just `/`
2. **Re-profile after major changes** — Update `default.pgo` when your hot paths change significantly
3. **Commit to repo** — `default.pgo` should be in version control so CI/CD builds are optimized
4. **Use Go 1.26+** — Green Tea GC + improved PGO gives the best results
5. **Combine with Sonic** — PGO + Sonic JSON gives maximum throughput

### Verify PGO is Active

```bash
go build -v ./... 2>&1 | grep -i pgo
```

You should see your package listed as using PGO.

## Wing Transport Tuning

Wing is Kruda's high-performance epoll/kqueue transport. Enable with `kruda.Wing()`.

### Wing Types (Feather)

Assign a Wing type per route to select the optimal dispatch strategy:

```go
app := kruda.New(kruda.Wing())

app.Get("/",          handler, kruda.WingPlaintext())  // Inline, zero-copy
app.Get("/json",      handler, kruda.WingJSON())       // Inline, fixed buffer
app.Get("/users/:id", handler, kruda.WingParamJSON())  // Inline, param extraction
app.Post("/json",     handler, kruda.WingPostJSON())   // Inline, body parse
app.Get("/db",        handler, kruda.WingQuery())      // Pool dispatch
app.Get("/fortunes",  handler, kruda.WingRender())     // Pool dispatch, growable buffer
```

CPU-bound routes (plaintext, JSON) use **Inline** dispatch — handler runs directly on the epoll worker with zero overhead.

I/O-bound routes (DB, Redis) use **Pool** dispatch — handler runs in a goroutine pool so the epoll worker isn't blocked.

### Handler Pool Size

Pool dispatch routes share a goroutine pool per worker. Default size = number of workers.

```bash
# Env var (per worker)
KRUDA_POOL_SIZE=16

# Or in code
kruda.New(kruda.Wing(wing.Config{HandlerPoolSize: 16}))
```

Rule of thumb: keep total pool goroutines (workers × pool_size) close to your DB `pool_max_conns`. Too many goroutines cause Go runtime scheduler contention — in benchmarks, 2048 goroutines fighting for 64 DB connections lost 37% throughput vs a right-sized pool.

### Env Vars

| Env Var | Default | Description |
|---------|---------|-------------|
| `KRUDA_WORKERS` | GOMAXPROCS | Number of epoll workers |
| `KRUDA_POOL_SIZE` | workers | Goroutine pool size per worker |
| `KRUDA_BATCH_WRITE` | off | Coalesce pipelined responses (`1` to enable) |
| `KRUDA_POOL_ROUTES` | — | Comma-separated routes for Pool dispatch |
| `KRUDA_SPAWN_ROUTES` | — | Comma-separated routes for Spawn dispatch |

## Production Tips

1. Use Wing transport on Linux for maximum throughput (`kruda.Wing()`)
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
6. Enable PGO for 2-7% free performance (see above)
