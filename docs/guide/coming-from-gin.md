# Coming from Gin

This guide helps Gin users migrate to Kruda. The API is intentionally similar — most concepts map 1:1.

## Quick Comparison

| Concept | Gin | Kruda |
|---------|-----|-------|
| Create app | `gin.Default()` | `kruda.New()` |
| Context | `*gin.Context` | `*kruda.Ctx` |
| Route | `r.GET("/path", handler)` | `app.Get("/path", handler)` |
| JSON response | `c.JSON(200, obj)` | `c.JSON(obj)` |
| Status code | `c.JSON(404, obj)` | `c.Status(404).JSON(obj)` |
| Query param | `c.Query("key")` | `c.Query("key")` |
| Path param | `c.Param("id")` | `c.Param("id")` |
| Middleware | `r.Use(middleware)` | `app.Use(middleware)` |
| Group | `r.Group("/api")` | `app.Group("/api")` |
| Bind JSON | `c.ShouldBindJSON(&obj)` | Typed handler `C[T]` |

## Hello World

**Gin:**
```go
package main

import "github.com/gin-gonic/gin"

func main() {
    r := gin.Default()
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })
    r.Run(":3000")
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

Key differences: Kruda handlers return `error` (no silent failures), and status code defaults to 200.

## Error Handling

**Gin** — manual per-handler:
```go
r.GET("/user/:id", func(c *gin.Context) {
    user, err := findUser(c.Param("id"))
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, user)
})
```

**Kruda** — return errors, handle centrally:
```go
app.Get("/user/:id", func(c *kruda.Ctx) error {
    user, err := findUser(c.Param("id"))
    if err != nil {
        return err // centralized error handler formats the response
    }
    return c.JSON(user)
})

// Optional: customize error mapping
app.OnError(func(c *kruda.Ctx, err error) {
    // map domain errors to HTTP responses
})
```

## Typed Handlers (Kruda exclusive)

Gin requires manual binding + validation. Kruda uses generics:

**Gin:**
```go
type CreateUser struct {
    Name  string `json:"name" binding:"required"`
    Email string `json:"email" binding:"required,email"`
}

r.POST("/users", func(c *gin.Context) {
    var input CreateUser
    if err := c.ShouldBindJSON(&input); err != nil {
        c.JSON(422, gin.H{"error": err.Error()})
        return
    }
    // use input...
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

No manual binding. No manual error checking. Validation errors return structured 422 automatically.

## Middleware

**Gin:**
```go
r.Use(gin.Logger())
r.Use(gin.Recovery())
r.Use(cors.Default())
```

**Kruda:**
```go
import "github.com/go-kruda/kruda/middleware"

app.Use(middleware.Logger())
app.Use(middleware.Recovery())
app.Use(middleware.CORS())
```

Built-in middleware lives in the `middleware` package — no extra `gin-contrib` packages needed.

## Route Groups

**Gin:**
```go
api := r.Group("/api/v1")
api.Use(authMiddleware())
{
    api.GET("/users", listUsers)
    api.POST("/users", createUser)
}
```

**Kruda:**
```go
app.Group("/api/v1").
    Guard(authMiddleware()).
    Get("/users", listUsers).
    Post("/users", createUser).
    Done()
```

## What You Gain

- **3x faster** than Gin in benchmarks (416ns vs 1318ns per request)
- **Type-safe handlers** — no more manual `ShouldBindJSON`
- **Pluggable transport** — Wing (default on Linux), fasthttp (default on macOS), or net/http (HTTP/2)
- **Zero boilerplate** — 60-70% less code than Gin
- **Auto OpenAPI** — generated from your typed handlers
- **Built-in DI** — optional dependency injection without codegen
