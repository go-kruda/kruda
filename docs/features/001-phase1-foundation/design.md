# Phase 1 — Foundation: Design

> **Spec Reference:** Sections 3-6, 9-10

## Architecture

```
User API: app.Get("/path", handler)
    ↓
App.addRoute() → stores in Router radix tree
    ↓
Listen(":3000") → starts transport (net/http)
    ↓
Request arrives → transport calls App.ServeKruda(w, r)
    ↓
acquireCtx(w, r) ← from sync.Pool
    ↓
buildChain: [global middleware...] + [group middleware...] + [handler]
    ↓
Execute chain: handlers[0](ctx) → ctx.Next() → handlers[1]... → handler
    ↓
Handler returns error? → resolveError → JSON error response
    ↓
releaseCtx(ctx) → back to sync.Pool
```

## Component Design

### 1. App struct (`kruda.go`)

```go
type App struct {
    config     Config
    router     *Router
    middleware []HandlerFunc
    hooks      Hooks
    errorMap   map[error]ErrorMapping
    transport  Transport
    ctxPool    sync.Pool
}
```

Key decisions:
- App implements `transport.Handler` interface → receives all requests
- sync.Pool for Ctx objects → zero-alloc on hot path
- Transport is pluggable: Phase 1 = net/http, Phase 3 = Netpoll
- Method chaining: all route methods return `*App`

### 2. Router (`router.go`)

```go
type Router struct {
    trees map[string]*node  // method → radix tree root
}

type node struct {
    path     string
    children []*node
    handler  HandlerFunc
    param    string      // parameter name
    wildcard bool
    regex    *regexp.Regexp
    indices  string      // first bytes of children for quick lookup
    handlers []HandlerFunc // full chain (middleware + handler)
}
```

Key decisions:
- Separate radix tree per HTTP method
- `indices` string for O(1) child lookup by first byte
- Params stored in pre-allocated `map[string]string` on Ctx (cleared, not reallocated)
- AOT: `Compile()` called at Listen() time, freezes the tree

### 3. Route Groups (`group.go`)

```go
type Group struct {
    prefix     string
    app        *App
    middleware []HandlerFunc
    parent     *Group  // for nested groups
}
```

Key decisions:
- Groups don't store routes — they delegate to App with full prefix
- `Done()` returns `*App` for chaining back to root
- Guard() is alias for Use() — semantic sugar for auth middleware

### 4. Middleware Chain (`middleware.go`)

```go
func buildChain(globalMW, groupMW []HandlerFunc, handler HandlerFunc) []HandlerFunc {
    chain := make([]HandlerFunc, 0, len(globalMW)+len(groupMW)+1)
    chain = append(chain, globalMW...)
    chain = append(chain, groupMW...)
    chain = append(chain, handler)
    return chain
}
```

Key decisions:
- Chain is pre-built at route registration → no allocation at request time
- c.Next() increments `routeIndex` and calls `handlers[routeIndex]`
- Middleware can short-circuit by not calling `c.Next()`

### 5. Lifecycle Hooks (`lifecycle.go`)

```go
type Hooks struct {
    OnRequest    []HookFunc
    BeforeHandle []HookFunc
    AfterHandle  []HookFunc
    OnResponse   []HookFunc
    OnError      []ErrorHookFunc
    OnShutdown   []func()
}
```

Key decisions:
- Hooks run in order: OnRequest → middleware → BeforeHandle → handler → AfterHandle → OnResponse
- OnError only fires if handler returns error
- OnShutdown fires during graceful shutdown
- Per-route hooks via HookConfig on route registration

### 6. Graceful Shutdown (detailed — NEW)

```go
func (app *App) Listen(addr string) error {
    app.router.Compile()
    go app.transport.ListenAndServe(addr, app)

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    ctx, cancel := context.WithTimeout(context.Background(), app.config.ShutdownTimeout)
    defer cancel()

    app.logger.Info("shutting down", "timeout", app.config.ShutdownTimeout)
    if err := app.transport.Shutdown(ctx); err != nil {
        app.logger.Error("drain failed", "error", err)
    }

    // LIFO order (like defer)
    for i := len(app.hooks.OnShutdown) - 1; i >= 0; i-- {
        app.hooks.OnShutdown[i]()
    }
    app.logger.Info("shutdown complete")
    return nil
}
```

- Default ShutdownTimeout: 10s, `WithShutdownTimeout(30*time.Second)`
- Programmatic: `app.Shutdown(ctx)` for tests

### 7. Request-scoped Logger (NEW)

```go
func (c *Ctx) Log() *slog.Logger {
    if c.logger == nil {
        c.logger = c.app.config.Logger.With(
            "request_id", c.Get("request_id"),
            "method", c.Method(),
            "path", c.Path(),
        )
    }
    return c.logger
}
```

- Lazy-init: only when first called, cached on Ctx
- Reset on releaseCtx: `c.logger = nil`

### 8. Environment Config (NEW)

```go
func WithEnvPrefix(prefix string) Option {
    return func(c *Config) {
        // APP_PORT → c.Port, APP_READ_TIMEOUT → c.ReadTimeout, etc.
    }
}
```

- `os.Getenv()` only — no deps
- CamelCase → SCREAMING_SNAKE_CASE mapping
- Duration/size parsers included

## File Dependencies

```
transport/transport.go  (no deps)
transport/nethttp.go    (depends on transport.go)
map.go                  (no deps)
error.go                (no deps)
config.go               (depends on transport)
context.go              (depends on config, error, transport)
router.go               (depends on context)
middleware.go            (depends on context)
lifecycle.go            (depends on context)
group.go                (depends on router, middleware)
kruda.go                (depends on everything above)
```

## Testing Strategy

Each component should have a `*_test.go`:
- `router_test.go` — route matching, params, wildcards, conflicts
- `context_test.go` — request/response, pooling, safety
- `kruda_test.go` — integration: register routes, send requests, check responses
- `group_test.go` — prefix handling, scoped middleware
- `middleware_test.go` — chain building, Next(), short-circuit
