# Performance

Kruda is designed for high-performance, low-latency, high-throughput CPU-bound routes with zero-allocation hot paths, pooled contexts, and pluggable transports.

## Transport Selection

Kruda defaults to **Wing** transport on Linux for maximum performance — raw `epoll` + `eventfd`.

| Transport | Option | Best For |
|-----------|--------|----------|
| Wing | `kruda.New()` (default on Linux) | Maximum throughput for CPU-bound plaintext and JSON routes |
| fasthttp | `kruda.New(kruda.FastHTTP())` (default on macOS) | Broad compatibility |
| net/http | `kruda.New(kruda.NetHTTP())` (default on Windows) | HTTP/2, TLS |

Auto-fallback: TLS config automatically selects net/http. Wing runs on Linux (epoll) and macOS (kqueue).

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

For cross-runtime claims, use the reproducible harness in `bench/reproducible/`. The committed v1.3.0 evidence satisfies the claim gate (median RPS at least +3%, p99 no worse than +10%, zero socket errors and non-2xx) against both rivals on every benchmark route: versus Actix in `bench/reproducible/results/2026-06-12-v1-3-0-string-lane-preset-evidence.md` (CPU routes +13.7% to +19.1%, `/db` +173% to +183%, `/queries` +157% to +170%, `/fortunes` +99.5%), and versus Fiber after the Wing netpoll takeover dispatch in `bench/reproducible/results/2026-06-12-wing-netpoll-takeover-evidence.md` (CPU routes +26.9% to +34.1%, `/fortunes` +10.4% to +11.9%, `/db` +3.1% to +4.5%, `/queries` +3.3% to +5.2% on the default Sonic build — `kruda_stdjson` builds are same ballpark on that one route).

That evidence is same-host loopback with a local PostgreSQL container. It is not a TLS, HTTP/2, or production network claim. The older resource run also shows that Actix still uses less RSS, while Kruda has higher RPS/core on the measured routes. The netpoll takeover trades a few milliseconds of `db`/`queries` p99 for the throughput win; the evidence doc carries the full tables.

Earlier accepted baselines remain in their own evidence files: the v1.2.6 read-only DB revalidation (`bench/reproducible/results/2026-06-06-v126-db-evidence.md`, `/db` +166.13% and `/fortunes` +92.47% versus Actix) and the `984f0d6` CPU-route evidence (`bench/reproducible/results/2026-05-25-main-984f0d6-tiger-evidence.md`).

For post-v1.2.5 candidate work, keep CPU-bound, read-only DB, pipelined HTTP/1.1, and read-buffer memory profiles separate; each profile needs its own evidence and wording.

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

### Reading Benchmark Changes

Go microbenchmarks are noisy. Treat `allocs/op` and `B/op` changes on hot paths as stronger signals than a single `ns/op` movement, and compare repeated runs on the same runner whenever possible.

Kruda's CI uses a 10% regression threshold for benchmark warnings. Borderline `ns/op` changes should be reviewed with `benchstat`, recent main-branch results, and the PR's code path impact before they are treated as real regressions.

Maintainer checklist for performance-sensitive PRs:

- Confirm the changed code runs on the measured hot path.
- Compare against same-runner `main` results, not only an older local baseline.
- Block releases on new allocations or bytes in hot-path benchmarks unless the change is explicitly opt-in.
- Treat security or correctness work that adds cost only when enabled as acceptable when the default path is unchanged.

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

Wing is Kruda's high-performance transport using epoll+eventfd on Linux and kqueue on macOS. It is built into core since v1.2.0. Enable explicitly with `kruda.Wing()` or use `kruda.New()` on Linux (auto-selected).

### Route Presets

Pass a Preset per route to select the optimal dispatch strategy. Presets are
RouteOptions — pass the value directly:

```go
app := kruda.New(kruda.Wing())

app.Get("/",          handler, kruda.Plaintext) // Inline in ioLoop
app.Get("/json",      handler, kruda.JSON)      // Inline in ioLoop
app.Get("/users/:id", handler, kruda.JSON)      // Inline in ioLoop
app.Post("/json",     handler, kruda.JSON)      // Inline in ioLoop
app.Get("/db",        handler, kruda.DB)        // Blocking goroutine per connection
app.Get("/fortunes",  handler, kruda.Render)    // Blocking goroutine per connection
```

