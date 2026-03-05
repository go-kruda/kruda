# Kruda (ครุฑ)

Type-safe Go web framework with auto-everything.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev)
[![CI](https://github.com/go-kruda/kruda/actions/workflows/test.yml/badge.svg)](https://github.com/go-kruda/kruda/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/go-kruda/kruda/branch/main/graph/badge.svg)](https://codecov.io/gh/go-kruda/kruda)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-kruda/kruda)](https://goreportcard.com/report/github.com/go-kruda/kruda)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## Why Kruda?

- Typed handlers `C[T]` — body + param + query parsed into one struct, validated at compile time
- Auto CRUD — implement `ResourceService[T]`, get 5 REST endpoints
- Built-in DI — optional, no codegen, type-safe generics
- Pluggable transport — Wing (Linux, epoll+eventfd), fasthttp, or net/http
- Zero external deps — core uses only Go stdlib
- Dev mode error page — rich HTML with source code context, like Next.js

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

## Benchmarks

<p align="center">
  <img src="docs/assets/benchmark-chart.png" alt="Kruda vs Fiber vs Actix Web benchmark" width="800">
</p>

Measured with `wrk -t4 -c256 -d5s` on Linux i5-13500 (8P cores), GOGC=400.

| Test | Kruda (Go) | Fiber (Go) | Actix (Rust) | vs Fiber | vs Actix |
|------|--:|--:|--:|--:|--:|
| plaintext | **846,622** | 670,240 | 814,652 | +26% | +4% |
| JSON | **805,124** | 625,839 | 790,362 | +29% | +2% |
| db | **108,468** | 107,450 | 37,373 | +1% | +190% |
| fortunes | 104,144 | **106,623** | 45,078 | -2% | +131% |

Wing transport uses raw `epoll` + `eventfd` on Linux — bypasses both fasthttp and net/http. macOS defaults to fasthttp.

- See [`bench/reproducible/`](bench/reproducible/) for full source code of all 3 frameworks and reproduction steps

## Documentation

Full documentation at [kruda.dev](https://kruda.dev):

- [Getting Started](https://kruda.dev/guide/getting-started)
- [Routing](https://kruda.dev/guide/routing)
- [Typed Handlers](https://kruda.dev/guide/typed-handlers)
- [Middleware](https://kruda.dev/guide/middleware)
- [DI Container](https://kruda.dev/guide/di-container)
- [Error Handling](https://kruda.dev/guide/error-handling)
- [API Reference](https://kruda.dev/api/app)

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
    // Parser limits (Wing transport)
    kruda.WithMaxHeaderCount(100),
    kruda.WithMaxHeaderSize(8192),
)

// Rate limiting — 100 req/min per IP
app.Use(ratelimit.New(ratelimit.Config{
    Max: 100, Window: time.Minute,
    TrustedProxies: []string{"10.0.0.0/8"},
}))

// Stricter limit on auth endpoints
app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))

// JWT authentication on protected routes
api := app.Group("/api", jwt.New(jwt.Config{
    Secret: []byte(os.Getenv("JWT_SECRET")),
}))
```

### Contrib Modules

| Module | Install | Description |
|--------|---------|-------------|
| [contrib/jwt](contrib/jwt/) | `go get github.com/go-kruda/kruda/contrib/jwt` | JWT sign, verify, refresh (HS256/384/512, RS256) |
| [contrib/ws](contrib/ws/) | `go get github.com/go-kruda/kruda/contrib/ws` | WebSocket upgrade, RFC 6455 frames, ping/pong |
| [contrib/ratelimit](contrib/ratelimit/) | `go get github.com/go-kruda/kruda/contrib/ratelimit` | Token bucket / sliding window rate limiting |

### Pre-release Checklist

Run vulnerability scan before every release:

```bash
# Install govulncheck (one-time)
go install golang.org/x/vuln/cmd/govulncheck@latest

# Scan root module
govulncheck ./...

# Scan Wing transport module
cd transport/wing && govulncheck ./...
```

Kruda core has zero external dependencies — all vulnerabilities will be Go stdlib issues. Upgrade to the latest Go patch release to resolve them.

**Minimum Go version for zero stdlib vulnerabilities:** go1.25.7+

## Contributing

Contributions welcome. Please read the [Contributing Guide](CONTRIBUTING.md) before submitting a PR.

## License

[MIT](LICENSE)
