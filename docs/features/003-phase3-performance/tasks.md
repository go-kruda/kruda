# Phase 3 — Netpoll & Performance: Tasks

> Component-level task breakdown for AI/developer to execute.
> Check off as completed. Each task = one focused unit of work.

## Component Status

| # | Component | File(s) | Status | Assignee |
|---|-----------|---------|--------|----------|
| 1 | Netpoll Transport | `transport/netpoll.go` | 🔴 Todo | - |
| 2 | Transport Auto-selection | `config.go`, `kruda.go` | 🔴 Todo | - |
| 3 | Zero-alloc Pool | `internal/pool/pool.go` | 🔴 Todo | - |
| 4 | Context Optimization | `context.go` | 🔴 Todo | - |
| 5 | Header Optimization | `context.go`, `transport/netpoll.go` | 🔴 Todo | - |
| 6 | HTTP/1.1 Parser | `transport/netpoll.go` (or vendored) | 🔴 Todo | - |
| 7 | Connection Pooling | `transport/netpoll.go` | 🔴 Todo | - |
| 8 | Graceful Shutdown (Netpoll) | `transport/netpoll.go`, `kruda.go` | 🔴 Todo | - |
| 9 | Windows Fallback | `config.go`, `transport/netpoll.go` | 🔴 Todo | - |
| 10 | Benchmark Suite | `bench/`, `bench/*_test.go` | 🔴 Todo | - |
| 11 | Transport Tests | `transport/netpoll_test.go` | 🔴 Todo | - |
| 12 | Integration Tests | `transport/integration_test.go` | 🔴 Todo | - |

---

## Detailed Task Breakdown

### Task 1: Netpoll Transport (`transport/netpoll.go`)
- [ ] Define `NetpollTransport` struct with listener, handler, connection pool
- [ ] Implement `Listen(addr string) error` — create netpoll listener, accept connections
- [ ] Implement `Serve(handler Handler)` — pass requests to app.ServeKruda via event loop
- [ ] Implement `Shutdown(ctx context.Context)` — close listener, wait for active connections
- [ ] Connection handling: read HTTP request, parse, call handler, write response
- [ ] Support HTTP/1.1 (1.0 optional)
- [ ] Support Keep-Alive connections with configurable timeout
- [ ] Handle incomplete requests gracefully
- [ ] Handle malformed HTTP gracefully (400 Bad Request)
- [ ] Respect Config timeouts (read, write, idle)

### Task 2: Transport Auto-selection (`config.go`, `kruda.go`)
- [ ] Add `Transport` field to Config struct (string: "auto", "netpoll", "nethttp")
- [ ] Add `WithTransport(name string)` functional option
- [ ] Implement `selectTransport(cfg Config) transport.Transport` function
- [ ] Logic: detect GOOS, allow override, fallback on error
- [ ] Handle environment variable: `KRUDA_TRANSPORT` override
- [ ] Update `App.Listen()` to call `selectTransport()` before starting
- [ ] Log which transport is selected (debug level)
- [ ] Return clear error if transport not supported on OS

### Task 3: Zero-alloc Pool (`internal/pool/pool.go`)
- [ ] Define `ContextPool` struct managing Ctx + maps + buffers
- [ ] Implement `New(size int) *ContextPool` — pre-allocate pool with N contexts
- [ ] Implement `Acquire() *Ctx` — get context from pool, clear maps
- [ ] Implement `Release(ctx *Ctx)` — return context to pool, clear state
- [ ] Pre-allocate all internal maps at init (params, locals)
- [ ] Implement efficient clear: range-delete over maps
- [ ] Pre-allocate route params slice for max 32 params
- [ ] Pre-allocate response buffer (bytes.Buffer) for each context

### Task 4: Context Optimization (`context.go`)
- [ ] Modify Ctx struct: add fixed-slot fields for common response headers
  - [ ] `contentType string`
  - [ ] `contentLength int`
  - [ ] `cacheControl string`
  - [ ] `vary string`
  - [ ] `etag string`
- [ ] Modify `SetHeader()` / `Header().Set()` to use fixed slots for common headers
- [ ] Modify response writing to batch header output (no per-header syscall)
- [ ] Modify map initialization: pre-allocate via ContextPool
- [ ] Update `Header()` to return lazy-initialized map for custom headers

