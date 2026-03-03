# FAQ

## Why Kruda?

Kruda fills the gap between raw performance frameworks (Fiber, Fasthttp) and type-safe but verbose frameworks (standard net/http). It gives you:

- Type-safe handlers with `C[T]` — body, params, and query in one struct, validated at compile time
- Auto-everything — validation, error mapping, OpenAPI generation, CRUD endpoints
- Pluggable transport — fasthttp for Linux performance, net/http for compatibility
- Built-in DI — no codegen, no reflection, just `Give` and `Use`
- 60-70% less boilerplate than Gin or Fiber

Think of it as the Go equivalent of tRPC — maximum DX without sacrificing performance.

## How does the DI container work?

Kruda's DI uses Go generics. Services are registered on a `*Container` and resolved in handlers via `*Ctx`:

```go
// Set up container
c := kruda.NewContainer()
c.Give(&UserService{db: connectDB()})

app := kruda.New(kruda.WithContainer(c))

// Resolve in a handler
app.Get("/users", func(c *kruda.Ctx) error {
    svc := kruda.MustResolve[*UserService](c)
    // ...
})
```

Services are singletons by default — registered via `Give` and resolved via `MustResolve` in handlers. Group related services into modules with the `Module` interface.

See the [DI Container guide](/guide/di-container) for details.

## fasthttp vs net/http — which transport should I use?

| | fasthttp | net/http |
|---|---------|----------|
| Platform | Linux only | All platforms |
| Performance | Higher throughput, lower latency | Good, standard Go performance |
| HTTP/2 | Not supported | Supported via TLS |
| Maturity | Production-ready (used by ByteDance) | Go stdlib, battle-tested |

Kruda auto-selects the transport:
- Linux with fasthttp available → fasthttp
- Windows/macOS or fasthttp unavailable → net/http

You can force a specific transport via configuration.

## How do I test my Kruda app?

Use the built-in `TestClient` for in-memory testing — no server startup, no port conflicts:

```go
func TestHello(t *testing.T) {
    app := kruda.New()
    app.Get("/hello", func(c *kruda.Ctx) error {
        return c.Text("Hello!")
    })

    tc := kruda.NewTestClient(app)
    resp, _ := tc.Get("/hello")

    if resp.StatusCode() != 200 {
        t.Fatalf("expected 200, got %d", resp.StatusCode())
    }
}
```

See the [Test Client API](/api/test-client) for the full builder API.

## Do I need CGO for Sonic JSON?

Sonic uses SIMD instructions and requires CGO on some platforms. If CGO is not available, Kruda automatically falls back to `encoding/json`.

To force stdlib JSON (no CGO required):

```bash
go build -tags kruda_stdjson ./...
go test -tags kruda_stdjson ./...
```

## What Go version do I need?

Go 1.25 or later is required for generic type aliases used by `C[T]` typed handlers and stdlib security fixes. Go 1.26+ is recommended for the Green Tea GC and self-referential generics support.

## Is Kruda production-ready?

Kruda is approaching v1.0.0-rc1. The core framework (routing, context, middleware, DI, CRUD, error handling, health checks) is implemented and tested. Security hardening, comprehensive documentation, and CI/CD are part of the Phase 5 production readiness effort.

## How do I contribute?

See the [Contributing Guide](https://github.com/go-kruda/kruda/blob/main/CONTRIBUTING.md) on GitHub. We welcome bug reports, feature requests, documentation improvements, and code contributions.
