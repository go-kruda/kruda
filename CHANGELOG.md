# Changelog

All notable changes to Kruda are documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.2.0] — 2026-04-19

### Added
- Cross-transport integration smoke test exercising nethttp/fasthttp/wing end-to-end
- Native Go fuzz tests for router patterns + match (`FuzzRouterPattern`, `FuzzRouterMatch`), JSON binding (`FuzzBindJSON`), and validation (`FuzzValidateString`)
- Package-level `doc.go` overview for the core package and every contrib package — appears on pkg.go.dev
- Runnable `Example*` functions in godoc for the core API (`ExampleNew`, `ExamplePost`, `ExampleApp_Use`, `ExampleApp_Group`)
- `app_serve.go` — extracted request-dispatch internals from `kruda.go`
- `ctx_request.go`, `ctx_response.go`, `ctx_state.go`, `ctx_lifecycle.go` — split out from `context.go`
- Per-symbol godoc for previously undocumented exports: `Container`, `GoViewEngine.Render`, `WingConfig`, Wing Transport, semantic Feather presets, Wing stub Transport / DispatchMode constants
- `scripts/pre-release.sh` — gating validator (clean tree, no dev replace directives, all builds + tests + fuzz suites + godoc + examples)

### Changed
- Wing transport flattened into the core `kruda` package; `import "github.com/go-kruda/kruda/transport/wing"` continues to work as a deprecation alias.
- Test files renamed by feature for clarity (the previous `coverage_boost*_test.go` names — which were real tests with misleading names — became files like `error_constructors_test.go`, `context_methods_test.go`, etc.)
- Release process simplified to a single tag covering core + contrib — see [docs/release-process.md](docs/release-process.md). Old `scripts/tag-submodules.sh` removed.

### Deprecated
- `import "github.com/go-kruda/kruda/transport/wing"` — use `github.com/go-kruda/kruda` instead. The alias package continues to work and will be removed in v2.0.0.

### Fixed
- Eliminated the circular dependency between core and `transport/wing` that produced the broken v1.1.0–v1.1.2 releases (those tags are retracted in `go.mod`).
- **Wing fd-recycling race in `worker.cleanup()`** — pool, Spawn, and Takeover dispatch goroutines are now tracked via `sync.WaitGroup` (`pool.wg` for the pool, `worker.dispatchWG` for Spawn/Takeover). Cleanup waits for in-flight `RawSyscall(SYS_WRITE)` calls to finish before closing fds, preventing writes from landing on fds the kernel has already recycled to a new connection.
- **Wing shutdown deadlock with Takeover dispatch** — Takeover goroutines that block on `syscall.Read` are now unblocked by `syscall.Shutdown(fd, SHUT_RD)` before `dispatchWG.Wait()`. SHUT_RD returns EOF without freeing the fd number, so concurrent Spawn writes still target the right connection.
- **Wing shutdown `doneCh` saturation deadlock** — a drain goroutine consumes `doneCh` while `dispatchWG.Wait()` runs, preventing a wave of completing Spawn/Takeover goroutines from blocking their channel sends and never reaching `Done()`.
- **`ServeKruda` lifecycle now mirrors `ServeHTTP`** — `OnRequest` hook errors `goto response` (was `return`) so `OnResponse` hooks always fire for metrics/logging consistency. All hook iterations gated by `if app.hasLifecycle { … }` for zero-cost when no hooks are registered.
- WebSocket `dialWS` test helper preserves bytes the bufio.Reader greedily read past the handshake response, fixing a flaky `TestConn_ConcurrentWrites` (visible on macOS CI runners).

## [1.0.0] - 2026-03-07

### Added
- Wing transport — custom epoll+eventfd engine (default on Linux)
- eventfd wake mechanism replacing pipe on Linux (lower syscall overhead)
- Feather dispatch system — per-route I/O strategy hints (`WingPlaintext`, `WingJSON`, `WingQuery`, `WingRender`)
- `Ctx.SetBody([]byte) *Ctx` — lazy-send response body (zero-copy, chainable)
- `Ctx.SendBytes([]byte) error` — eager-send response body (immediate flush, terminal)
- `Ctx.SendBytesWithTypeBytes([]byte, []byte) error` — zero-alloc typed response
- `Ctx.SendStaticWithTypeBytes([]byte, []byte) error` — zero-copy for immutable data
- `Ctx.SetContentType(string) *Ctx` — set Content-Type header (chainable)
- sendfile(2) zero-copy support for Wing static file serving
- Wing RawRequest interface with Fd, RawHeader, RawBody, KeepAlive
- Wing MultipartForm support
- Wing read/write/idle timeouts + request Context cancellation
- TechEmpower Framework Benchmarks submission (`frameworks/Go/kruda/`)
- HTTP parser fuzz testing with seed corpus
- contrib/jwt — JWT authentication (HS256/384/512, RS256)
- contrib/ws — WebSocket with RFC 6455 compliance, ping rate limiting
- contrib/ratelimit — token bucket + sliding window rate limiting
- Security hardening: path traversal (3-layer), header injection, ReDoS prevention, SSE injection
- `SECURITY.md` with responsible disclosure policy (48h ack, 7d assessment, 90d disclosure)
- Consolidated security guide at `docs/guide/security.md`
- GC tuning documentation with GOGC/GOMEMLIMIT presets

