# Kruda — Competitive Analysis

> **Purpose:** Track competitive landscape. Update this doc as competitors evolve.
> **Last updated:** 2026-02-22

## Market Segments

### Segment A: "Traditional" Go Frameworks (raw performance, ecosystem)
| Framework | Transport | Typed Handlers | Auto OpenAPI | DI | Learning Curve |
|-----------|-----------|----------------|--------------|-----|----------------|
| **Gin** | net/http | ❌ | ❌ | ❌ | ⭐ Low |
| **Fiber** | fasthttp | ❌ | ❌ | ❌ | ⭐ Low |
| **Echo** | net/http | ❌ | ❌ | ❌ | ⭐ Low |

### Segment B: "Next-Gen" Go Frameworks (typed handlers, auto-everything)
| Framework | Transport | Typed Handlers | Auto OpenAPI | DI | Learning Curve |
|-----------|-----------|----------------|--------------|-----|----------------|
| **Kruda** | Netpoll + net/http | ✅ C[T] | ✅ (Phase 2) | ✅ (Phase 4) | ⭐ Target: Low |
| **Fuego** | net/http (Go 1.22) | ✅ ContextWithBody[T] | ✅ | ❌ | ⭐⭐ Medium |
| **Huma** | Adapter (any router) | ✅ struct I/O | ✅ 3.1 | ❌ | ⭐⭐ Medium |
| **Hertz** | Netpoll | ❌ | ✅ (plugin) | ❌ | ⭐⭐ Medium |

## Direct Competitors (Segment B)

### Fuego
- **Repo:** github.com/go-fuego/fuego
- **Transport:** net/http (Go 1.22+ ServeMux)
- **Typed Handlers:** `ContextWithBody[T]` — body only, params separate
- **Auto OpenAPI:** ✅ from Go types
- **Strengths:**
  - Active development, growing community
  - 2025 roadmap: strongly typed params (Go 1.24), plug into Gin/Echo
  - Good documentation
- **Weaknesses:**
  - No custom router (uses stdlib ServeMux) → slower than radix tree
  - No Netpoll → ceiling on raw throughput
  - Body-only generics — params still manual
  - No DI container
- **Threat level:** 🟡 HIGH — moving fast, overlapping market

### Huma
- **Repo:** github.com/danielgtaylor/huma
- **Transport:** Adapter pattern (any router: chi, fiber, gin, echo)
- **Typed Handlers:** Struct-based Input/Output, separate from context
- **Auto OpenAPI:** ✅ OpenAPI 3.1
- **Strengths:**
  - Router-agnostic — works with any Go router
  - Mature OpenAPI generation
  - Good validation
- **Weaknesses:**
  - Adapter overhead — extra abstraction layer
  - Verbose: Input/Output structs separate from context
  - No built-in transport optimization
  - No DI container
- **Threat level:** 🟡 MEDIUM — mature but less momentum

### Hertz (ByteDance)
- **Repo:** github.com/cloudwego/hertz
- **Transport:** Netpoll (same as Kruda Phase 3)
- **Typed Handlers:** ❌ traditional handler pattern
- **Auto OpenAPI:** Plugin-based
- **Strengths:**
  - Production-proven at ByteDance scale
  - Netpoll reference implementation
  - HTTP/1.1, HTTP/2, HTTP/3 support
  - Great performance
- **Weaknesses:**
  - No typed handlers / generics
  - Complex — enterprise-oriented
  - Documentation mostly Chinese
  - No DI container
- **Threat level:** 🟢 LOW — different market (enterprise Chinese ecosystem)

## Feature Comparison Matrix

