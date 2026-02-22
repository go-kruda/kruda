# Phase 4 — Ecosystem: Requirements

> **Goal:** Auto CRUD, DI, error mapping make Kruda a complete platform
> **Timeline:** Week 9-12
> **Spec Reference:** Sections 13, 14, 15, 16
> **Depends on:** Phase 1-3 complete

## Milestone

```go
// Dependency Injection
container := kruda.NewContainer()
container.Give(userService)
container.GiveLazy(database, initDB)

app := kruda.New(kruda.WithContainer(container)).
    Use(container.InjectMiddleware())

// Auto CRUD from service interface
app.Resource("/users", userService)  // GET all, GET by ID, POST, PUT, DELETE auto-wired

// Health check discovery
app.Get("/health", kruda.HealthHandler())  // auto-detects HealthChecker interfaces

// Error mapping
kruda.MapError(app, sql.ErrNoRows, 404, "User not found")
```

## Components

| # | Component | File(s) | Est. Days | Priority | Status |
|---|-----------|---------|-----------|----------|--------|
| 1 | DI Container | `container.go` | 4 | 🔴 | 🔴 |
| 2 | DI Modules | `module.go` | 1 | 🔴 | 🔴 |
| 3 | Health Check | `health.go` | 1 | 🔴 | 🔴 |
| 4 | Auto CRUD | `resource.go` | 4 | 🔴 | 🔴 |
| 5 | Built-in Middleware | `middleware/*.go` | 2 | 🔴 | ✅ (Phase 1) |
| 6 | Test Helpers | `test.go` | 2 | 🔴 | 🔴 |
| 7 | Contrib Modules | `contrib/jwt/`, `contrib/swagger/`, etc. | 4 | 🟡 | 🔴 |

## Key Requirements

### Dependency Injection (`container.go`)

> **CRITICAL DESIGN PRINCIPLE:** DI is **100% optional**. Apps work perfectly without it.
> Go community dislikes forced "magic" (unlike Spring). Kruda DI has no codegen (unlike Wire)
> and no struct tags required (unlike FX). Users can always `new(MyService)` manually.

```go
// WITHOUT DI — works perfectly fine:
svc := &UserService{db: myDB}
app.Get("/users", func(c *kruda.Ctx) error { ... })

// WITH DI — opt-in convenience:
container := kruda.NewContainer()
container.Give(userService)
app := kruda.New(kruda.WithContainer(container))
```

```go
type Container interface {
    Give(instance any) error                            // singleton
    GiveAs(instance any, iface any) error               // singleton with interface
    GiveTransient(fn func() (any, error)) error         // transient factory
    GiveLazy(iface any, fn func() (any, error)) error   // lazy singleton
    GiveNamed(name string, instance any) error          // named instances
    Use(ctx *Ctx, iface any) (any, error)               // resolve in handler
}
```

- Type-safe resolution via `container.Use[T](ctx)`
- Circular dependency detection
- Lifecycle hooks: OnInit, OnShutdown
- Inject middleware: `container.InjectMiddleware()`
- Support struct field tagging: `kruda:"inject"`
- **IMPORTANT:** No DI import unless user explicitly uses `NewContainer()`

### DI Modules (`module.go`)

```go
type Module interface {
    Install(c Container) error
}

// Usage
app := kruda.New().
    Module(NewJWTModule(secret)).
    Module(NewDatabaseModule(connStr))
```

- Modular DI registration
- Each module registers its own services
- Encapsulation: modules don't expose internals
- Built-in modules: Logger, Database, Cache, etc.

### Health Check (`health.go`)

```go
type HealthChecker interface {
    Check(ctx context.Context) error
}

// Auto-discovery
app.Get("/health", kruda.HealthHandler())  // discovers all HealthChecker implementations
```

- Interface-based discovery
- Parallel health checks with timeout
- Structured response: status, checks detail
- Graceful: service starts even if health check fails

### Auto CRUD (`resource.go`)

