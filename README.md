# Kruda

Fast by default, type-safe by design.

[![Go Version](https://img.shields.io/badge/Go-1.25.11+-00ADD8?logo=go)](https://go.dev)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-kruda/kruda.svg)](https://pkg.go.dev/github.com/go-kruda/kruda)
[![CI](https://github.com/go-kruda/kruda/actions/workflows/test.yml/badge.svg)](https://github.com/go-kruda/kruda/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/go-kruda/kruda/graph/badge.svg?token=9HIGH8L2Q7)](https://codecov.io/gh/go-kruda/kruda)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why Kruda?

- Typed handlers `C[T]` — body + param + query parsed into one struct, validated at compile time
- Auto CRUD — implement `ResourceService[T]`, get 5 REST endpoints
- Built-in DI — optional, no codegen, type-safe generics
- Pluggable transport — Wing in core (Linux, epoll+eventfd), fasthttp, or net/http
- Single-tag releases — one `vX.Y.Z` covers core and contrib (no more sub-module coordination)
- Minimal deps — Sonic JSON (opt-out via `kruda_stdjson`), pluggable transport
- AI-friendly — typed API + 23 examples = AI generates correct code on first try

## Quick Start

```bash
go get github.com/go-kruda/kruda
```

```go
package main

import (
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/middleware"
)

func main() {
    app := kruda.New()
    app.Use(middleware.Recovery(), middleware.Logger())

    app.Get("/ping", func(c *kruda.Ctx) error {
        return c.JSON(kruda.Map{"pong": true})
    })

    app.Listen(":3000")
}
```

## Typed Handlers

```go
type CreateUser struct {
    Name  string `json:"name" validate:"required,min=2"`
    Email string `json:"email" validate:"required,email"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

kruda.Post[CreateUser, User](app, "/users", func(c *kruda.C[CreateUser]) (*User, error) {
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})
```

## Auto CRUD

```go
kruda.Resource[User, string](app, "/users", &UserCRUD{db: db})
// Registers: GET /users, GET /users/:id, POST /users, PUT /users/:id, DELETE /users/:id
```

## Dependency Injection

```go
c := kruda.NewContainer()
c.Give(&UserService{})
c.GiveLazy(func() (*DBPool, error) { return connectDB() })
c.GiveNamed("write", &DB{DSN: "primary"})

app := kruda.New(kruda.WithContainer(c))
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)
    return c.JSON(svc.ListAll())
})
```

## Error Mapping

```go
app.MapError(ErrNotFound, 404, "resource not found")
kruda.MapErrorType[*ValidationError](app, 422, "validation failed")
```

## Coming from Another Framework?

| Concept | Kruda | Gin | Fiber | Echo | stdlib |
|---------|-------|-----|-------|------|--------|
| App | `kruda.New()` | `gin.Default()` | `fiber.New()` | `echo.New()` | `http.NewServeMux()` |
| Route | `app.Get("/path", h)` | `r.GET("/path", h)` | `app.Get("/path", h)` | `e.GET("/path", h)` | `mux.HandleFunc("GET /path", h)` |
| Typed handler | `kruda.Post[In, Out](app, "/path", h)` | — | — | — | — |
| Group | `app.Group("/api")` | `r.Group("/api")` | `app.Group("/api")` | `e.Group("/api")` | — |
| Middleware | `app.Use(mw)` | `r.Use(mw)` | `app.Use(mw)` | `e.Use(mw)` | — |
| Context | `*kruda.Ctx` | `*gin.Context` | `*fiber.Ctx` | `echo.Context` | `http.ResponseWriter, *http.Request` |
| JSON response | `return &obj, nil` | `c.JSON(200, obj)` | `c.JSON(obj)` | `c.JSON(200, obj)` | `json.NewEncoder(w).Encode(obj)` |
| Path param | `c.Param("id")` | `c.Param("id")` | `c.Params("id")` | `c.Param("id")` | `r.PathValue("id")` |
| Query param | `c.Query("q")` | `c.Query("q")` | `c.Query("q")` | `c.QueryParam("q")` | `r.URL.Query().Get("q")` |
| Body binding | `c.Bind(&v)` or `C[T].In` | `c.ShouldBindJSON(&v)` | `c.BodyParser(&v)` | `c.Bind(&v)` | `json.NewDecoder(r.Body).Decode(&v)` |
| Auto CRUD | `kruda.Resource[T, ID](app, "/path", svc)` | — | — | — | — |
| DI | `Container.Give()` / `MustResolve[T](c)` | — | — | — | — |

> Full migration guides: [Gin](docs/guide/coming-from-gin.md) · [Fiber](docs/guide/coming-from-fiber.md) · [Echo](docs/guide/coming-from-echo.md) · [stdlib](docs/guide/coming-from-stdlib.md)

## Benchmarks

Kruda is built for high-performance, low-latency, high-throughput CPU-bound routes. Public comparison claims use the reproducible harness in [`bench/reproducible/`](bench/reproducible/) and must report throughput, p50/p90/p99/max latency, socket errors, and non-2xx responses.

Default CPU-bound routes:

| Route | What it measures |
|------|------------------|
| `/plaintext-handler` | Normal handler path returning plaintext |
| `/json-static` | Normal handler path returning constant JSON bytes, no serialization |
| `/json-serialize` | Normal handler path performing real JSON serialization |

The benchmark runs Kruda, Fiber, and Actix with `wrk --latency` across latency and throughput profiles. Kruda should be described as faster than a rival only when median RPS is at least 3% higher and p99 is no worse than 10% above that rival with zero errors. Otherwise, use "same ballpark."

Committed tiger evidence for the v1.3.0 release gate (`142a418`) satisfies that gate for the CPU-bound Wing handler routes below:

| Route | Profile | Kruda vs Actix median RPS | Kruda vs Actix p99 |
|------|---------|---------------------------:|-------------------:|
| `/plaintext-handler` | throughput | +13.88% | -70.73% |
| `/json-static` | throughput | +13.71% | -72.33% |
| `/json-serialize` | throughput | +14.53% | -70.76% |

Evidence: `bench/reproducible/results/2026-06-12-v1-3-0-string-lane-preset-evidence.md` (earlier baseline: `bench/reproducible/results/2026-05-25-main-984f0d6-tiger-evidence.md`). These are normal handler-path routes, not Wing static bypass routes. The older resource evidence (`resource-main-984f0d6-20260524T174429Z/`) shows higher RPS/core than Actix while Actix still uses less RSS.

Opt-in read-only DB workload evidence is tracked separately because database driver, pool, and schema behavior can dominate framework overhead. In the accepted v1.2.6 tiger revalidation with `BENCH_ENABLE_DB=1`, `BENCH_KRUDA_DB_DISPATCH=takeover`, framework-specific default DB DSNs, and zero socket errors/non-2xx responses, Kruda was materially faster than Actix on read-style routes:

| Route | Profile | Kruda vs Actix median RPS | Kruda vs Actix p99 |
|------|---------|---------------------------:|-------------------:|
| `/db` | throughput | +183.42% | -84.42% |
| `/fortunes` | throughput | +99.48% | -71.57% |

Evidence: `bench/reproducible/results/2026-06-12-v1-3-0-string-lane-preset-evidence.md` (v1.2.6 baseline: `2026-06-06-v126-db-evidence.md`). Treat this as a workload-specific read-only DB result, not a broad CPU-bound handler-path claim.

With the Wing netpoll takeover plus the v1.3.1 adaptive-spin dispatch, Kruda's standing versus Fiber on every benchmark route (default Sonic build, 5 rounds, zero errors):

| Route | Profile | Kruda vs Fiber median RPS | Kruda vs Fiber p99 |
|------|---------|---------------------------:|-------------------:|
| `/plaintext-handler` | throughput | +28.8% | -78.2% |
| `/json-static` | throughput | +27.8% | -80.1% |
| `/json-serialize` | throughput | +27.2% | -69.1% |
| `/fortunes` | throughput | +6.7% | -1.7% |
| `/db` | throughput | +2.7% | -15.5% |
| `/queries` | throughput | +1.9% | -6.6% |

Evidence: `bench/reproducible/results/2026-06-13-v1-3-1-consolidated-evidence.md` (5 rounds per cell, zero errors). Read it by workload: the CPU routes and `/fortunes` are decisive RPS *and* p99 wins; `/db` and `/queries` are pool-bound and sit at the pgx ceiling (`2026-06-13-db-route-ceiling-evidence.md`), so Kruda **matches** Fiber on their RPS and **beats** it on p99. The v1.3.1 takeover-spin (`2026-06-13-takeover-spin-p99-evidence.md`) removed the v1.3.0 `db`/`queries` p99 trade rather than chasing RPS the floor will not yield.

These figures were measured at the v1.3.1 runtime and **revalidated at the v1.4.0 code** (`12d6a80`): a paired `v1.3.1 → HEAD` A/B (Kruda-only, default Sonic, 5 rounds, zero socket errors / zero non-2xx) shows **RPS parity on all six routes**. The DB-bound routes (`/fortunes`, `/db`, `/queries`) were additionally re-run in reversed checkout order, which confirmed their forward-pass p99 spikes as shared-box run-order contention rather than a code change; the CPU routes' p99 values are sub-millisecond (~1 ms at ~800K RPS), so their forward-only percentage deltas swing on a tiny denominator and are within measurement noise. So the standing comparisons carry forward to the released code. Evidence: `bench/reproducible/results/2026-06-27-v1-4-0-consolidated-ab-evidence.md`.

Wing transport uses raw `epoll` + `eventfd` on Linux and bypasses both fasthttp and net/http. macOS defaults to fasthttp.

Production HTTPS: terminate TLS at a proxy/load balancer in front of Wing, or use direct `WithTLS` which serves via net/http (these benchmark numbers then do not apply) — see the [transport guide](docs/guide/transport.md#production-tls-terminate-in-front-of-wing).

## Documentation

- [API Reference (pkg.go.dev)](https://pkg.go.dev/github.com/go-kruda/kruda)
- [Examples](examples/) — 23 runnable examples
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Benchmark Charts](https://go-kruda.github.io/kruda/benchmarks/)

## AI Integration

Kruda includes a built-in [MCP](https://modelcontextprotocol.io/) server for AI coding assistants (Claude Code, Cursor, Copilot).

```bash
# Install CLI
go install github.com/go-kruda/kruda/cmd/kruda@latest

# New projects — .mcp.json included automatically
kruda new myapp

# Existing projects
kruda mcp init        # generates .mcp.json + .cursor/mcp.json
kruda mcp --test      # verify it works
```

| Tool | Description |
|------|-------------|
| `kruda_new` | Scaffold a new project |
| `kruda_add_handler` | Generate a typed handler with `C[T]` pattern |
| `kruda_add_resource` | Generate a CRUD `ResourceService` |
| `kruda_list_routes` | Scan source code and list all registered routes |
| `kruda_suggest_wing` | Suggest Wing route presets for routes |
| `kruda_docs` | Look up Kruda docs and code examples |

## Security

See [SECURITY.md](SECURITY.md) for our responsible disclosure policy.

### Security Hardening (Recommended)

```go
import (
    "os"
    "time"

    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/middleware"
    "github.com/go-kruda/kruda/contrib/jwt"
    "github.com/go-kruda/kruda/contrib/ratelimit"
)

app := kruda.New(
    kruda.WithBodyLimit(1024 * 1024), // 1MB body limit
    kruda.WithReadTimeout(10 * time.Second),
)

// Rate limiting — 100 req/min per IP
app.Use(ratelimit.New(ratelimit.Config{
    Max: 100, Window: time.Minute,
    TrustedProxies: []string{"10.0.0.1", "10.0.0.2"},
}))

// Stricter limit on auth endpoints
app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))

