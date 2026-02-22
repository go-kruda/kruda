# Phase 3 — Netpoll & Performance: Design

> Detailed design to be written when Phase 2 is complete.
> See spec Sections 11, 12 for full implementation details.

## Key Design Points (from spec)

### Netpoll Transport (`transport/netpoll.go`)

```go
type NetpollTransport struct {
    listener       netpoll.Listener  // epoll/kqueue listener
    handler        Handler           // app.ServeKruda
    connPool       sync.Pool         // reuse *connection objects
    config         NetpollConfig
}

type connection struct {
    fd           int          // file descriptor
    buffer       []byte       // reuse parse buffer
    requestPool  sync.Pool    // reuse *http.Request
    responsePool sync.Pool    // reuse response writers
}
```

- Wrap `netpoll.Listen()` to handle connections via event loop
- Connection pooling: reuse buffers + request/response objects
- Graceful shutdown: drainConnections() on SIGINT
- Windows fallback: detect at runtime, switch to net/http

### Transport Auto-selection

```go
func selectTransport(cfg Config) transport.Transport {
    if cfg.Transport != "" {
        // manual override
        if cfg.Transport == "netpoll" {
            return NewNetpollTransport(cfg)
        }
        return NewNetHTTPTransport(cfg)
    }
    // auto-detect
    if runtime.GOOS == "windows" {
        return NewNetHTTPTransport(cfg)
    }
    // Try Netpoll, fall back to net/http on error
    if t, err := NewNetpollTransport(cfg); err == nil {
        return t
    }
    return NewNetHTTPTransport(cfg)
}
```

### Zero-alloc Context Design

```go
// internal/pool/pool.go
type ctxPool struct {
    pool      sync.Pool
    mapPool   sync.Pool  // pre-allocated maps
    strSlice  sync.Pool  // param values
}

func (p *ctxPool) Acquire() *Ctx {
    ctx := p.pool.Get().(*Ctx)
    // Clear, don't reallocate
    for k := range ctx.params {
        delete(ctx.params, k)
    }
    // Maps reused as-is
    return ctx
}
```

- Ctx includes: params map, locals map, headers fixed slots, response bytes buffer
- All allocated once at pool init
- Maps cleared in bulk via range-delete (faster than re-allocation)
- Response buffer: `bytes.Buffer` reused via sync.Pool

### Header Optimization

```go
type Context struct {
    // Fixed-slot headers (most common)
    contentType     string  // Content-Type
    contentLength   int     // Content-Length
    cacheControl    string  // Cache-Control

    // Fallback for custom headers
    customHeaders   http.Header
}

func (c *Ctx) SetHeader(key, value string) {
    switch key {
    case "Content-Type":
        c.contentType = value
    case "Cache-Control":
        c.cacheControl = value
    default:
        c.customHeaders[key] = []string{value}
    }
}
```

### Benchmark Design

```
bench/
├── http_test.go          // simple GET benchmark
├── routing_test.go       // parameterized routes
├── middleware_test.go    // middleware chain overhead
├── json_test.go          // JSON encoding benchmark
├── handlers_test.go      // typed handler overhead
└── compare_test.go       // vs Fiber/Gin/Echo
```

## File Dependencies

```
transport/transport.go      (no deps)
transport/nethttp.go        (depends on transport.go) ✅
transport/netpoll.go        (depends on transport.go)
internal/pool/pool.go       (depends on context.go) [new]
context.go                  (modified: add fixed slots, pool integration)
config.go                   (modified: add Transport field, auto-select logic)
kruda.go                    (modified: call transport selection)
bench/                      (depends on kruda.go) [new]
```

## Testing Strategy

- `transport/netpoll_test.go` — connection handling, graceful shutdown
- `transport/integration_test.go` — test both transports with same app
- `bench/*_test.go` — benchmarks comparing implementations
- Benchmark CI: store baseline, detect regressions >5%
