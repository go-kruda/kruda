# Coming from Fiber

Fiber users will feel right at home — Kruda has the same Express-like API, but with Wing transport (epoll+eventfd) that's 26% faster.

## Quick Comparison

| Concept | Fiber | Kruda |
|---------|-------|-------|
| Create app | `fiber.New()` | `kruda.New()` |
| Context | `*fiber.Ctx` | `*kruda.Ctx` |
| Route | `app.Get("/path", handler)` | `app.Get("/path", handler)` |
| JSON response | `c.JSON(obj)` | `c.JSON(obj)` |
| Status code | `c.Status(404).JSON(obj)` | `c.Status(404).JSON(obj)` |
| Query param | `c.Query("key")` | `c.Query("key")` |
| Path param | `c.Params("id")` | `c.Param("id")` |
| Body parser | `c.BodyParser(&obj)` | Typed handler `C[T]` |
| Middleware | `app.Use(middleware)` | `app.Use(middleware)` |
| Group | `app.Group("/api")` | `app.Group("/api")` |
| Listen | `app.Listen(":3000")` | `app.Listen(":3000")` |

## Hello World

**Fiber:**
```go
package main

import "github.com/gofiber/fiber/v2"

func main() {
    app := fiber.New()
    app.Get("/", func(c *fiber.Ctx) error {
        return c.JSON(fiber.Map{"message": "hello"})
    })
    app.Listen(":3000")
}
```

**Kruda:**
```go
package main

import "github.com/go-kruda/kruda"

func main() {
    app := kruda.New()
    app.Get("/", func(c *kruda.Ctx) error {
        return c.JSON(kruda.Map{"message": "hello"})
    })
    app.Listen(":3000")
}
```

Almost identical! The main difference is the import path.

## Key Differences

### 1. No Context Reuse Bugs

Fiber's `*fiber.Ctx` is pooled and reused — storing it across goroutines causes data races:

```go
// Fiber — DANGEROUS
app.Get("/", func(c *fiber.Ctx) error {
    go func() {
        // c is reused by another request — data race!
        fmt.Println(c.Query("name"))
    }()
    return nil
})
```

Kruda copies all strings on access — safe by default:

```go
// Kruda — SAFE
app.Get("/", func(c *kruda.Ctx) error {
    name := c.Query("name") // string is copied, safe to use anywhere
    go func() {
        fmt.Println(name) // no data race
    }()
    return nil
})
```

### 2. Typed Handlers Replace BodyParser

**Fiber:**
```go
type CreateUser struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

app.Post("/users", func(c *fiber.Ctx) error {
    var input CreateUser
    if err := c.BodyParser(&input); err != nil {
        return c.Status(422).JSON(fiber.Map{"error": err.Error()})
    }
    // manually validate...
    return c.JSON(createUser(input))
})
```

**Kruda:**
```go
type CreateUser struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

app.Post("/users", func(c *kruda.Ctx) error {
    var input CreateUser
    if err := c.Bind(&input); err != nil {
        return err
    }
    return c.JSON(createUser(input))
})
// Or use typed handlers for compile-time safety:
// kruda.Post[CreateUser, User](app, "/users", handler)
```

### 3. Pluggable Transport

Fiber is locked to fasthttp (no HTTP/2, no TLS 1.3). Kruda lets you switch:

```go
// Default on Linux: Wing transport (epoll+eventfd, fastest)
// Default on macOS: fasthttp
app := kruda.New()

// Explicit fasthttp:
app := kruda.New(kruda.FastHTTP())

// Need HTTP/2 or TLS? Switch to net/http:
app := kruda.New(kruda.NetHTTP())

// Auto-select: TLS config → automatically uses net/http
app := kruda.New(kruda.WithTLS("cert.pem", "key.pem"))
```

### 4. Route Groups with Method Chaining

**Fiber:**
```go
api := app.Group("/api/v1")
api.Use(jwtMiddleware)
api.Get("/users", listUsers)
api.Post("/users", createUser)
```

**Kruda:**
```go
app.Group("/api/v1").
    Guard(jwtMiddleware).
    Get("/users", listUsers).
    Post("/users", createUser).
    Done()
```

## Middleware Migration

| Fiber | Kruda |
|-------|-------|
| `fiber.Logger()` (built-in) | `middleware.Logger()` (built-in) |
| `fiber.Recover()` | `middleware.Recovery()` (built-in) |
| `fiber.CORS()` (built-in) | `middleware.CORS()` (built-in) |
| `fiber.RequestID()` | `middleware.RequestID()` (built-in) |
| `fiber.Timeout()` | `middleware.Timeout()` (built-in) |
| `jwtware.New()` (contrib) | `jwt.New()` (contrib) |
| `websocket.New()` (contrib) | `ws.New()` (contrib) |
| `limiter.New()` (contrib) | `ratelimit.New()` (contrib) |
| `compress.New()` (contrib) | `compress.New()` (contrib) |

## What You Gain

- **Wing transport** — raw epoll+eventfd on Linux, 26% faster than Fiber (846K vs 670K req/s plaintext)
- **Type-safe handlers** — no more `BodyParser` + manual validation
- **No context reuse bugs** — safe string copies by default
- **HTTP/2 support** — switch to net/http when needed
- **Auto OpenAPI** — generated from typed handlers
- **Built-in DI** — optional dependency injection
- **Auto CRUD** — `kruda.Resource[Product, string](app, "/products", service)` generates full REST API
