# Phase 1 — Foundation: Requirements

> **Goal:** Basic API working on net/http. A working "hello world" framework.
> **Timeline:** Week 1-3
> **Spec Reference:** Sections 3, 4, 5, 6, 9, 10

## Milestone

```go
app := kruda.New()
app.Get("/ping", func(c *kruda.Ctx) error {
    return c.JSON(kruda.Map{"pong": true})
})
app.Listen(":3000")
```

## Functional Requirements

### FR-1: Application Core (`kruda.go`)
- App struct encapsulates all framework state
- `New(opts ...Option) *App` creates app with functional options
- Route registration methods: `Get`, `Post`, `Put`, `Delete`, `Patch`, `Options`, `Head`, `All`
- All methods return `*App` for method chaining
- `Listen(addr string) error` starts the server
- Graceful shutdown on SIGINT/SIGTERM with configurable timeout

### FR-2: Configuration (`config.go`) ✅ Done
- Config struct with timeouts (read/write/idle), limits (body/header)
- SecurityConfig for security headers
- Functional options pattern: `WithReadTimeout()`, `WithBodyLimit()`, etc.
- Sensible defaults (30s timeout, 4MB body, security headers on)

### FR-3: Transport Layer (`transport/`) ✅ Done
- Pluggable Transport interface: `ListenAndServe`, `Shutdown`
- Request abstraction: `Method()`, `Path()`, `Header()`, `Body()`, `QueryParam()`
- ResponseWriter abstraction: `WriteHeader()`, `Header()`, `Write()`
- net/http implementation with body limiting

### FR-4: Context (`context.go`) ✅ Done
- Pooled via sync.Pool, zero allocation per request
- All string values are safe copies (no fasthttp-style reuse bugs)
- Headers map lazy-parsed and cached
- Request API: `Method()`, `Path()`, `Param()`, `ParamInt()`, `Query()`, `QueryInt()`, `Header()`, `Cookie()`, `IP()`, `BodyBytes()`, `BodyString()`, `Bind()`
- Response API: `Status()`, `JSON()`, `Text()`, `HTML()`, `File()`, `Stream()`, `NoContent()`, `Redirect()`, `SetHeader()`, `SetCookie()`
- Locals: `Set()`, `Get()` for request-scoped values
- Flow control: `Next()` for middleware chaining, `Latency()`
- Context: `Context()`, `SetContext()` for stdlib compatibility
- Security headers auto-applied on every response

### FR-5: Router (`router.go`)
- Radix tree data structure for efficient route matching
- Zero allocation during matching (pre-allocated param slots)
- Route patterns supported:
  - Static: `/users`
  - Parameterized: `/users/:id`
  - Multi-param: `/users/:id/posts/:postId`
  - Wildcard: `/files/*filepath`
  - Regex constraint: `/users/:id<[0-9]+>`
  - Optional param: `/users/:id?`
- AOT-compiled: tree built at startup, immutable at runtime
- Method-based trees (GET, POST, etc.)

### FR-6: Route Groups (`group.go`)
- Group struct with prefix, middleware, hooks
- Methods: `Get`, `Post`, `Put`, `Delete`, `Use`, `Guard` (alias for Use)
- Nested groups: `Group()` within a group
- `Done()` returns parent for chaining
- Scoped middleware: only applies to routes within the group

### FR-7: Middleware Chain (`middleware.go`)
- `MiddlewareFunc = func(c *Ctx) error`
- `Use()` registers global middleware
- `buildChain()` combines: global middleware → group middleware → route handler
- Ordered execution with `c.Next()`

### FR-8: Error Handling (`error.go`) ✅ Done
- KrudaError with HTTP status + message + wrapped error
- `MapError(err, status, message)` for auto error→HTTP mapping
- Convenience constructors: BadRequest, NotFound, Unauthorized, etc.
- Global error handler via `WithErrorHandler()`

### FR-9: Lifecycle Hooks (`lifecycle.go`)
- Hooks struct: OnRequest, BeforeHandle, AfterHandle, OnResponse, OnError, OnShutdown
- Per-route hooks via HookConfig
- Execution order: OnRequest → middleware → BeforeHandle → handler → AfterHandle → OnResponse

### FR-10: Map alias (`map.go`) ✅ Done
- `Map = map[string]any` for convenience JSON responses

### FR-11: Graceful Shutdown Design (NEW — production requirement)
- `app.Listen()` catches SIGINT/SIGTERM
- Drain in-flight connections before shutting down
- Configurable timeout: `WithShutdownTimeout(10 * time.Second)` (default 10s)
- OnShutdown hooks fire after drain but before process exits
- `app.OnShutdown(func() { db.Close() })` for cleanup registration
- `app.Shutdown(ctx)` for programmatic shutdown (e.g. in tests)
- Log shutdown progress: "shutting down...", "draining N connections...", "shutdown complete"

### FR-12: Request ID → slog Integration (NEW — observability)
- RequestID middleware sets `X-Request-ID` header (already planned in FR-7)
- **NEW:** Also inject request_id into slog context via `c.Log()`
- Every log line within a request automatically includes request_id:
```go
c.Log().Info("user created", "id", user.ID)
// output: level=INFO msg="user created" id=123 request_id=abc-456 method=GET path=/users
```
- `c.Log()` returns `*slog.Logger` with pre-set attributes: request_id, method, path
- Uses `slog.With()` — zero allocation after first call per request

### FR-13: Environment-based Config (NEW — DX improvement)
```go
app := kruda.New(kruda.WithEnvPrefix("APP"))
// reads: APP_PORT, APP_READ_TIMEOUT, APP_BODY_LIMIT, etc.
```
- Mapping: `ReadTimeout` → `APP_READ_TIMEOUT`, `BodyLimit` → `APP_BODY_LIMIT`
- Duration parsing: `30s`, `5m`, `1h`
- Size parsing: `4MB`, `10KB` for limits
- Environment overrides config struct values (env wins)
- Optional: no overhead if user doesn't call `WithEnvPrefix()`

### FR-14: Timeout Middleware (NEW — per-route timeout)
```go
app.Post("/slow-endpoint", kruda.Timeout(30*time.Second), handler)
```
- Uses `context.WithTimeout` — handler's `c.Context()` carries the deadline
- On timeout: returns 503 Service Unavailable
- File: `middleware/timeout.go`

## Non-Functional Requirements

- **NFR-1:** Zero external dependencies (stdlib only)
- **NFR-2:** All exported types/functions have doc comments
- **NFR-3:** Code compiles with `go build ./...` and passes `go vet ./...`
- **NFR-4:** Go 1.24+ required (for generic type aliases)
- **NFR-5:** Follow Go standard conventions (gofmt)