CPU-bound routes (plaintext, JSON) use **Inline** dispatch — handler runs directly on the epoll worker with zero overhead.

Read-style I/O routes (DB, Redis) use **Spear** dispatch — handler runs in a blocking goroutine that owns the connection, and the Go runtime auto-creates OS threads as needed.

Phase 6 tiger evidence supports `kruda.DB`/Spear for DB read-style workloads in the reproducible harness: `/db`, `/queries`, and `/fortunes` had much higher median throughput than Inline with zero socket errors and zero non-2xx responses. Write-heavy routes are different. The `/updates` route had zero errors with `kruda.DB` but much higher p99 latency, while Inline produced socket errors in the throughput profile. Treat write-heavy DB routes as workload-specific tuning: benchmark with your real DB pool, p99 target, and error gate before choosing a preset.

For cross-runtime DB comparisons, keep the claim scoped to read-style routes. The accepted v1.2.6 tiger revalidation with framework-specific DB DSN defaults measured throughput-profile `/db` at +166.13% median throughput versus Actix and `/fortunes` at +92.47%, with lower p99 latency in both throughput and latency profiles. This supports a workload-specific DB claim only; the default CPU-bound fair-handler benchmark remains a separate claim with lower but still positive Actix deltas.

### Handler-Level Static JSON

Use `SendStaticJSON` for immutable package-level JSON bytes that should still
run through the normal handler, middleware, lifecycle hooks, cookies, CORS, and
security headers:

```go
var versionJSON = []byte(`{"version":"1.2.3"}`)

app.Get("/version", func(c *kruda.Ctx) error {
    return c.SendStaticJSON(versionJSON)
}, kruda.JSON)
```

This is appropriate for fair handler-path benchmarks and public static JSON
responses that still need application behavior. The byte slice must be
immutable for the lifetime of the program.

### Opt-in Static Wing Responses

For public static hot paths, Wing can bypass the handler pipeline entirely with prebuilt responses:

```go
app.Get("/healthz", func(c *kruda.Ctx) error { return c.Text("ok") },
    kruda.StaticText(200, "text/plain; charset=utf-8", "ok"))
app.Get("/version", func(c *kruda.Ctx) error { return c.JSON(kruda.Map{"version": "1.2.3"}) },
    kruda.StaticJSON(200, `{"version":"1.2.3"}`))
```

These options bypass the handler, middleware, lifecycle hooks, cookies, CORS, and secure-header injection on Wing transports. Use normal handlers when a response needs application behavior. Do not use static bypass routes for fair normal-handler benchmark comparisons.

### Handler Pool Size

Pool dispatch routes share a goroutine pool per worker. Default size = number of workers.

```bash
# Env var (per worker)
KRUDA_POOL_SIZE=16

# Or in code
kruda.New(kruda.WithTransport(kruda.NewWingTransport(kruda.WingConfig{HandlerPoolSize: 16})))
```

Rule of thumb: keep total pool goroutines (workers × pool_size) close to your DB `pool_max_conns`. Too many goroutines cause Go runtime scheduler contention — in benchmarks, 2048 goroutines fighting for 64 DB connections lost 37% throughput vs a right-sized pool.

### Env Vars

`KRUDA_READ_BUF_SIZE` is advanced tuning for Wing's per-connection read buffer.
Lower values can reduce RSS in short-header CPU-only profiles, but requests
whose request line and headers do not fit the buffer are rejected. Keep the
default for general APIs unless a workload-specific benchmark proves the smaller
buffer is safe. The reproducible CPU-bound benchmark uses `4096` for the current
balanced throughput/p99 evidence profile; `2048` is only an optional
short-header memory profile candidate.

| Env Var | Default | Description |
|---------|---------|-------------|
| `KRUDA_WORKERS` | GOMAXPROCS | Number of epoll workers |
| `KRUDA_POOL_SIZE` | workers | Goroutine pool size per worker |
| `KRUDA_READ_BUF_SIZE` | 8192 | Wing read buffer bytes per connection |

