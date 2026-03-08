# Getting Started

## Installation

Requires Go 1.25 or later.

```bash
go get github.com/go-kruda/kruda
```

## Quick Start

Create a `main.go`:

```go
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New()

    app.Get("/", func(c *kruda.Ctx) error {
        return c.JSON(map[string]string{
            "message": "Hello, Kruda!",
        })
    })

    app.Listen(":3000")
}
```

Run it:

```bash
go run main.go
```

Visit `http://localhost:3000` to see the response.

## Configuration

Kruda uses the functional options pattern:

```go
app := kruda.New(
    kruda.WithReadTimeout(30 * time.Second),
    kruda.WithWriteTimeout(30 * time.Second),
    kruda.WithBodyLimit(4 * 1024 * 1024), // 4MB
)
```

See [Config API](/api/config) for all available options.

## Next Steps

- [Routing](/guide/routing) — route registration, parameters, groups
- [Handlers](/guide/handlers) — typed handlers with `C[T]`
- [Middleware](/guide/middleware) — built-in and custom middleware
- [Error Handling](/guide/error-handling) — error mapping and custom handlers
- [DI Container](/guide/di-container) — dependency injection with Give/Use
- [Auto CRUD](/guide/auto-crud) — automatic CRUD endpoints
- [Security](/guide/security) — hardening options and best practices
- [API Reference](/api/app) — complete API documentation
