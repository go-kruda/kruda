# Middleware

Middleware wraps handlers to add cross-cutting behavior. In Kruda, `MiddlewareFunc` is a type alias for `HandlerFunc` — they are interchangeable.

## Built-in Middleware

### Logger

Logs request method, path, status, and latency:

```go
import "github.com/go-kruda/kruda/middleware"

app.Use(middleware.Logger())
```

### Recovery

Recovers from panics and returns a 500 error:

```go
app.Use(middleware.Recovery())
```

### CORS

Configures Cross-Origin Resource Sharing:

```go
app.Use(middleware.CORS(middleware.CORSConfig{
    AllowOrigins: []string{"https://example.com"},
    AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders: []string{"Content-Type", "Authorization"},
}))
```

### RequestID

Adds a unique request ID to each request:

```go
app.Use(middleware.RequestID())
// Access via c.Header("X-Request-ID")
```

### Timeout

Enforces a per-request timeout:

```go
app.Use(middleware.Timeout(5 * time.Second))
```

## Custom Middleware

Write middleware as a function that calls `c.Next()`:

```go
func AuthMiddleware(c *kruda.Ctx) error {
    token := c.Header("Authorization")
    if token == "" {
        return c.Status(401).JSON(map[string]string{"error": "unauthorized"})
    }

    // validate token...

    return c.Next()
}

app.Use(AuthMiddleware)
```

## Middleware Ordering

Middleware executes in registration order. The first registered middleware runs first on the request path and last on the response path:

```go
app.Use(middleware.Logger())    // 1st: logs request
app.Use(middleware.Recovery())  // 2nd: catches panics
app.Use(AuthMiddleware)         // 3rd: checks auth
```

Request flow: `Logger → Recovery → Auth → Handler → Auth → Recovery → Logger`

## Group-Level Middleware

Apply middleware to specific route groups:

```go
// Public routes — no auth
app.Get("/health", healthHandler)

// Protected routes — auth required
api := app.Group("/api", AuthMiddleware)
api.Get("/users", listUsers)
api.Get("/profile", getProfile)

// Admin routes — auth + admin check
admin := api.Group("/admin", AdminMiddleware)
admin.Get("/stats", getStats)
```

## Skipping Middleware

Return early from middleware to skip the handler:

```go
func RateLimiter(c *kruda.Ctx) error {
    if isRateLimited(c) {
        return c.Status(429).JSON(map[string]string{"error": "too many requests"})
    }
    return c.Next()
}
```
