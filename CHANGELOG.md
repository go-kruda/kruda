# Changelog

All notable changes to Kruda are documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased] — TechEmpower Domination

### Added
- `Ctx.SetBody([]byte) *Ctx` — lazy-send response body (zero-copy, chainable)
- `Ctx.SendBytes([]byte) error` — eager-send response body (immediate flush, terminal)
- `Ctx.SetContentType(string) *Ctx` — set Content-Type header (chainable)
- Turbo Mode (`WithTurbo`) — SO_REUSEPORT prefork with per-core child processes (Linux only)
- TechEmpower Framework Benchmarks submission (`frameworks/Go/kruda/`)

### Removed
- Legacy TFB code (`cmd/techempower/`, `techempower/`, root `Dockerfile.techempower`, root `benchmark_config.json`)

## [Unreleased] — Phase 5: Production Ready

### Added
- Dev mode error page with source code context, stack trace, request details (`devmode.go`)
- Security hardening: path traversal prevention, header injection prevention, DoS defaults
- `docs/SECURITY.md` with threat model and mitigations
- CLI tool `cmd/kruda/` with `new`, `dev`, `generate`, `validate` commands
- Hot reload dev server with 100ms file polling
- 12 runnable example applications
- GitHub Actions CI/CD: test matrix, benchmark regression, docs deployment
- VitePress documentation site in `docs/`
- Benchmark baseline (`bench/baseline.txt`)
- Cross-runtime benchmark suite (Kruda vs Elysia/Bun)
- fasthttp transport for maximum throughput
- AI-friendly DX: `llms.txt`, `.cursor/rules`, `copilot-instructions.md`
- Integration tests (`integration_test.go`)
- Coverage gap tests reaching 92%+ on core package

### Changed
- Security headers defaults: X-Frame-Options DENY, X-XSS-Protection 0, Referrer-Policy strict-origin
- Default transport: fasthttp (Linux/macOS), net/http (Windows)

### Performance
- Kruda beats Elysia (Bun) by 38% on GET routes with fasthttp transport
- 3x faster than Echo/Gin, 5x faster than Fiber on Go benchmarks

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
- fasthttp transport for maximum throughput (now the default)
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
- File upload support with `c.FormFile()`
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