| Feature | Kruda | Fuego | Huma | Hertz | Gin | Fiber |
|---------|-------|-------|------|-------|-----|-------|
| **Typed Handlers** | ✅ C[T] unified | ✅ body only | ✅ struct I/O | ❌ | ❌ | ❌ |
| **Auto OpenAPI** | ✅ Phase 2 | ✅ | ✅ 3.1 | Plugin | ❌ | ❌ |
| **Validation** | ✅ structured errors | ✅ basic | ✅ | ❌ | Manual | Manual |
| **DI Container** | ✅ Phase 4 | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Auto CRUD** | ✅ Phase 4 | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Netpoll** | ✅ Phase 3 | ❌ | ❌ | ✅ | ❌ | ❌ (fasthttp) |
| **Custom Router** | ✅ radix tree | ❌ stdlib | ❌ adapter | ✅ | ✅ | ✅ |
| **Zero-alloc** | ✅ target | ❌ | ❌ | ✅ | Partial | ✅ |
| **Error Mapping** | ✅ MapError | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Frontend-ready Errors** | ✅ structured | ❌ | Partial | ❌ | ❌ | ❌ |
| **Dev Error Page** | ✅ Phase 5 | ❌ | ❌ | ❌ | ❌ | ❌ |
| **OpenAPI 3.1** | ✅ core Phase 2 | ✅ | ✅ 3.1 | Plugin | ❌ | ❌ |
| **File Upload Tags** | ✅ struct tags | ❌ | ❌ | ✅ basic | Manual | Manual |
| **SSE Helper** | ✅ c.SSE() | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Hot Reload CLI** | ✅ kruda dev | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Graceful Shutdown** | ✅ built-in | ❌ manual | ❌ | ✅ | ❌ manual | ✅ |
| **Request Logger** | ✅ c.Log() slog | ❌ | ❌ | ✅ | ❌ | ❌ |
| **Env Config** | ✅ WithEnvPrefix | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Observability** | ✅ OTel+Prom | ❌ | ❌ | ✅ | Community | Community |
| **HTTP/2** | ✅ via net/http TLS | ❌ | ❌ | ✅ | ✅ via net/http | ❌ (fasthttp) |
| **HTTP/3** | ✅ Phase 3 (QUIC) | ❌ | ❌ | ✅ | ❌ | ❌ |
| **WebSocket** | ✅ contrib | ❌ | ❌ | ✅ | Community | ✅ |
| **Response Cache** | ✅ contrib | ❌ | ❌ | ❌ | Community | Community |
| **Playground** | ✅ Phase 6 | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Context Safety** | ✅ safe copies | ✅ | ✅ | ✅ | ✅ | ⚠️ unsafe |

## Kruda's Unique Selling Points (USP)

1. **Unified C[T]** — body + param + query in one struct via tags (cleanest API)
2. **Pluggable Transport** — Netpoll for Linux/macOS, net/http for Windows (unique)
3. **Built-in DI** — no Wire, no FX, no codegen (only framework with built-in DI)
4. **Auto CRUD** — `app.Resource("/users", svc)` (no competitor has this)
5. **Frontend-ready Validation Errors** — structured JSON, i18n-ready (unique)
6. **Dev Mode Error Page** — rich HTML errors like Next.js (unique in Go)
7. **OpenAPI 3.1 in Core** — auto-generated from C[T], zero config (Fuego/Huma need setup)
8. **File Upload with Tags** — `validate:"max_size=5mb,mime=image/*"` (unique struct tag approach)
9. **Hot Reload CLI** — `kruda dev` with auto-rebuild (unique in Go frameworks)
10. **Full Observability Stack** — OTel + Prometheus + structured slog out of the box
11. **Full Protocol Stack** — HTTP/1.1 + HTTP/2 + HTTP/3 (QUIC) with auto-negotiation (matches Hertz, beats everyone else)

## Things Kruda MUST Match or Exceed

- [ ] Fuego's OpenAPI generation quality
- [ ] Huma's OpenAPI 3.1 compliance
- [ ] Hertz's Netpoll performance numbers
- [ ] Gin's ecosystem breadth (via contrib + adapters)
- [ ] Fiber's raw benchmark throughput
- [ ] Fuego's documentation quality
- [ ] Hertz's HTTP/2 and HTTP/3 protocol support

## Watch List

- **Fuego 2025 roadmap:** strongly typed params (Go 1.24), Gin/Echo plug-in
- **Go 1.26 Green Tea GC:** benchmark with 1.24 vs 1.26 to show improvement
- **Huma v2 evolution:** any new features or breaking changes
- **Hertz community growth:** if they add generics support

## Action Items

1. **Phase 1:** Graceful shutdown + request logger must work perfectly (table stakes)
2. **Phase 2:** Ensure validation errors are significantly better than Fuego/Huma
3. **Phase 2:** OpenAPI 3.1 must match Huma's compliance level
4. **Phase 2:** File upload DX must be simpler than any competitor
5. **Phase 3:** Benchmark against Hertz Netpoll — must be comparable
6. **Phase 3:** Test Go 1.24 vs 1.26 (Green Tea GC) as marketing data
7. **Phase 4:** DI must be optional — Go community dislikes forced "magic"
8. **Phase 4:** Contrib modules must have separate go.mod (no bloat in core)
9. **Phase 5:** Dev error page + hot reload = killer DX combo for marketing
10. **Ongoing:** Monitor Fuego releases — they move fast

## Future Ideas (Contrib)

### TypeScript Client Generation (`contrib/typegen/`)
Inspired by Elysia's Eden Treaty — auto-generate TypeScript SDK from Kruda's OpenAPI 3.1 spec.

- `kruda generate client --lang typescript --output ./client`
- Reads OpenAPI spec generated from typed handlers `C[T]`
- Outputs typed TypeScript client with full request/response types
- Zero manual type maintenance — regenerate on API change
- Enables end-to-end type safety: Go backend → TypeScript frontend
- Alternative to tRPC-style approach — works via standard OpenAPI, not proprietary protocol
- Priority: Post-launch contrib, not blocking v1.0.0
