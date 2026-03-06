# FAQ

## Why Kruda?

Kruda fills the gap between raw performance frameworks (Fiber, Fasthttp) and type-safe but verbose frameworks (standard net/http). It gives you:

- Type-safe handlers with `C[T]` — body, params, and query in one struct, validated at compile time
- Auto-everything — validation, error mapping, OpenAPI generation, CRUD endpoints
- Pluggable transport — Wing (epoll+eventfd) on Linux, fasthttp, or net/http
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

## Which transport should I use?

| | Wing | fasthttp | net/http |
|---|------|---------|----------|
| Platform | Linux only | All platforms | All platforms |
| Performance | Highest (846K req/s) | High | Good |
| HTTP/2 | No | No | Yes (via TLS) |
| Set-Cookie | Limited (fast path skips) | Yes | Yes |
| SSE / WebSocket | No | No | Yes |
| Default on | Linux | macOS | Windows |

Kruda auto-selects:
- Linux → Wing (epoll+eventfd)
- macOS → fasthttp
- Windows → net/http
- TLS configured → net/http (auto-fallback)

Override with `kruda.Wing()`, `kruda.FastHTTP()`, or `kruda.NetHTTP()`. See [Transport Guide](/guide/transport) for details.

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

Kruda has completed Phases 1-7 (Foundation through TechEmpower Domination). The core framework, type system, performance optimization, ecosystem (DI, CRUD), security hardening, and Wing transport are all implemented and tested. Phase 8 (Launch) is the v1.0.0 release milestone.

## How do I contribute?

See the [Contributing Guide](https://github.com/go-kruda/kruda/blob/main/CONTRIBUTING.md) on GitHub. We welcome bug reports, feature requests, documentation improvements, and code contributions.