### Task 5: Header Optimization (`context.go`, `transport/`)
- [ ] Implement fixed-slot header writing: optimize order (common first)
- [ ] Implement batch header encoding into buffer
- [ ] Benchmark: header set + write vs raw bytes copy
- [ ] Update both transports to use optimized header format
- [ ] Add `c.HasHeader(name)` convenience method
- [ ] Add `c.GetHeader(name)` convenience method (reads both slots + custom)

### Task 6: HTTP/1.1 Parser
- [ ] Research: use net/http's parser (wrap stdlib), or vendor netpoll's
- [ ] If wrapping stdlib: ensure zero-copy where possible
- [ ] If vendoring: document source + license
- [ ] Support: method, path, query, headers, body reading
- [ ] Validation: reject invalid requests with 400
- [ ] Support chunked encoding (if needed for HTTP/1.1)

### Task 7: Connection Pooling (`transport/netpoll.go`)
- [ ] Define `connection` struct: fd, read buffer, request parser state
- [ ] Implement connection pool via sync.Pool
- [ ] Reuse buffers across requests on same connection
- [ ] Clear state on release (prevent data leaks)
- [ ] Handle connection reuse limits (max requests per connection)
- [ ] Implement connection timeout (idle, read, write)

### Task 8: Graceful Shutdown (Netpoll) (`transport/netpoll.go`, `kruda.go`)
- [ ] Implement `Shutdown(ctx context.Context) error` for NetpollTransport
- [ ] Stop accepting new connections immediately
- [ ] Wait for active connections to finish (up to ctx deadline)
- [ ] Force-close remaining connections after timeout
- [ ] Log shutdown progress (debug)
- [ ] Test with in-flight requests

### Task 9: Windows Fallback (`config.go`, `transport/netpoll.go`)
- [ ] Detect Windows via `runtime.GOOS == "windows"`
- [ ] Automatically use net/http on Windows (no Netpoll available)
- [ ] Allow manual override: `WithTransport("netpoll")` on Windows → warn + fallback
- [ ] Test on both Windows + Unix CI
- [ ] Document platform support in README

### Task 10: Benchmark Suite (`bench/`)
- [ ] Create `bench/` directory with benchmark tests
- [ ] Implement `BenchmarkGET` — simple GET request, single route
- [ ] Implement `BenchmarkPOST` — POST with JSON body
- [ ] Implement `BenchmarkParamRoute` — `/users/:id` parameterized
- [ ] Implement `BenchmarkMiddlewareChain` — measure middleware overhead
- [ ] Implement `BenchmarkJSONEncode` — JSON response encoding
- [ ] Implement `BenchmarkTypedHandler` — measure generic handler overhead
- [ ] Run: `go test -bench=. -benchmem ./bench/`
- [ ] Document results in README (throughput, latency, allocs)
- [ ] Add baseline tracking for CI (detect regressions)

### Task 11: Transport Tests (`transport/netpoll_test.go`)
- [ ] Test connection accept + request handling
- [ ] Test graceful shutdown (existing requests complete)
- [ ] Test connection timeout
- [ ] Test keep-alive behavior
- [ ] Test malformed request handling (400)
- [ ] Test incomplete request timeout
- [ ] Benchmark: netpoll vs net/http latency + throughput

### Task 12: Integration Tests (`transport/integration_test.go`)
- [ ] Run same app on both transports
- [ ] Send requests, verify identical responses
- [ ] Test route matching, params, middleware, error handling
- [ ] Test graceful shutdown on both
- [ ] Benchmark: compare performance side-by-side
- [ ] Test environment variable override: `KRUDA_TRANSPORT=nethttp`

---

## Execution Order (Recommended)

Best order for AI to implement, respecting dependencies:

1. `internal/pool/pool.go` — context pooling (foundation)
2. `context.go` — integrate pool, add fixed-slot headers
3. `transport/netpoll.go` — Netpoll implementation (4 days)
4. `config.go` → add Transport field + WithTransport()
5. `kruda.go` → integrate selectTransport() in Listen()
6. `transport/netpoll_test.go` — test Netpoll transport
7. `transport/integration_test.go` — test both transports
8. `bench/*_test.go` — benchmark suite
9. Run benchmarks, verify performance goals met
10. Document results in README

---

## Performance Goals

- **Throughput:** >200K req/sec (single core, GET request)
- **Latency P99:** <5ms at 50K req/sec load
- **Memory/req:** <1KB allocation per request (lower than Phase 1)
- **Comparison:** within 10% of Fiber/Hertz
