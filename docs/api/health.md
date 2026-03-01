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

All healthy (HTTP 200):

```json
{
  "status": "healthy",
  "checks": {
    "database": { "status": "healthy" },
    "redis": { "status": "healthy" }
  }
}
```

One or more unhealthy (HTTP 503):

```json
{
  "status": "unhealthy",
  "checks": {
    "database": { "status": "healthy" },
    "redis": { "status": "unhealthy", "error": "connection refused" }
  }
}
```

## Example

```go
package main

import (
    "context"
    "github.com/go-kruda/kruda"
)

type AppChecker struct{}

func (c *AppChecker) Name() string                    { return "app" }
func (c *AppChecker) Check(ctx context.Context) error { return nil }

func main() {
    app := kruda.New()

    app.Get("/health", kruda.HealthHandler(&AppChecker{}))

    app.Listen(":3000")
}
```
