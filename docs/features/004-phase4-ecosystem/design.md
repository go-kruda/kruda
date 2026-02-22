# Phase 4 — Ecosystem: Design

> Detailed design to be written when Phase 3 is complete.
> See spec Sections 13-16 for full implementation details.

## Key Design Points (from spec)

### DI Container (`container.go`)

```go
type Container struct {
    singletons map[reflect.Type]any           // resolved singletons
    transients map[reflect.Type]func() any    // transient factories
    lazies     map[reflect.Type]func() any    // lazy factories
    named      map[string]any                 // named instances
    mu         sync.RWMutex
}

func (c *Container) Give(instance any) error {
    t := reflect.TypeOf(instance)
    c.singletons[t] = instance
    return nil
}

func (c *Container) Use[T any](ctx *Ctx) (T, error) {
    var zero T
    t := reflect.TypeOf(zero)
    if inst, ok := c.singletons[t]; ok {
        return inst.(T), nil
    }
    // lazy init, caching
}
```

- Type-keyed storage via `reflect.Type`
- Three lifetime models: singleton, transient, lazy singleton
- Thread-safe: `sync.RWMutex`
- Inject middleware: wraps handlers to resolve dependencies

### DI Modules (`module.go`)

```go
type Module interface {
    Install(c *Container) error
}

// In App
func (app *App) Module(m Module) *App {
    m.Install(app.container)
    return app
}
```

- Simple interface: Install method
- Modules register their services + hooks
- Order: modules installed in order given

### Health Check (`health.go`)

```go
type HealthChecker interface {
    Check(ctx context.Context) error
}

func HealthHandler(c *Ctx) error {
    checks := discoverHealthCheckers(c.app.container)
    results := runInParallel(checks, timeout)
    return c.JSON(healthResponse)
}
```

- Interface-based discovery via container
- Parallel execution with timeout
- Response format: `{status: "ok", checks: {service: "ok"}}`

### Auto CRUD (`resource.go`)

```go
type ResourceService interface {
    List(ctx context.Context, page, limit int) ([]any, error)
    Create(ctx context.Context, item any) (any, error)
    Get(ctx context.Context, id string) (any, error)
    Update(ctx context.Context, id string, item any) (any, error)
    Delete(ctx context.Context, id string) error
}

func (app *App) Resource(path string, svc ResourceService) *App {
    app.Get(path, listHandler(svc))
    app.Post(path, createHandler(svc))
    app.Get(path+"/:id", getHandler(svc))
    app.Put(path+"/:id", updateHandler(svc))
    app.Delete(path+"/:id", deleteHandler(svc))
    return app
}
```

- Detects service methods via reflection
- Auto-wires routes with error handling
- Pagination: query params → method args

### Test Client (`test.go`)

```go
type TestClient struct {
    app *App
    // no actual HTTP, direct to app.ServeKruda
}

func (tc *TestClient) Post(path string, body any) (*TestResponse, error) {
    req := &http.Request{...}
    w := httptest.NewRecorder()
    tc.app.ServeKruda(w, req)
    return parseResponse(w), nil
}
```

- Use `httptest.ResponseRecorder`
- No actual network I/O
- Request builder with fluent API
- Response parser with JSON helper

## File Dependencies

```
container.go            (depends on context.go)
module.go              (depends on container.go)
health.go              (depends on container.go, context.go)
resource.go            (depends on container.go, context.go)
test.go                (depends on kruda.go)
contrib/jwt/           (depends on container.go, context.go) [separate]
contrib/swagger/       (depends on container.go) [separate]
contrib/ratelimit/     (depends on context.go) [separate]
```

## Testing Strategy

- `container_test.go` — Give/Use, circular deps, thread-safety
- `resource_test.go` — auto CRUD, route matching, error handling
- `health_test.go` — health check discovery, parallel execution
- `test_test.go` — test client request/response
- Integration: test with real app + DI