// JWT authentication on protected routes
api := app.Group("/api").Guard(jwt.New(jwt.Config{
    Secret: []byte(os.Getenv("JWT_SECRET")),
}))
```

### Contrib Modules

| Module | Install | Description |
|--------|---------|-------------|
| [contrib/jwt](contrib/jwt/) | `go get github.com/go-kruda/kruda/contrib/jwt` | JWT sign, verify, refresh (HS256/384/512, RS256) |
| [contrib/ws](contrib/ws/) | `go get github.com/go-kruda/kruda/contrib/ws` | WebSocket upgrade, RFC 6455 frames, ping/pong |
| [contrib/ratelimit](contrib/ratelimit/) | `go get github.com/go-kruda/kruda/contrib/ratelimit` | Token bucket / sliding window rate limiting |
| [contrib/session](contrib/session/) | `go get github.com/go-kruda/kruda/contrib/session` | Session middleware with pluggable store |
| [contrib/compress](contrib/compress/) | `go get github.com/go-kruda/kruda/contrib/compress` | Response compression (gzip, deflate) |
| [contrib/etag](contrib/etag/) | `go get github.com/go-kruda/kruda/contrib/etag` | ETag response caching |
| [contrib/cache](contrib/cache/) | `go get github.com/go-kruda/kruda/contrib/cache` | Response cache (in-memory, Redis) |
| [contrib/otel](contrib/otel/) | `go get github.com/go-kruda/kruda/contrib/otel` | OpenTelemetry tracing |
| [contrib/prometheus](contrib/prometheus/) | `go get github.com/go-kruda/kruda/contrib/prometheus` | Prometheus metrics |
| [contrib/swagger](contrib/swagger/) | `go get github.com/go-kruda/kruda/contrib/swagger` | Swagger UI HTML |
| [contrib/observability](contrib/observability/) | `go get github.com/go-kruda/kruda/contrib/observability` | Turnkey OpenTelemetry — one-call `Enable()`: tracing + RED metrics + K8s probes + `/metrics` |

### Pre-release Checklist

Run vulnerability scan before every release:

```bash
# Install govulncheck (one-time)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan root module — covers core + Wing (since v1.2.0 Wing lives in core)
govulncheck ./...
```

Wing's runtime implementation lives in the core module (`wing_*.go` at the repo root), so scanning the root module covers it. The legacy `transport/wing` shim was removed in v1.3.0.

Kruda core has minimal external dependencies (Sonic JSON, fasthttp). Use `kruda_stdjson` build tag to switch to stdlib JSON. Upgrade to the latest Go patch release for security fixes.

**Minimum Go patch release for zero known standard-library vulnerabilities:** go1.25.11+ or go1.26.4+

## Contributing

Contributions welcome. Please read the [Contributing Guide](CONTRIBUTING.md) before submitting a PR.

## License

[MIT](LICENSE)
