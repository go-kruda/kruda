# Contrib Modules

Kruda ships optional contrib modules as separate Go modules with their own `go.mod`. Install only what you need.

## Available Modules

| Module | Install | Description |
|--------|---------|-------------|
| [jwt](https://github.com/go-kruda/kruda/tree/main/contrib/jwt) | `go get github.com/go-kruda/kruda/contrib/jwt` | JWT sign, verify, refresh (HS256/384/512, RS256) |
| [ws](https://github.com/go-kruda/kruda/tree/main/contrib/ws) | `go get github.com/go-kruda/kruda/contrib/ws` | WebSocket upgrade, RFC 6455 frames, ping/pong |
| [ratelimit](https://github.com/go-kruda/kruda/tree/main/contrib/ratelimit) | `go get github.com/go-kruda/kruda/contrib/ratelimit` | Token bucket / sliding window rate limiting |
| [session](https://github.com/go-kruda/kruda/tree/main/contrib/session) | `go get github.com/go-kruda/kruda/contrib/session` | Session middleware with pluggable store |
| [compress](https://github.com/go-kruda/kruda/tree/main/contrib/compress) | `go get github.com/go-kruda/kruda/contrib/compress` | Response compression (gzip, deflate) |
| [etag](https://github.com/go-kruda/kruda/tree/main/contrib/etag) | `go get github.com/go-kruda/kruda/contrib/etag` | ETag response caching |
| [cache](https://github.com/go-kruda/kruda/tree/main/contrib/cache) | `go get github.com/go-kruda/kruda/contrib/cache` | Response cache (in-memory, Redis) |
| [otel](https://github.com/go-kruda/kruda/tree/main/contrib/otel) | `go get github.com/go-kruda/kruda/contrib/otel` | OpenTelemetry tracing |
| [prometheus](https://github.com/go-kruda/kruda/tree/main/contrib/prometheus) | `go get github.com/go-kruda/kruda/contrib/prometheus` | Prometheus metrics |
| [swagger](https://github.com/go-kruda/kruda/tree/main/contrib/swagger) | `go get github.com/go-kruda/kruda/contrib/swagger` | Swagger UI HTML |

## Example: JWT Authentication

```go
import (
    "os"
    "github.com/go-kruda/kruda"
    "github.com/go-kruda/kruda/contrib/jwt"
)

// Protect a route group
api := app.Group("/api").Guard(jwt.New(jwt.Config{
    Secret: []byte(os.Getenv("JWT_SECRET")),
}))

api.Get("/profile", func(c *kruda.Ctx) error {
    claims := jwt.ClaimsFrom(c)
    return c.JSON(claims)
})
```

## Example: Rate Limiting

```go
import (
    "time"
    "github.com/go-kruda/kruda/contrib/ratelimit"
)

// 100 requests per minute per IP
app.Use(ratelimit.New(ratelimit.Config{
    Max:    100,
    Window: time.Minute,
}))

// Stricter limit on auth endpoints
app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))
```

## Example: Response Cache

```go
import (
    "time"
    "github.com/go-kruda/kruda/contrib/cache"
)

app.Get("/users", handler, cache.New(cache.Config{
    Expiration: 5 * time.Minute,
}))
```
