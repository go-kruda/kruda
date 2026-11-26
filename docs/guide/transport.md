# Transport Selection Guide

Kruda ships with three transports. The default is **Wing** on Linux, **fasthttp** on macOS.

## When to use which

| Feature needed | Transport | Why |
|---------------|-----------|-----|
| Maximum throughput (plaintext, JSON, DB) | **Wing** (default on Linux) | epoll+eventfd, zero-copy, no goroutine-per-conn |
| Fixed-length media segments with HTTP/1.1 connection reuse | **Wing** on Linux | Opt in per route with `kruda.Stream.With(kruda.StreamKeepAlive())` |
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

An explicit `WithTransport(custom)` is never replaced. With `WithTLS`,
`App.Listen` requires that custom transport to implement
`transport.TLSServer` and otherwise returns an error before binding. A caller
that pre-wraps a listener with TLS can continue to pass it to `App.Serve`.

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
    kruda.WithTrustProxy(true),
    kruda.WithTrustedProxies("10.0.0.2"), // exact IP of the reverse proxy
)
```

and make the proxy strip/overwrite inbound `X-Forwarded-For` so clients cannot
spoof it. If you need TLS directly on the app process (no proxy), use
`kruda.WithTLS(...)` and accept the net/http transport — that is the supported
and honest trade.

## Choose a Wing stream preset

Use `kruda.Stream.With(kruda.StreamKeepAlive())` for a Linux Wing GET route that streams
a fixed-length HTTP 200 or 206 response. HLS segments and byte-range reads are
good candidates. It reuses the connection only when the response states one
valid, positive `Content-Length`, the body comes from `c.Stream`, and that copy
writes exactly the declared length. A handler that sends the headers before
`c.Stream` (a `Flush`, or a body write) closes the connection even if the
length turns out right. Set the header from a trusted upstream response that
describes the bytes you actually send.

```go
app.Get("/media/:name", streamSegment,
    kruda.Stream.With(kruda.StreamKeepAlive()))
app.Get("/events", streamEvents, kruda.Stream)
```

Use `kruda.Stream` for SSE, live feeds and streams whose final length is unknown.
A stream with unknown length, an incomplete body, a write failure or a request
to close the connection uses the close path. On platforms or transports where
the Wing option is unavailable, the route uses the transport's ordinary
streaming behavior. For an upstream HTTP request, use `c.Context()` so a client
disconnect can cancel the upstream work.

When comparing presets for your app, use the same segment size, viewer rate
and CPU budget. Check completed downloads, connection reuse, CPU, RSS and p99
time to read the complete segment.

## Wing limitations

Wing is optimized for raw throughput. It intentionally skips some HTTP features:

- **Prebuilt static responses bypass middleware** — `StaticText` and `StaticJSON` write route-level prebuilt responses directly. Use normal handlers when a response needs middleware, lifecycle hooks, CORS, cookies, or `WithSecureHeaders()`.
- **Incremental responses need a preset** — plain handlers write fixed `Content-Length` responses. SSE and chunked streaming (`c.SSE()` / `c.Stream()`) work on Wing via the `kruda.Stream` route preset, and WebSocket via `ws.HandleFunc` (the `kruda.Hijack` preset) — but a default route does not stream.
- **No HTTP/2** — Wing speaks HTTP/1.1 only. Use `kruda.NetHTTP()` with TLS for HTTP/2.
- **No chunked transfer encoding** — Wing pre-computes Content-Length.
- **`App.Serve(listener)` re-binds on Wing** — Wing serves one `SO_REUSEPORT` socket per worker, so it adopts the listener's *address* and closes the passed file descriptor rather than serving it. For systemd socket activation or graceful-restart fd-passing, use `kruda.NetHTTP()` (or fasthttp on macOS), which serve the supplied fd directly.

Wing apps can mix normal handlers with streaming and WebSocket routes by applying `kruda.Stream` or registering WebSockets with `ws.HandleFunc`. Choose net/http when you need direct TLS, HTTP/2, chunked request bodies, or exact stdlib transport semantics.
