# Wing Protocol Support Matrix

Wing is a deliberate, high-performance **HTTP/1.1 subset**. This page is the
authoritative statement of what it speaks. Anything marked "via net/http"
means Kruda automatically serves through the net/http transport in that
configuration — you keep correctness, you are just not running Wing.

| Capability | Wing | Notes |
|---|---|---|
| HTTP/1.1 keep-alive | ✅ | Default |
| HTTP/1.1 pipelining | ✅ | Responses written in request order |
| Request header retention | ✅ All headers | 8 inline slots + heap spill; spills observable via `WingHeaderSpills()` |
| Request size limits | ✅ | `BodyLimit` → 413, `HeaderLimit` (8 KB default) → 431 |
| `Expect: 100-continue` | ✅ | Interim 100 sent when the body must be awaited |
| Chunked **request** bodies (`Transfer-Encoding`) | ❌ → 501 | Send `Content-Length`; chunked uploads need net/http |
| Chunked **response** bodies | ❌ (by design) | Fixed responses use `Content-Length`; incremental responses use the `Stream` preset (close-delimited, SSE-style) |
| Trailers | ❌ | Not parsed, not emitted |
| SSE / incremental streaming | ✅ | `c.SSE()` / `c.Stream()` with the `kruda.Stream` route preset (v1.5.0+) |
| WebSocket | ✅ | `ws.HandleFunc(app, "/ws", handler)` — Wing hands the connection to `contrib/ws` via the `kruda.Hijack` preset (v1.6.0+) |
| TLS / HTTPS | via net/http | `WithTLS` auto-falls back — see [Production TLS](/guide/transport#production-tls-terminate-in-front-of-wing) |
| HTTP/2 | via net/http | Negotiated by net/http under TLS |
| HTTP/3 | ❌ | Cancelled from the roadmap (v1.5.0) |
| Proxy client IPs | ✅ | `X-Forwarded-For` / `X-Real-IP` require `WithTrustProxy(true)` and an exact peer match in `WithTrustedProxies` |
| Accept-side DoS limits | ✅ | Connection cap, per-IP cap, accept-rate bucket (v1.4.0) |

## Deployment guidance

- **Linux + plaintext HTTP/1.1** (typically behind a TLS-terminating proxy or
  load balancer): Wing — this is the configuration all published benchmarks use.
- **Direct HTTPS / HTTP/2 / Windows**: net/http transport, selected
  automatically.
- Anything this table calls unsupported fails **loudly** (501/413/431), never
  silently.

Static routes admit at most 1024 open files per route by default. Set
`WithMaxOpenFiles(n)` on `Static` or `StaticFS` to change this limit; excess
requests receive 503 before the filesystem is opened. Admission is retained
until the file's `Close` returns (a panicking Close retains its admission), including empty files, canceled reads, and
slow closes. Custom files whose `Read` never returns cannot be forcibly stopped;
shutdown does not wait for them, and they retain their admission until cleanup
can complete. Deferred body reads and closes run outside the Wing event loop and read
chunks stay bounded to 32 KiB. Keep write timeouts enabled to expire stalled
sources and clients.

The admission limit applies to built-in static routes, including sendfile delivery.
Handlers using `transport.FileStreamer` directly manage their own admission.
Filesystem operations performed before body transfer (Open, Stat, directory
handling, and error cleanup) follow the route's handler dispatch and recovery
policy; use Pool or Spawn for filesystems that can block in those operations.
