# Health

Health check endpoint with pluggable health checkers.

## HealthChecker Interface

```go
type HealthChecker interface {
    Name() string
    Check(ctx context.Context) error
}
```

Implement this interface for each service dependency:

```go
type DBHealthChecker struct {
    db *sql.DB
}

func (h *DBHealthChecker) Name() string { return "database" }

func (h *DBHealthChecker) Check(ctx context.Context) error {
    return h.db.PingContext(ctx)
}
```

```go
type RedisHealthChecker struct {
    client *redis.Client
}

func (h *RedisHealthChecker) Name() string { return "redis" }

func (h *RedisHealthChecker) Check(ctx context.Context) error {
    return h.client.Ping(ctx).Err()
}
```

## HealthHandler

```go
func HealthHandler(checkers ...HealthChecker) HandlerFunc
```

Creates a handler that runs all health checkers and returns the aggregate status.

```go
app.Get("/health", kruda.HealthHandler(
    &DBHealthChecker{db: db},
    &RedisHealthChecker{client: rdb},
))
```

### Response Format

All healthy:

```json
{
  "status": "healthy",
  "checks": {
    "database": { "status": "healthy" },
    "redis": { "status": "healthy" }
  }
}
```

HTTP 200

One or more unhealthy:

```json
{
  "status": "unhealthy",
  "checks": {
    "database": { "status": "healthy" },
    "redis": { "status": "unhealthy", "error": "connection refused" }
  }
}
```

HTTP 503

## DI Integration

Register health checkers as DI services:

```go
kruda.Give(app, func() *DBHealthChecker {
    db := kruda.Use[*sql.DB](app)
    return &DBHealthChecker{db: db}
})

app.Get("/health", func(c *kruda.Ctx) error {
    checker := kruda.Use[*DBHealthChecker](c)
    return kruda.HealthHandler(checker)(c)
})
```

## Example

```go
package main

import (
    "context"
    "github.com/go-kruda/kruda"
)

type AppChecker struct{}

func (c *AppChecker) Name() string                        { return "app" }
func (c *AppChecker) Check(ctx context.Context) error     { return nil }

func main() {
    app := kruda.New()

    app.Get("/health", kruda.HealthHandler(&AppChecker{}))

    app.Listen(":3000")
}
```
