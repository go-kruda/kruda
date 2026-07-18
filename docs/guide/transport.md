# Transport Selection Guide

Kruda ships with three transports. The default is **Wing** on Linux, **fasthttp** on macOS.

## When to use which

| Feature needed | Transport | Why |
|---------------|-----------|-----|
| Maximum throughput (plaintext, JSON, DB) | **Wing** (default on Linux) | epoll+eventfd, zero-copy, no goroutine-per-conn |
| SSE (Server-Sent Events) | **Wing** or **net/http** | Wing requires the `kruda.Stream` route preset |
| Session cookies / Set-Cookie headers | **Wing** or **net/http** | Wing keeps headers by falling back from its zero-copy fast path when needed |
| HTTP/2, TLS termination | **net/http** | Wing is HTTP/1.1 only |
| WebSocket | **Wing** or **net/http** | Use `ws.HandleFunc`; fasthttp does not support the upgrade |
| Battle-tested HTTP/1.1 (chunked, expect-100) | **fasthttp** | Mature, widely deployed |

## Usage

```go
// Default: Wing on Linux, fasthttp on macOS, net/http on Windows
app := kruda.New()

// Explicit net/http (for direct TLS, HTTP/2, or stdlib compatibility)
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

## Production TLS: terminate in front of Wing {#production-tls-terminate-in-front-of-wing}

Wing does not implement TLS. When `WithTLS` is configured, Kruda automatically
serves through the net/http transport instead — you keep HTTPS and gain HTTP/2,
but you are **no longer running Wing**, so Wing benchmark numbers do not apply to
a direct-TLS deployment.

To keep Wing's performance in production, terminate TLS in front of the app —
nginx, Caddy, HAProxy, or a cloud load balancer — and let Kruda serve plaintext
HTTP/1.1 behind it. This is the standard deployment shape for epoll-based Go
servers. Two things to set:

```go
app := kruda.New(
    kruda.WithTrustProxy(true), // client IPs come from X-Forwarded-For
)
```

and make the proxy strip/overwrite inbound `X-Forwarded-For` so clients cannot
spoof it. If you need TLS directly on the app process (no proxy), use
`kruda.WithTLS(...)` and accept the net/http transport — that is the supported
and honest trade.

## Wing limitations

Wing is optimized for raw throughput. It intentionally skips some HTTP features:

- **Prebuilt static responses bypass middleware** — `StaticText` and `StaticJSON` write route-level prebuilt responses directly. Use normal handlers when a response needs middleware, lifecycle hooks, CORS, cookies, or `WithSecureHeaders()`.
- **Incremental responses need a preset** — plain handlers write fixed `Content-Length` responses. SSE and chunked streaming (`c.SSE()` / `c.Stream()`) work on Wing via the `kruda.Stream` route preset, and WebSocket via `ws.HandleFunc` (the `kruda.Hijack` preset) — but a default route does not stream.
- **No HTTP/2** — Wing speaks HTTP/1.1 only. Use `kruda.NetHTTP()` with TLS for HTTP/2.
- **No chunked transfer encoding** — Wing pre-computes Content-Length.
- **`App.Serve(listener)` re-binds on Wing** — Wing serves one `SO_REUSEPORT` socket per worker, so it adopts the listener's *address* and closes the passed file descriptor rather than serving it. For systemd socket activation or graceful-restart fd-passing, use `kruda.NetHTTP()` (or fasthttp on macOS), which serve the supplied fd directly.

Wing apps can mix normal handlers with streaming and WebSocket routes by applying `kruda.Stream` or registering WebSockets with `ws.HandleFunc`. Choose net/http when you need direct TLS, HTTP/2, chunked request bodies, or exact stdlib transport semantics.