Per-route dispatch env vars (`KRUDA_ASYNC`, `KRUDA_POOL_ROUTES`,
`KRUDA_SPAWN_ROUTES`, `KRUDA_STATIC`) were removed in v1.3.0 — use route
presets or `WingConfig.Presets` instead.

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
7. Tune GC for your workload (see below)

## GC Tuning

Go's garbage collector can be tuned via two environment variables. No code changes needed.

### GOGC — GC Frequency

`GOGC` controls how often the garbage collector runs. The default is `100`, meaning GC triggers when the heap grows to **2x** the live data size.

```
Live heap after GC = 50MB

GOGC=100 (default) → next GC at 100MB  (50 + 50×100%)
GOGC=200           → next GC at 150MB  (50 + 50×200%)
GOGC=400           → next GC at 250MB  (50 + 50×400%)
```

Higher values = fewer GC pauses = more throughput, but more memory usage.

### GOMEMLIMIT — Memory Safety Net

`GOMEMLIMIT` sets a soft memory limit (Go 1.19+). When the heap approaches this limit, the GC runs more aggressively — regardless of `GOGC`. This prevents OOM when using high `GOGC` values.

```bash
# High throughput + OOM protection
GOGC=400 GOMEMLIMIT=512MiB ./myapp
```

Without `GOMEMLIMIT`, high `GOGC` values can cause the heap to grow unbounded under load.

### Recommended Presets

| Workload | GOGC | GOMEMLIMIT | Use case |
|----------|------|------------|----------|
| Balanced | `100` (default) | not set | General purpose |
| High throughput | `200`-`500` | set to available RAM | High-traffic API, benchmarks |
| Low memory | `50` | not set | Containers with limited RAM |
| Maximum perf | `off` | set to available RAM | Short benchmarks only |

### How to Set

```bash
# Environment variables (recommended)
GOGC=200 GOMEMLIMIT=512MiB ./myapp

# Docker
ENV GOGC=200 GOMEMLIMIT=512MiB
CMD ["/app"]

# Kubernetes
env:
  - name: GOGC
    value: "200"
  - name: GOMEMLIMIT
    value: "512MiB"
```

> **Tip:** Start with defaults. Profile under load, then tune if GC appears in pprof. `GOGC=200` with `GOMEMLIMIT` set to 80% of your container's memory limit is a safe starting point for high-traffic services.

## Benchmark Results

Use [`bench/reproducible/`](https://github.com/go-kruda/kruda/tree/main/bench/reproducible) for current Kruda vs Fiber vs Actix evidence. The default benchmark is CPU-bound and records RPS, p50, p90, p99, max latency, socket errors, and non-2xx responses for:

| Route | Workload |
|------|----------|
| `/plaintext-handler` | Normal handler path returning plaintext |
| `/json-static` | Normal handler path returning constant JSON bytes, no serialization |
| `/json-serialize` | Normal handler path performing real JSON serialization |

The harness runs both `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s` with one warmup and five measured rounds per framework/route/profile.

**Claim rule:** say Kruda is faster than a rival only when Kruda median RPS is at least 3% higher and p99 is no worse than 10% above that rival with zero socket errors and zero non-2xx responses. Otherwise, say "same ballpark."

The current claims under that rule live in the v1.3.0 evidence files named above (versus Actix and versus Fiber on every benchmark route). The historical `984f0d6` baseline that first satisfied the rule for the CPU-bound Wing handler routes:

| Route | Profile | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda vs Actix p99 | Evidence |
|------|---------|-----------------:|-----------------:|-------------------:|-------------------:|----------|
| `/plaintext-handler` | throughput | 809773.82 | 722328.75 | +12.11% | -77.06% | `main-984f0d6-20260524T171346Z` |
| `/json-static` | throughput | 808953.90 | 712763.16 | +13.50% | -74.24% | `main-984f0d6-20260524T171346Z` |
| `/json-serialize` | throughput | 798032.15 | 706941.96 | +12.89% | -72.87% | `main-984f0d6-20260524T171346Z` |

These are fair handler-path benchmark claims. Wing static bypass route options are documented separately and should not be mixed into handler-path comparison claims.

DB and fortunes workloads are opt-in with `BENCH_ENABLE_DB=1` because database driver, pool, and schema configuration can dominate framework overhead.
