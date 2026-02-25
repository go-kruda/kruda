# Phase 4 — Ecosystem: Design

> Detailed design to be written when Phase 3 is complete.
> See spec Sections 13-16 for full implementation details.

## Key Design Points (from spec)

### DI Container (`container.go`)

```go
type Container struct {
    singletons map[reflect.Type]any             // resolved singletons
    transients map[reflect.Type]*transientEntry  // transient factories
    lazies     map[reflect.Type]*lazyEntry       // lazy factories (atomic.Bool done)
    named      map[string]any                    // named instances
    initOrder  []any                             // registration/resolution order
    mu         sync.RWMutex
}

func (c *Container) Give(instance any) error {
    t := reflect.TypeOf(instance)
    c.singletons[t] = instance
    c.initOrder = append(c.initOrder, instance)
    return nil
}

// Use is a free function (Go doesn't support generic methods)
func Use[T any](c *Container) (T, error) {
    t := reflect.TypeOf((*T)(nil)).Elem()
    if inst, ok := c.singletons[t]; ok {
        return inst.(T), nil
    }
    // transient → factory per call
    // lazy → factory once, cached, retry on error
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

// discoverHealthCheckers scans singletons, resolved lazies (atomic.Bool),
// and named instances with deduplication.
func HealthHandler(opts ...HealthOption) HandlerFunc {
    // discovers checkers from container, runs in parallel with timeout
    // Response: {status: "ok"|"unhealthy", checks: {name: "ok"|"error msg"}}
}
```

- Interface-based discovery via container
- Parallel execution with timeout
- Response format: `{status: "ok", checks: {service: "ok"}}`

### Auto CRUD (`resource.go`)

```go
type ResourceService[T any, ID comparable] interface {
    List(ctx context.Context, page, limit int) ([]T, int, error)
    Create(ctx context.Context, item T) (T, error)
    Get(ctx context.Context, id ID) (T, error)
    Update(ctx context.Context, id ID, item T) (T, error)
    Delete(ctx context.Context, id ID) error
}

// Uses routeRegistrar interface to deduplicate App/Group registration.
func Resource[T any, ID comparable](app *App, path string, svc ResourceService[T, ID], opts ...ResourceOption) *App
func GroupResource[T any, ID comparable](g *Group, path string, svc ResourceService[T, ID], opts ...ResourceOption) *Group
```

- Detects service methods via reflection
- Auto-wires routes with error handling
- Pagination: query params → method args

### Test Client (`test_client.go`)

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
test_client.go         (depends on kruda.go)
contrib/jwt/           (depends on container.go, context.go) [separate]
contrib/swagger/       (depends on container.go) [separate]
contrib/ratelimit/     (depends on context.go) [separate]
```

## Testing Strategy

- `container_test.go` — Give/Use, circular deps, thread-safety
- `resource_test.go` — auto CRUD, route matching, error handling
- `health_test.go` — health check discovery, parallel execution
- `test_client_test.go` — test client request/response
- Integration: test with real app + DI