### Changed
- Default transport: Wing on Linux, fasthttp on macOS, net/http on Windows (auto-fallback)
- TLS auto-fallback: Wing/fasthttp → net/http when TLS is configured
- Security headers defaults: X-Frame-Options DENY, X-XSS-Protection 0, Referrer-Policy strict-origin
- HTML escape in fortunes uses `&#34;` for quotes (TFB spec compliance)

### Removed
- Turbo/prefork mode (replaced by Wing transport)
- Legacy TFB code (`cmd/techempower/`, `techempower/`, root `Dockerfile.techempower`)
- Bone configuration axis (simplified to Feather presets)

### Performance
- Plaintext: 846K req/s (vs Fiber 670K, +26%; vs Actix 814K, +4%)
- JSON: 805K req/s (vs Fiber 625K, +29%; vs Actix 790K, +2%)
- DB: 108K req/s (vs Fiber 107K, +1%; vs Actix 37K, +190%)
- Fortunes: 104K req/s (vs Actix 45K, +131%)
- eventfd wake: +18% plaintext throughput vs pipe

## [0.5.0] — Phase 5: Production Ready

### Added
- Dev mode error page with source code context, stack trace, request details
- CLI tool `cmd/kruda/` with `new`, `dev`, `generate`, `validate` commands
- Hot reload dev server with 100ms file polling
- 21 runnable example applications
- GitHub Actions CI/CD: test matrix, benchmark regression, docs deployment
- VitePress documentation site in `docs/`
- Benchmark baseline (`bench/baseline.txt`)
- Cross-runtime benchmark suite (Go frameworks comparison)
- fasthttp transport for maximum throughput
- Integration tests (`integration_test.go`)
- Coverage gap tests reaching 92%+ on core package

### Security
- Default security headers: X-Content-Type-Options, X-Frame-Options, Referrer-Policy
- CRLF header injection prevention in SetHeader/AddHeader
- Request body size limit (4MB default) with 413 response
- CORS credentials + wildcard validation (panic at init)

## [0.4.0] — Phase 4: Ecosystem

### Added
- DI container with `Give`, `Use`, `GiveNamed`, `UseNamed`, `GiveLazy`, `GiveTransient`
- `MustUse`, `MustUseNamed` panic variants for DI resolution
- Module system with `Install(container)` interface
- Auto CRUD via `Resource(app, path, service)` with `ResourceService[T]` interface
- Resource options: `WithResourceOnly`, `WithResourceMiddleware`, `WithResourceIDParam`
- Health check endpoint with auto-discovery of `HealthChecker` services
- Error mapping: `MapError`, `MapErrorType[T]`, `MapErrorFunc`
- Test client `NewTestClient(app)` with fluent request builder
- `Resolve`, `ResolveNamed`, `MustResolve`, `MustResolveNamed` for handler-scoped DI
- Container lifecycle: `Start(ctx)`, `Shutdown(ctx)` for managed services

## [0.3.0] — Phase 3: Performance

### Added
- fasthttp transport for maximum throughput (default on macOS; Wing is default on Linux)
- `FastHTTP()` and `NetHTTP()` transport options
- Transport auto-fallback: TLS or Windows → net/http
- `WithTransport(transport.Transport)` for custom transport implementations
- HTTP/2 support via net/http TLS
- HTTP/3 (QUIC) config with `WithHTTP3(cert, key)` and Alt-Svc header
- Benchmark suite in `bench/` comparing Kruda, Gin, Fiber, Echo

## [0.2.0] — Phase 2: Type System & Validation

### Added
- Generic typed handlers `C[T]` with `Get/Post/Put/Delete/Patch[In, Out]`
- Short handlers `GetX/PostX/PutX/DeleteX/PatchX` (no error return)
- Group typed handlers `GroupGet/GroupPost/GroupPut/GroupDelete/GroupPatch`
- Input binding from param, query, body via struct tags
- Validation engine with `required`, `min`, `max`, `email`, `oneof` rules
- Custom validator registration via `app.Validator().Register()`
- JSON engine abstraction with Sonic + stdlib fallback (`json/`)
- `WithJSONEncoder`, `WithJSONDecoder` config options
- File upload support with `*FileUpload` struct binding
- SSE helper `c.SSE(callback)` with auto headers
- Route options `WithDescription`, `WithTags` for OpenAPI metadata

## [0.1.0] — Phase 1: Foundation

### Added
- `App` struct with `New()`, `Listen()`, graceful shutdown (SIGINT/SIGTERM)
- Radix tree router with static, `:param`, `*wildcard`, `:id<regex>`, `:id?` patterns
- AOT route compilation via `Compile()`
- `Ctx` struct with sync.Pool reuse, request/response API, method chaining
- Route groups with `Group(prefix)`, scoped middleware, `Done()`
- Middleware chain pre-built at registration time (zero-alloc hot path)
- Lifecycle hooks: OnRequest, BeforeHandle, AfterHandle, OnResponse, OnError, OnShutdown
- Config with functional options: `WithReadTimeout`, `WithWriteTimeout`, `WithIdleTimeout`, etc.
- Environment config via `WithEnvPrefix("APP")`
- Built-in middleware: Logger, Recovery, CORS, RequestID, Timeout
- `Map` type alias (`map[string]any`)
- `KrudaError` with convenience constructors (BadRequest, NotFound, InternalError, etc.)
- Transport interface with net/http implementation
- `internal/bytesconv` zero-copy byte/string conversion
- Hello world example (`examples/hello/`)
- MIT License
