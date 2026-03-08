# Coming from Echo

Echo and Kruda share similar design philosophies — both are minimal, high-performance, and use `error`-returning handlers.

## Quick Comparison

| Concept | Echo | Kruda |
|---------|------|-------|
| Create app | `echo.New()` | `kruda.New()` |
| Context | `echo.Context` (interface) | `*kruda.Ctx` (concrete) |
| Route | `e.GET("/path", handler)` | `app.Get("/path", handler)` |
| JSON response | `c.JSON(200, obj)` | `c.JSON(obj)` |
| Status code | `c.JSON(404, obj)` | `c.Status(404).JSON(obj)` |
| Query param | `c.QueryParam("key")` | `c.Query("key")` |
| Path param | `c.Param("id")` | `c.Param("id")` |
| Bind | `c.Bind(&obj)` | Typed handler `C[T]` |
| Middleware | `e.Use(middleware)` | `app.Use(middleware)` |
| Group | `e.Group("/api")` | `app.Group("/api")` |
| Start | `e.Start(":3000")` | `app.Listen(":3000")` |

## Hello World

**Echo:**
```go
package main

import (
    "net/http"
    "github.com/labstack/echo/v4"
)

func main() {
    e := echo.New()
    e.GET("/", func(c echo.Context) error {
        return c.JSON(http.StatusOK, map[string]string{
            "message": "hello",
        })
    })
    e.Start(":3000")
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

Key differences: no `http.StatusOK` needed (200 is default), concrete `*Ctx` instead of interface.

## Key Differences

### 1. Concrete Context vs Interface

Echo uses `echo.Context` interface — powerful for mocking but adds overhead:

```go
// Echo — interface dispatch on every method call
func handler(c echo.Context) error {
    return c.JSON(200, data)
}
```

Kruda uses concrete `*kruda.Ctx` — direct method calls, zero interface overhead:

```go
// Kruda — direct method calls, inlinable
func handler(c *kruda.Ctx) error {
    return c.JSON(data)
}
```

### 2. Typed Handlers Replace Bind

**Echo:**
```go
type CreateUser struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

e.POST("/users", func(c echo.Context) error {
    var input CreateUser
    if err := c.Bind(&input); err != nil {
        return echo.NewHTTPError(422, err.Error())
    }
    if err := c.Validate(input); err != nil {
        return err
    }
    return c.JSON(201, createUser(input))
})

// Plus: register validator globally
e.Validator = &CustomValidator{validator: validator.New()}
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
    return c.Status(201).JSON(createUser(input))
})
// Or use typed handlers for compile-time safety:
// kruda.Post[CreateUser, User](app, "/users", handler)
```

No separate bind + validate steps. No global validator registration. Validation is built into the type system.

### 3. Pluggable Transport

Echo uses net/http exclusively. Kruda offers both:

```go
// Wing (default on Linux) — epoll+eventfd, 846K req/s
app := kruda.New()

// fasthttp — broad compatibility
app := kruda.New(kruda.FastHTTP())

// net/http — same as Echo, but with Kruda's ergonomics
app := kruda.New(kruda.NetHTTP())
```

### 4. Error Handling

**Echo:**
```go
// Custom HTTP error handler
e.HTTPErrorHandler = func(err error, c echo.Context) {
    he, ok := err.(*echo.HTTPError)
    if ok {
        c.JSON(he.Code, he.Message)
    } else {
        c.JSON(500, map[string]string{"error": err.Error()})
    }
}
```

**Kruda:**
```go
// Error mapping — cleaner, supports domain errors
app.MapError(ErrNotFound, 404, "resource not found")
app.MapError(ErrUnauthorized, 401, "unauthorized")

// Or custom handler
app.OnError(func(c *kruda.Ctx, err error) {
    // full control over error responses
})
```

### 5. Route Groups with Guards

**Echo:**
```go
api := e.Group("/api/v1")
api.Use(jwtMiddleware)
api.GET("/users", listUsers)
api.POST("/users", createUser)
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

| Echo | Kruda |
|------|-------|
| `middleware.Logger()` | `middleware.Logger()` (built-in) |
| `middleware.Recover()` | `middleware.Recovery()` (built-in) |
| `middleware.CORS()` | `middleware.CORS()` (built-in) |
| `middleware.RequestID()` | `middleware.RequestID()` (built-in) |
| `middleware.TimeoutWithConfig()` | `middleware.Timeout()` (built-in) |
| `middleware.RateLimiter()` | `ratelimit.New()` (contrib) |
| `middleware.GzipWithConfig()` | `compress.New()` (contrib) |
| `echojwt.WithConfig()` | `jwt.New()` (contrib) |

## What You Gain

- **846K req/s** — Wing transport (epoll+eventfd) by default on Linux
- **Type-safe handlers** — no more bind + validate dance
- **Concrete context** — no interface overhead, better inlining
- **Pluggable transport** — Wing, fasthttp, or net/http, auto-selected
- **Auto OpenAPI** — generated from `C[T]` types
- **Auto CRUD** — `kruda.Resource[Product, string](app, "/products", service)` generates full REST
- **Built-in DI** — optional, no codegen needed
