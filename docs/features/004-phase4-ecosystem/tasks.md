# Phase 4 — Ecosystem: Tasks

> Component-level task breakdown for AI/developer to execute.
> Check off as completed. Each task = one focused unit of work.

## Component Status

| # | Component | File(s) | Status | Assignee |
|---|-----------|---------|--------|----------|
| 1 | DI Container | `container.go` | 🔴 Todo | - |
| 2 | DI Modules | `module.go` | 🔴 Todo | - |
| 3 | Health Check | `health.go` | 🔴 Todo | - |
| 4 | Auto CRUD Resource | `resource.go` | 🔴 Todo | - |
| 5 | Test Client | `test.go` | 🔴 Todo | - |
| 6 | Container Tests | `container_test.go` | 🔴 Todo | - |
| 7 | Resource Tests | `resource_test.go` | 🔴 Todo | - |
| 8 | Health Check Tests | `health_test.go` | 🔴 Todo | - |
| 9 | Test Client Tests | `test_test.go` | 🔴 Todo | - |
| 10 | Inject Middleware | `middleware/inject.go` | 🔴 Todo | - |

---

## Detailed Task Breakdown

### Task 1: DI Container (`container.go`)
- [ ] Define `Container` struct: singletons map, transients map, lazies map, named map, mutex
- [ ] Implement `NewContainer() *Container`
- [ ] Implement `Give(instance any) error` — register singleton by type
- [ ] Implement `GiveAs(instance any, iface any) error` — register singleton as interface
- [ ] Implement `GiveTransient(fn func() (any, error)) error` — register transient factory
- [ ] Implement `GiveLazy(iface any, fn func() (any, error)) error` — register lazy singleton
- [ ] Implement `GiveNamed(name string, instance any) error` — register by name
- [ ] Implement `Use(ctx *Ctx, iface any) (any, error)` — resolve dependency (non-generic version)
- [ ] Implement generic `Use[T any](ctx *Ctx) (T, error)` — type-safe resolution
- [ ] Implement circular dependency detection
- [ ] Thread-safe access: all maps protected by mutex
- [ ] Clear error messages for resolution failures
- [ ] Support struct field injection via `kruda:"inject"` tag (reflect-based)

### Task 2: DI Modules (`module.go`)
- [ ] Define `Module` interface: `Install(*Container) error`
- [ ] Implement `App.Module(Module) *App` method (returns app for chaining)
- [ ] Create example modules: LoggerModule, DatabaseModule, CacheModule
- [ ] Ensure modules can be installed in any order (no ordering assumptions)
- [ ] Document how to create custom modules

### Task 3: Health Check (`health.go`)
- [ ] Define `HealthChecker` interface: `Check(ctx context.Context) error`
- [ ] Implement `HealthHandler(c *Ctx) error` function
  - [ ] Discover all `HealthChecker` instances in container
  - [ ] Run checks in parallel with configurable timeout (default 5s)
  - [ ] Aggregate results
- [ ] Return health response: `{status: "ok"|"degraded"|"error", checks: {...}}`
- [ ] Handle individual check failures (mark as degraded, not error)
- [ ] Add to app routing: `app.Get("/health", kruda.HealthHandler())`
- [ ] Tests: mock health checkers, timeout behavior

### Task 4: Auto CRUD Resource (`resource.go`)
- [ ] Define `ResourceService` interface (or detect methods dynamically)
  - [ ] `List(ctx context.Context, page, limit int) ([]any, error)`
  - [ ] `Create(ctx context.Context, item any) (any, error)`
  - [ ] `Get(ctx context.Context, id string) (any, error)`
  - [ ] `Update(ctx context.Context, id string, item any) (any, error)`
  - [ ] `Delete(ctx context.Context, id string) error`
- [ ] Implement `App.Resource(path string, svc ResourceService) *App`
  - [ ] Auto-wires: GET /path, POST /path, GET /path/:id, PUT /path/:id, DELETE /path/:id
  - [ ] Extract ID from path, pagination from query
  - [ ] Handle errors: 404 on not found, 400 on bad request, 500 on server error
- [ ] Implement handlers: listHandler, createHandler, getHandler, updateHandler, deleteHandler
- [ ] Pagination: parse ?page=N&limit=M, default page=1, limit=20
- [ ] Return structured responses with status codes
- [ ] Support optional hooks: OnListFilter, OnCreateValidate, OnUpdateValidate

