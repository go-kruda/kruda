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
| Proxy client IPs | ✅ | `X-Forwarded-For` / `X-Real-IP` honored only under `WithTrustProxy(true)` |
| Accept-side DoS limits | ✅ | Connection cap, per-IP cap, accept-rate bucket (v1.4.0) |

## Deployment guidance

- **Linux + plaintext HTTP/1.1** (typically behind a TLS-terminating proxy or
  load balancer): Wing — this is the configuration all published benchmarks use.
- **Direct HTTPS / HTTP/2 / Windows**: net/http transport, selected
  automatically.
- Anything this table calls unsupported fails **loudly** (501/413/431), never
  silently.
