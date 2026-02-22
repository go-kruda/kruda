# Phase 3 — Netpoll & Performance: Requirements

> **Goal:** Be the fastest type-safe Go framework
> **Timeline:** Week 7-8
> **Spec Reference:** Sections 3, 4, 5, 11, 12
> **Depends on:** Phase 1-2 complete

## Milestone

```go
// App auto-selects transport on startup
app := kruda.New().Listen(":3000")  // Windows: net/http, Linux/macOS: Netpoll

// Manual override (for testing)
app := kruda.New(
    kruda.WithTransport("netpoll"),  // or "nethttp"
).Listen(":3000")

// Benchmark results (goal)
// Throughput: >200K req/sec (single core)
// Latency P99: <5ms (at 50K req/sec load)
// Memory/req: <1KB allocation
```

## Components

| # | Component | File(s) | Est. Days | Priority | Status |
|---|-----------|---------|-----------|----------|--------|
| 1 | Netpoll Transport | `transport/netpoll.go` | 4 | 🔴 | 🔴 |
| 2 | Transport Auto-selection | `config.go`, `kruda.go` | 1 | 🔴 | 🔴 |
| 3 | Zero-alloc Context | `context.go`, `internal/pool/` | 3 | 🔴 | 🔴 |
| 4 | Header Optimization | `context.go`, `transport/` | 2 | 🟡 | 🔴 |
| 5 | Benchmark Suite | `bench/` | 2 | 🟡 | 🔴 |
| 6 | HTTP/2 Documentation | `transport/nethttp.go`, docs | 0.5 | 🟡 | 🔴 |
| 7 | HTTP/3 Transport | `transport/quic.go` | 3 | 🟡 | 🔴 |

## Key Requirements

### Netpoll Transport
- `transport/netpoll.go` — Netpoll listener, request handler, graceful shutdown
- Support for network event multiplexing (epoll/kqueue)
- HTTP/1.1 parsing (reuse code from Netpoll library if available)
- Same `transport.Transport` interface as net/http
- Connection pooling for efficiency
- Handle Windows gracefully (fall back to net/http or use Netpoll emulation)

### Transport Auto-selection
- `Config.Transport` option: auto-detect OS, allow manual override
- Logic: `GOOS == "windows"` → net/http, else → Netpoll
- Fallback: if Netpoll fails to start, retry with net/http
- Environment variable override: `KRUDA_TRANSPORT=nethttp` or `netpoll`

