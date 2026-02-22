# Phase 1 — Foundation: Tasks

> Component-level task breakdown for AI/developer to execute.
> Check off as completed. Each task = one focused unit of work.

## Component Status

| # | Component | File(s) | Status | Assignee |
|---|-----------|---------|--------|----------|
| 1 | Config | `config.go` | ✅ Done | - |
| 2 | Transport Interface | `transport/transport.go` | ✅ Done | - |
| 3 | Transport net/http | `transport/nethttp.go` | ✅ Done | - |
| 4 | Error Handling | `error.go` | ✅ Done | - |
| 5 | Context | `context.go` | ✅ Done | - |
| 6 | Map Alias | `map.go` | ✅ Done | - |
| 7 | Router | `router.go` | ✅ Done | - |
| 8 | Middleware Chain | `middleware.go` | ✅ Done | - |
| 9 | Lifecycle Hooks | `lifecycle.go` | ✅ Done | - |
| 10 | Route Groups | `group.go` | ✅ Done | - |
| 11 | App Core | `kruda.go` | ✅ Done | - |
| 12 | Built-in Logger | `middleware/logger.go` | ✅ Done | - |
| 13 | Built-in Recovery | `middleware/recovery.go` | ✅ Done | - |
| 14 | Built-in CORS | `middleware/cors.go` | ✅ Done | - |
| 15 | Built-in RequestID | `middleware/requestid.go` | ✅ Done | - |
| 16 | Typed Handler Stub | `handler.go` | ✅ Done | - |
| 17 | Bind Stub | `bind.go` | ✅ Done | - |
| 18 | Bytes Conv | `internal/bytesconv/bytesconv.go` | ✅ Done | - |
| 19 | Hello Example | `examples/hello/main.go` | ✅ Done | - |
| 20 | Project Files | `Makefile`, `LICENSE`, `README.md` | ✅ Done | - |
| 21 | Request-scoped Logger | `context.go` (add `Log()`) | ✅ Done | - |
| 22 | Env Config | `config.go` (add `WithEnvPrefix`) | ✅ Done | - |
| 23 | Timeout Middleware | `middleware/timeout.go` | ✅ Done | - |

---

## Detailed Task Breakdown

### Task 7: Router (`router.go`)
- [x] Define `Router` struct with `map[string]*node` (method → tree)
- [x] Define `node` struct: path, children, handler, param, wildcard, regex, indices
- [x] Implement `addRoute(method, path, handler)` — inserts into radix tree
- [x] Implement `find(method, path)` — returns handler + params, zero-alloc
- [x] Support static routes: `/users`, `/users/settings`
- [x] Support param routes: `/users/:id` → params["id"]
- [x] Support multi-param: `/users/:id/posts/:postId`
- [x] Support wildcard: `/files/*filepath` → params["filepath"]
- [x] Support regex constraint: `/users/:id<[0-9]+>` → validates before matching
- [x] Support optional param: `/users/:id?` → matches with or without
- [x] Detect route conflicts at registration time (panic with clear message)
- [x] `Compile()` freezes tree (called by Listen)
- [x] Write `router_test.go` — cover all pattern types

### Task 8: Middleware Chain (`middleware.go`)
- [x] Define `MiddlewareFunc` type (alias for HandlerFunc)
- [x] Implement `buildChain(globalMW, groupMW, handler)` → pre-built slice
- [x] Ensure chain is allocated once at route registration
- [x] Write `middleware_test.go` — chain order, Next(), short-circuit

### Task 9: Lifecycle Hooks (`lifecycle.go`)
- [x] Define `Hooks` struct: OnRequest, BeforeHandle, AfterHandle, OnResponse, OnError, OnShutdown
- [x] Define `HookFunc = func(c *Ctx) error`
- [x] Define `ErrorHookFunc = func(c *Ctx, err error)`
- [x] Define `HookConfig` for per-route hooks
- [x] Implement hook execution in request flow
- [x] Helper: `WithHooks(HookConfig)` for route registration
- [x] Write `lifecycle_test.go`

