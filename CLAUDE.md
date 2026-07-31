# Kruda Framework — AI Context

> This file helps AI coding assistants (Claude Code, Cursor, Copilot) understand the Kruda codebase.

## What is Kruda?

Kruda is a high-performance Go web framework that combines speed with type-safety through Go generics.

- **Module**: `github.com/go-kruda/kruda`
- **Go version**: 1.25+
- **License**: MIT

## Architecture

```
User API Layer (app.Get/Post/Typed Handlers/Resource)
    ↓
Middleware Pipeline (onRequest → beforeHandle → handler → afterHandle)
    ↓
Radix Tree Router (AOT-compiled, zero-alloc)
    ↓
Context (sync.Pool, safe copies)
    ↓
Transport Layer (pluggable: Wing on Linux / fasthttp on macOS / net/http fallback)
```

## Key Design Decisions

### Transport Layer
- Default: Wing on Linux (epoll+eventfd), fasthttp on macOS, net/http on Windows
- Override: `kruda.New(kruda.FastHTTP())` or `kruda.New(kruda.NetHTTP())`
- Auto-fallback: TLS or Windows → net/http even without explicit option

### Context
- Pooled via sync.Pool, all strings are safe copies
- Maps pre-allocated and cleared (not re-allocated) per request
- Body is copy-on-read

### Router
- Radix tree, zero-alloc matching
- Patterns: static, `:param`, `*wildcard`, `:id<regex>`, `:id?` (optional)
- AOT-compiled at startup via Compile()

### Typed Handlers
```go
type C[T any] struct {
    *Ctx
    In T  // parsed and validated input
}

// Usage: body + param + query parsed into one struct.
// validate: tags are enforced by default since v1.7.1. Opt out with
// kruda.New(kruda.WithoutValidation()). A tag naming a rule Kruda does not
// implement is skipped with a startup warning rather than a panic.
kruda.Post[CreateUser, User](app, "/users", func(c *kruda.C[CreateUser]) (*User, error) {
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})
```

### Auto CRUD
```go
// Implement ResourceService[T] → get 5 REST endpoints
kruda.Resource[User, string](app, "/users", &UserCRUD{db: db})
// GET /users, GET /users/:id, POST /users, PUT /users/:id, DELETE /users/:id
```

### Dependency Injection
```go
c := kruda.NewContainer()
c.Give(&UserService{})
c.GiveLazy(func() (*DBPool, error) { return connectDB() })

app := kruda.New(kruda.WithContainer(c))
svc := kruda.MustResolve[*UserService](c)
```

### Method Chaining
```go
app := kruda.New().
    Use(middleware.Logger(), middleware.CORS()).
    Get("/", handler).
    Group("/api/v1").
        Guard(jwt).
        Resource("/products", svc).
        Done().
    Listen(":3000")
```

## Project Structure

### Core (`github.com/go-kruda/kruda`)
- App, Router, Context, Config, Error, Middleware chain
- Typed Handlers `C[T]`, Input Binding, Validation
- OpenAPI 3.1 generation from `C[T]` types
- Lifecycle hooks, Graceful shutdown
- Built-in middleware: Logger, Recovery, CORS, RequestID, Timeout
- SSE helper, File upload
- Transport interface + net/http transport

### Contrib (`contrib/*`, separate go.mod per package)
- `contrib/jwt/` — JWT authentication
- `contrib/ws/` — WebSocket
- `contrib/ratelimit/` — Token bucket / sliding window rate limiting
- `contrib/session/` — Session middleware with pluggable store
- `contrib/compress/` — Response compression (gzip, brotli)
- `contrib/etag/` — ETag response caching
- `contrib/cache/` — Response cache (in-memory, Redis)
- `contrib/otel/` — OpenTelemetry tracing
- `contrib/prometheus/` — Prometheus metrics
- `contrib/swagger/` — Swagger UI
- `contrib/observability/` — Turnkey observability: one-call `Enable()` for otel tracing + RED metrics + trace/log correlation + K8s probes + `/metrics`

### Wing transport (in core since v1.2.0 — `wing_*.go` at repo root)
- Wing: custom async I/O transport (epoll+eventfd on Linux, kqueue on macOS)
- Presets: per-route composition passed directly as a RouteOption — structural `Bolt`/`Arrow`/`Spear` + semantic `Plaintext`/`JSON`/`DB`/`Render` (e.g. `app.Get("/db", h, kruda.DB)`); customize via `Preset.With(...)`; `StaticText`/`StaticJSON` are the opt-in handler-bypass options
- String fast lane: `c.Text`/`c.HTML`/`c.JSON` with no custom headers/cookies serialize through zero-copy responders (`transport.StringResponder`/`JSONResponder`) gated by `canBypassHeaderWrite`
- Blocking advisor: inline routes that block the event loop (>100µs, 10×) log one warning per route per process; no auto-switching ever
- All Wing types live in `wing_types_shared.go` (no build tag) so cross-platform stubs can never drift
- `transport/wing/` deprecation shim was removed in v1.3.0 (module tags ≤ transport/wing/v1.1.3 keep serving pinned users)