```go
type ResourceService interface {
    List(ctx context.Context, page int, limit int) ([]any, error)
    Create(ctx context.Context, item any) (any, error)
    Get(ctx context.Context, id string) (any, error)
    Update(ctx context.Context, id string, item any) (any, error)
    Delete(ctx context.Context, id string) error
}

// Usage
app.Resource("/users", userService)  // auto-wires GET, POST, PUT, DELETE, GET/:id
```

- Detect service interface methods
- Auto-wire: GET /users, POST /users, GET /users/:id, PUT /users/:id, DELETE /users/:id
- Optional hooks: OnListFilter, OnCreateValidate, etc.
- Pagination: ?page=1&limit=20 on List
- Structured errors: 404 for not found, 400 for validation, etc.

### Test Helpers (`test.go`)

```go
client := kruda.NewTestClient(app)
resp, err := client.Post("/users", kruda.Map{"name": "John"})
assert.Equal(t, 201, resp.StatusCode)
```

- In-memory test client (no HTTP)
- Request builder: method, path, headers, body
- Response parser: status, headers, JSON body
- Cookie handling
- Session management helpers

### Built-in Middleware

From Phase 1 (already done):
- `middleware/logger.go` — structured logging
- `middleware/recovery.go` — panic recovery
- `middleware/cors.go` — CORS handling
- `middleware/requestid.go` — request ID generation

### Contrib Modules

Separate Go modules in `contrib/` directory (each has own `go.mod` to isolate dependencies):

- `contrib/jwt/` — JWT authentication (imports `golang-jwt/jwt`)
- `contrib/swagger/` — Swagger UI HTML serving (static files, serves `/docs/*`)
- `contrib/ratelimit/` — Rate limiting middleware (token bucket, in-memory or Redis)
- `contrib/csrf/` — CSRF protection middleware
- `contrib/compress/` — Response compression (gzip, brotli)
- `contrib/cache/` — Response caching middleware (in-memory, configurable TTL)
- `contrib/ws/` — WebSocket support (NEW — wraps `nhooyr.io/websocket` or `gorilla/websocket`)
- `contrib/otel/` — OpenTelemetry tracing middleware (NEW — imports `go.opentelemetry.io/otel`)
- `contrib/prometheus/` — Prometheus metrics endpoint (NEW — imports `prometheus/client_golang`)
- `contrib/typegen/` — TypeScript client generation from OpenAPI spec (NEW — inspired by Elysia Eden Treaty, post-launch)

**WebSocket (`contrib/ws/`) design:**
```go
import "github.com/go-kruda/kruda/contrib/ws"

app.Get("/ws", ws.Handler(func(conn *ws.Conn) error {
    for {
        msg, err := conn.Read()
        if err != nil { return err }
        conn.Write(ws.TextMessage, msg)
    }
}))
```
- Wraps underlying WebSocket library
- Clean API: `conn.Read()`, `conn.Write()`, `conn.Close()`
- Supports both text and binary messages
- Ping/pong handling built-in

**OpenTelemetry (`contrib/otel/`) design:**
```go
import "github.com/go-kruda/kruda/contrib/otel"

app.Use(otel.Middleware(tracerProvider))
// Auto: creates span per request, propagates trace context, records attributes
```

**Prometheus (`contrib/prometheus/`) design:**
```go
import "github.com/go-kruda/kruda/contrib/prometheus"

app.Use(prometheus.Middleware())
app.Get("/metrics", prometheus.Handler())
// Auto: http_requests_total, http_request_duration_seconds, http_requests_in_flight
```

**Response Cache (`contrib/cache/`) design:**
```go
import "github.com/go-kruda/kruda/contrib/cache"

app.Get("/products", cache.Middleware(5*time.Minute), handler)
// Caches response body + headers, serves from cache on hit
// Options: TTL, key function, store (memory/redis), bypass header
```

## Non-Functional Requirements

- **NFR-1:** No breaking API changes from Phase 1-3
- **NFR-2:** DI type-safe: compile-time checks where possible
- **NFR-3:** All DI/resource code has doc comments
- **NFR-4:** Test helpers work for both net/http and Netpoll transports
- **NFR-5:** Container is thread-safe
