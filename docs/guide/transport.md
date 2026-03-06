# Transport Selection Guide

Kruda ships with three transports. The default is **Wing** on Linux, **fasthttp** on macOS.

## When to use which

| Feature needed | Transport | Why |
|---------------|-----------|-----|
| Maximum throughput (plaintext, JSON, DB) | **Wing** (default on Linux) | epoll+eventfd, zero-copy, no goroutine-per-conn |
| SSE (Server-Sent Events) | **net/http** | SSE requires `http.Flusher` streaming |
| Session cookies / Set-Cookie headers | **net/http** | Wing's fast path skips custom headers |
| HTTP/2, TLS termination | **net/http** | Wing is HTTP/1.1 only |
| WebSocket | **net/http** | WebSocket upgrade needs full HTTP semantics |
| Battle-tested HTTP/1.1 (chunked, expect-100) | **fasthttp** | Mature, widely deployed |

## Usage

```go
// Default: Wing on Linux, fasthttp on macOS, net/http on Windows
app := kruda.New()

// Explicit net/http (for SSE, sessions, HTTP/2, TLS)
app := kruda.New(kruda.NetHTTP())

// Explicit fasthttp
app := kruda.New(kruda.FastHTTP())

// Explicit Wing
app := kruda.New(kruda.Wing())
```

## Environment override

```bash
KRUDA_TRANSPORT=nethttp  ./myapp   # force net/http
KRUDA_TRANSPORT=wing     ./myapp   # force Wing
KRUDA_TRANSPORT=fasthttp ./myapp   # force fasthttp
```

## Automatic fallback

- Wing + TLS config → auto-falls back to net/http (for HTTP/2)
- Wing on Windows → auto-falls back to net/http
- Unsupported OS → defaults to fasthttp (macOS) or net/http (Windows)

## Wing limitations

Wing is optimized for raw throughput. It intentionally skips some HTTP features:

- **No `Set-Cookie` in JSON fast path** — Wing's JSON response builder bypasses custom headers for speed. Use `kruda.NetHTTP()` if your routes need cookies.
- **No `http.Flusher`** — SSE streaming requires flushing, which Wing doesn't support.
- **No HTTP/2** — Wing speaks HTTP/1.1 only. Use `kruda.NetHTTP()` with TLS for HTTP/2.
- **No chunked transfer encoding** — Wing pre-computes Content-Length.

For apps that mix high-throughput API routes with session/SSE routes, run two instances or use net/http for the full app.