### Wing model vocabulary (ครุฑ)
- Kruda (ครุฑ) = the bird: the framework
- Wing = the transport: socket I/O, protocol parsing, connection lifecycle, timeouts, response writes — app-level, one per app
- Preset = the per-route composition (public type `Preset`): dispatch mode, response-mode tag, optional static response — route-level, many per app
- Feather = component-axis vocabulary in docs/architecture (dispatch feather, response feather); not a public type name
- Bone = internal invariants; never public API
- Default Kruda behavior is the framework contract: handler, middleware, lifecycle hooks, cookies, CORS, secure headers, safety checks, panic recovery, and error handling must remain intact unless an opt-in Preset option documents a bypass (StaticText/StaticJSON)
- Design rationale in-repo: `docs/decisions/0001-break-api-in-v1-minor.md` + the CHANGELOG 1.3.0 migration table (full design specs are maintainer-local notes, not tracked)

### Tooling (`cmd/kruda/`)
- `kruda new` — project scaffolding
- `kruda dev` — hot reload dev server
- `kruda generate` — code generation
- `kruda validate` — validate Kruda project configuration
- `kruda mcp` — run as an MCP stdio server for AI coding assistants
- `kruda pgo` — generate a PGO (Profile-Guided Optimization) profile

### Examples (`examples/`)
- 23 runnable examples covering all major features
- Each has `main.go` + `README.md`

## Code Style
- All exported types/functions have doc comments
- Follow Go standard conventions (gofmt, go vet)
- Minimal external deps for core (Sonic JSON opt-out via `kruda_stdjson` build tag)
- Use `slog` for logging (Go 1.21+ standard)
- Functional options pattern for configuration

## Versioning & Breaking Changes
Governed by `docs/decisions/0002-breaking-changes-after-adoption.md`. Kruda has a
production adopter, so "adopters ≈ 0" (ADR 0001) is no longer a valid argument.
Four obligations for a breaking **or behaviour** change:

1. **The version number is a signal, not the warning.** Kruda deliberately does
   **not** ban behaviour changes from patches — v1.7.1 ships validation-by-default,
   which changes behaviour and adds exported API. In exchange the disclosure duty is
   unconditional: any release that can change what an application does carries a
   `### Breaking` section with the opt-out and the concrete before/after, **whatever
   its number**. Consequence to state plainly, not hide: **Kruda's patch channel is
   not safe to install unread**, and `gorelease`/`apidiff` will flag such a patch.
2. **Ship a documented way to keep the old behaviour**, in the same CHANGELOG entry
   (`-tags kruda_stdjson`; `kruda.WithoutValidation()`). ⚠️ **Obligation 1's
   indifference to version levels does NOT extend to a compile-time break.** A
   removed or renamed symbol can never satisfy this obligation — nothing brings it
   back — and **a deprecation window does not fix that**, since the window lives in an
   earlier release while ob. 2 asks for the opt-out in *this* entry. The window
   substitutes, and is paid for on both ends: the deprecation release keeps the old
   symbol alive and announces its end; the removal release must give the replacement
   mapping **and name the version that announced the deprecation**, and must **not be
   a patch** — a removal is the only change with no opt-out at all, so the number is
   the last signal left. No window → **v2.0.0**. Additions are fine at any level,
   patches included.
3. **State it concretely** — v1.7.0's before/after byte table is the standard. This
   now does the work the version number used to be trusted for.
4. **One attributable cause per release, and stage them.** A change plus the
   prerequisites that make it safe is one item; two independent behaviour changes are
   two releases. Staging has the teeth: v1.7.1 waits until Rianhub has absorbed
   v1.7.0. Waiting is the default; **jumping the queue** is what must be justified.

**The exception — a real attack may jump the queue** and ship out of band without
waiting for the adopter to absorb the previous release.
The gate is deliberately narrow and needs **all three**:

1. **Reproduced** — in Kruda's code, the fix's diff carries a test that **fails
   without the fix** (how `TestWingParser_RejectsMalformedHeaderLines` came to
   exist); in a dependency, an upstream advisory plus govulncheck showing it
   reachable from Kruda (the GO-2026-6061 pattern). Provenance is irrelevant: a
   `FuzzParserDifferential` hit and a stranger's report owe the same test. **No
   document substitutes** — a `Triage` report, an accepted draft, an advisory id and
   a filled-in advisory all prove nothing, since `SECURITY.md` asks the *reporter*
   for affected versions and impact.
2. **An attack, not just a defect** — every bug fix has a failing test, so this is
   the half that does the work. The advisory must say **who the attacker is, what
   they gain, and which boundary breaks**; "nothing they didn't already have" means
   ordinary bug, waits its turn, however severe. Worked example:
   validation-by-default is trivially reproducible and still not a vulnerability —
   `validate` tags were inert, so an app relying on them relied on something that
   never ran, and no Kruda boundary breaks. So v1.7.1 waits for Rianhub to absorb
   v1.7.0 rather than jumping.
3. **Narrow enough to ship on its own** — jumping the queue means shipping without
   the soak staging would have given, so it must not carry anything that needed one.

Can't reproduce in time? It waits its turn; embargo is fine. Proactive hardening has
no defect to reproduce (v1.4.0's DoS caps took the ordinary route); a fix moving a
floor for everyone takes it too, with a `### Security` section (v1.5.0's Go bump,
despite two advisory ids).

## Testing
```bash
# Core tests — Wing tests are included since v1.2.0 (no separate cd needed)
go test -v -race -tags kruda_stdjson ./...

# Pre-release validation
./scripts/pre-release.sh

# Contrib package tests (each contrib has its own go.mod)
cd contrib/jwt && go test -tags kruda_stdjson ./...
```

## Build Tags
- `kruda_stdjson` — use stdlib `encoding/json` instead of Sonic (for portability/CI)