### Task 10: Route Groups (`group.go`)
- [x] Define `Group` struct: prefix, app, middleware, parent
- [x] Implement: `Get`, `Post`, `Put`, `Delete`, `Patch` on Group
- [x] Implement `Use(middleware ...HandlerFunc)` → scoped middleware
- [x] Implement `Guard(middleware ...HandlerFunc)` → alias for Use
- [x] Implement `Group(prefix)` → nested group
- [x] Implement `Done()` → returns *App for chaining
- [x] Path joining: handle trailing/leading slashes correctly
- [x] Write `group_test.go`

### Task 11: App Core (`kruda.go`)
- [x] Define `App` struct with all fields
- [x] Implement `New(opts ...Option) *App` — init config, router, pool, transport
- [x] Implement `ServeKruda(w, r)` — the main request handler
  - [x] acquireCtx from pool
  - [x] Find route in router
  - [x] Build handler chain (or use pre-built)
  - [x] Execute chain
  - [x] Handle errors with resolveError
  - [x] releaseCtx back to pool
- [x] Implement route methods: `Get`, `Post`, `Put`, `Delete`, `Patch`, `Options`, `Head`, `All`
- [x] Implement `Use(middleware ...HandlerFunc)` → global middleware
- [x] Implement `Group(prefix string) *Group`
- [x] Implement `Listen(addr string) error`
  - [x] Compile router
  - [x] Start transport
  - [x] Signal handling: SIGINT, SIGTERM
  - [x] Graceful shutdown with timeout
  - [x] Run OnShutdown hooks
- [x] Implement 404 handler (route not found)
- [x] Implement 405 handler (method not allowed)
- [x] Write `kruda_test.go` — integration tests

### Task 12-15: Built-in Middleware
- [x] `middleware/logger.go` — slog structured logging (method, path, status, latency, ip)
- [x] `middleware/recovery.go` — recover panics, log stack trace, return 500
- [x] `middleware/cors.go` — CORSConfig with defaults, preflight handling
- [x] `middleware/requestid.go` — crypto/rand UUID, set X-Request-ID header
- [x] Write tests for each middleware

### Task 16: Typed Handler Stub (`handler.go`)
- [x] Define `C[T any]` struct embedding `*Ctx` with `In T`
- [x] Stub `Get[In, Out]`, `Post[In, Out]`, `Put[In, Out]`, `Delete[In, Out]`
- [x] Stub functions should work (JSON bind → handler → JSON response)
- [x] Full parser/validator deferred to Phase 2

### Task 17: Bind Stub (`bind.go`)
- [x] Stub `bindInput[T any](c *Ctx) (T, error)` — simple JSON-only bind for now
- [x] Full struct tag-based binding deferred to Phase 2

### Task 18: Bytes Conv (`internal/bytesconv/bytesconv.go`)
- [x] `UnsafeString(b []byte) string` — zero-copy
- [x] `UnsafeBytes(s string) []byte` — zero-copy
- [x] Write test + benchmark

### Task 19: Hello Example (`examples/hello/main.go`)
- [x] Working example using app.Get, app.Post, groups, middleware
- [x] Must compile and run: `go run examples/hello/main.go`

### Task 20: Project Files
- [x] `Makefile` — build, test, bench, vet, lint targets
- [x] `LICENSE` — MIT, Copyright 2026 Tiger
- [x] `README.md` — install, quick start, features, benchmarks placeholder

---

## Execution Order (Recommended)

Best order for AI to implement, respecting dependencies:

1. `internal/bytesconv/bytesconv.go` (no deps)
2. `middleware.go` (depends on context.go ✅)
3. `lifecycle.go` (depends on context.go ✅)
4. `router.go` (depends on context.go ✅)
5. `group.go` (depends on router, middleware)
6. `kruda.go` (depends on everything above)
7. `handler.go` + `bind.go` (stubs, depends on kruda.go)
8. `middleware/logger.go`, `recovery.go`, `cors.go`, `requestid.go`
9. `examples/hello/main.go`
10. `Makefile`, `LICENSE`, `README.md`
11. Run `go build ./...` and `go vet ./...` → fix any issues