### Task 5: Test Client (`test.go`)
- [ ] Define `TestClient` struct wrapping app
- [ ] Implement `NewTestClient(app *App) *TestClient`
- [ ] Implement `TestClient.Request(method, path string, opts ...Option) (*TestResponse, error)`
  - [ ] Set method, path, headers, body
  - [ ] Call app.ServeKruda directly (no HTTP)
  - [ ] Capture response via httptest.ResponseRecorder
- [ ] Implement convenience methods: `Get`, `Post`, `Put`, `Delete`, `Patch`
- [ ] Implement `TestResponse` struct: StatusCode, Headers, Body, BodyJSON()
- [ ] Implement request builder: `client.Post("/path").Header("X-Key", "val").Body(obj).Do()`
- [ ] Support JSON body auto-marshaling
- [ ] Support form-encoded body
- [ ] Document: no actual HTTP, direct handler invocation

### Task 6: Container Tests (`container_test.go`)
- [ ] Test `Give()` — singleton registration and resolution
- [ ] Test `GiveAs()` — interface-based registration
- [ ] Test `GiveTransient()` — factory functions
- [ ] Test `GiveLazy()` — lazy initialization
- [ ] Test `GiveNamed()` — named instances
- [ ] Test `Use[T]()` — type-safe resolution with generics
- [ ] Test type resolution: exact match, interface match
- [ ] Test circular dependency detection (panic with clear message)
- [ ] Test thread-safety: concurrent Give/Use operations
- [ ] Test not-found error: resolving non-registered type
- [ ] Test struct field injection via `kruda:"inject"` tag

### Task 7: Resource Tests (`resource_test.go`)
- [ ] Create mock ResourceService implementation
- [ ] Test `App.Resource()` — registers all 5 routes
- [ ] Test GET /path — calls List, parses pagination
- [ ] Test POST /path — calls Create, returns 201
- [ ] Test GET /path/:id — calls Get, returns 200 or 404
- [ ] Test PUT /path/:id — calls Update, returns 200 or 404
- [ ] Test DELETE /path/:id — calls Delete, returns 204 or 404
- [ ] Test error handling: validation errors (400), not found (404), server errors (500)
- [ ] Test pagination: default values, custom page/limit
- [ ] Test JSON response marshaling

### Task 8: Health Check Tests (`health_test.go`)
- [ ] Create mock HealthChecker implementations
- [ ] Test single health check (ok)
- [ ] Test multiple health checks (ok)
- [ ] Test one check failing (degraded status)
- [ ] Test all checks failing (error status)
- [ ] Test timeout: checks that exceed timeout
- [ ] Test parallel execution: multiple checks run concurrently
- [ ] Test response format: status + checks detail
- [ ] Test registration: app auto-discovers HealthChecker in container

### Task 9: Test Client Tests (`test_test.go`)
- [ ] Test GET request
- [ ] Test POST with JSON body
- [ ] Test PUT with form data
- [ ] Test DELETE request
- [ ] Test response status code
- [ ] Test response headers parsing
- [ ] Test response body JSON parsing
- [ ] Test request builder chaining
- [ ] Test no actual HTTP (direct handler)

### Task 10: Inject Middleware (`middleware/inject.go`)
- [ ] Create middleware that resolves dependencies into Ctx.locals
- [ ] Usage: `app.Use(container.InjectMiddleware())`
- [ ] Middleware: before handler, resolve registered services
- [ ] Store in ctx.locals[type] for handler access
- [ ] Error handling: resolution failures return 500
- [ ] Performance: cache resolution metadata to avoid reflection per request

---

## Execution Order (Recommended)

Best order for AI to implement, respecting dependencies:

1. `container.go` — foundation for DI
2. `module.go` — DI modules (depends on container)
3. `test.go` — test client (depends on kruda)
4. `middleware/inject.go` — inject middleware (depends on container)
5. `resource.go` — auto CRUD (depends on container)
6. `health.go` — health checks (depends on container)
7. `container_test.go` — test container
8. `test_test.go` — test client tests
9. `resource_test.go` — test auto CRUD
10. `health_test.go` — test health checks
11. `contrib/ws/` — WebSocket support
12. `contrib/otel/` — OpenTelemetry middleware
13. `contrib/prometheus/` — Prometheus metrics
14. `contrib/cache/` — Response caching
15. `contrib/swagger/` — Swagger UI
16. `contrib/jwt/`, `contrib/ratelimit/`, `contrib/csrf/`, `contrib/compress/`
17. Integration tests: test all components together
