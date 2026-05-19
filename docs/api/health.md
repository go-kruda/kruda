# Health

Health check endpoint backed by services registered in the app container.

## HealthChecker Interface

```go
type HealthChecker interface {
    Check(ctx context.Context) error
}
```

Kruda discovers `HealthChecker` implementations from `WithContainer(...)`.
Singletons, named singletons, and already-resolved lazy singletons are included.
Checker names are derived from the registered type name, or from the named
registration key for `GiveNamed`.

## HealthHandler

```go
func HealthHandler(opts ...HealthOption) HandlerFunc
```

Creates a handler that runs discovered health checkers in parallel.

```go
c := kruda.NewContainer()
c.Give(&DBHealthChecker{db: db})

app := kruda.New(kruda.WithContainer(c))
app.Get("/health", kruda.HealthHandler())
```

### Options

```go
func WithHealthTimeout(d time.Duration) HealthOption
```

Sets the timeout for all health checks. The default is 5 seconds.

### Response Format

All healthy (HTTP 200):

```json
{
  "status": "ok",
  "checks": {
    "DBHealthChecker": "ok"
  }
}
```

One or more unhealthy (HTTP 503):

```json
{
  "status": "unhealthy",
  "checks": {
    "DBHealthChecker": "connection refused"
  }
}
```

No registered health checkers returns HTTP 200:

```json
{
  "status": "ok",
  "checks": {}
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

func (c *AppChecker) Check(ctx context.Context) error { return nil }

func main() {
    container := kruda.NewContainer()
    container.Give(&AppChecker{})

    app := kruda.New(kruda.WithContainer(container))
    app.Get("/health", kruda.HealthHandler())

    app.Listen(":3000")
}
```