### Zero-alloc Context
- Pre-allocate all request maps at pool init time
- Maps cleared (not re-allocated) on releaseCtx
- `internal/pool/pool.go` — Custom pool with pre-allocated Ctx + maps
- Route params slice: pre-allocated for max 32 params
- Response headers: fixed-slot array (not map) for common headers
- Request headers: lazy map allocation on first access (most requests don't read all headers)

### Header Optimization
- Common response headers (Content-Type, Content-Length, Cache-Control, etc.) in fixed slots
- Header writing optimized: batch I/O, avoid string copies
- `http.Header` still available for custom headers
- Benchmark: measure `c.Header().Set()` + write latency vs raw slot

#### Phase 1 Review Finding: `respHeaders` Allocation (N2)

During Phase 1 code review, `respHeaders` was changed from `map[string]string` to `map[string][]string` to support multi-value headers (e.g., `Set-Cookie`, `Vary`). This adds ~120 bytes extra allocation per request (24 bytes per `[]string` slice header × ~5 typical response headers).

**Current implementation** (`context.go`):
```go
respHeaders map[string][]string
```

**Optimization candidates** (decide after benchmark data):
- **Option A — Dual maps**: Keep `map[string]string` for single-value headers (hot path), use `map[string][]string` only for multi-value headers (`Set-Cookie`, `Vary`). Avoids slice allocation for the common case.
- **Option B — Pre-allocated fixed-slot array**: Use a fixed-size array for common headers (Content-Type, Content-Length, etc.) with overflow to map. Eliminates map allocation entirely for typical responses.

**Decision**: Needs benchmark data from Component #5 (Benchmark Suite) before choosing. Measure per-request allocation delta between `map[string]string` vs `map[string][]string` under realistic workloads.

### HTTP/2 Support (via net/http)
- Document that net/http transport already supports HTTP/2 via Go stdlib
- When TLS is configured, HTTP/2 is auto-negotiated (ALPN)
- `kruda.New(kruda.WithTLS(cert, key))` enables HTTP/2 automatically
- No additional dependency needed
- Test: verify HTTP/2 negotiation works with TLS

### HTTP/3 Transport (QUIC)
- `transport/quic.go` — HTTP/3 transport using `github.com/quic-go/quic-go`
- Same `transport.Transport` interface as net/http and Netpoll
- `kruda.New(kruda.WithHTTP3())` enables HTTP/3
- Dual-stack: serve HTTP/2 (TCP) + HTTP/3 (QUIC) simultaneously
- `Alt-Svc` header auto-set to advertise HTTP/3 availability
- Fallback: if QUIC fails, clients fall back to HTTP/2 transparently
- Benefits: 0-RTT connection, no head-of-line blocking, better mobile performance
- Dependency: `github.com/quic-go/quic-go` (only pulled when HTTP/3 is used)

### Benchmark Suite
- `bench/` directory with benchmarks
- Compare: Kruda vs Fiber, Gin, Echo, Fuego, Hertz
- Workloads: GET/POST, static/parameterized, middleware chain
- Report: throughput, latency p50/p99/p99.9, allocations per request
- CI: run benchmarks on PR, track results over time
- **Go 1.24 vs 1.26 comparison** (NEW): run same benchmarks on both versions
  - Green Tea GC (Go 1.26 default): expect 10-40% GC overhead reduction
  - cgo improvement (~30% faster): directly benefits Sonic JSON
  - Use as marketing data: "Kruda + Go 1.26 = X% faster"

### HTTP/2 & HTTP/3 Support
- HTTP/2: Already supported via net/http transport (Go stdlib auto-negotiates HTTP/2 over TLS)
  - Document clearly that `WithTLS()` enables HTTP/2 automatically
  - Ensure Netpoll transport can fallback to net/http for HTTP/2 when needed
- HTTP/3 (QUIC): New transport implementation
  - `transport/quic.go` — HTTP/3 transport using `github.com/quic-go/quic-go`
  - Same `transport.Transport` interface as net/http and Netpoll
  - Usage: `kruda.New(kruda.WithHTTP3())` or auto-detect
  - Alt-Svc header support for HTTP/2 → HTTP/3 upgrade
  - UDP-based, benefits mobile/lossy networks
  - Optional: only imported when user opts in (build tag or separate module)

### Hertz Reference Study (NEW — learn from ByteDance)

Hertz (github.com/cloudwego/hertz) uses Netpoll same as Kruda. Key learnings:
- Requests >1MB: recommend go net + streaming (not Netpoll)
  - Kruda should auto-detect large requests and switch to streaming mode
- Protocol abstraction: Hertz supports HTTP/1.1, HTTP/2, HTTP/3
  - Kruda Phase 3: focus on HTTP/1.1 first, protocol layer extensible for future
- Netpoll doesn't support Windows: auto-fallback (Kruda already does this)
- Connection management: Hertz uses per-connection buffer pools
  - Study their buffer allocation strategy for Kruda's implementation

**Action:** Before implementing `transport/netpoll.go`, study:
- `github.com/cloudwego/hertz/pkg/network/netpoll/` — transport layer
- `github.com/cloudwego/netpoll` — the Netpoll library itself
- `github.com/cloudwego/hertz/pkg/protocol/` — HTTP parsing

## Non-Functional Requirements

- **NFR-1:** No breaking API changes from Phase 1-2
- **NFR-2:** Backward compatible with net/http transport
- **NFR-3:** Benchmarks reproducible: same results ±5% across runs
- **NFR-4:** All Netpoll code has doc comments
- **NFR-5:** Benchmark results published in README/docs
