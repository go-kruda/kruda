# Changelog

All notable changes to Kruda are documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.3.0] — unreleased

### Breaking

The per-route tuning API was renamed to match the Kruda model: a route rides
a Preset composed from Wing's internals. With effectively zero external
adopters, the rename ships as a v1 minor release instead of a v2 — rationale
in `docs/decisions/0001-break-api-in-v1-minor.md`. Migration table:

| v1.2.x | v1.3.0 |
|--------|--------|
| `kruda.WingPlaintext()` | `kruda.Plaintext` |
| `kruda.WingJSON()` | `kruda.JSON` |
| `kruda.WingQuery()` | `kruda.DB` |
| `kruda.WingRender()` | `kruda.Render` |
| `kruda.WingFeather(f)` | pass the `Preset` value directly as a route option |
| `kruda.WingStaticText(...)` | `kruda.StaticText(...)` |
| `kruda.WingStaticJSON(...)` | `kruda.StaticJSON(...)` |
| `kruda.Feather` / `kruda.FeatherOption` | `kruda.Preset` / `kruda.PresetOption` |
| `kruda.FeatherTable` / `kruda.NewFeatherTable` | `kruda.PresetTable` / `kruda.NewPresetTable` |
| `WingConfig.Feathers` / `.DefaultFeather` | `WingConfig.Presets` / `.DefaultPreset` |
| `transport.FeatherConfigurator.SetRouteFeather` | `transport.PresetConfigurator.SetRoutePreset` |
| `transport.StaticTextResponder` | removed — `c.Text`/`c.HTML` ride the string fast lane |
| `RouteOption` (func type) | interface — presets implement it; custom func options wrap internally |
| `KRUDA_ASYNC`, `KRUDA_POOL_ROUTES`, `KRUDA_SPAWN_ROUTES`, `KRUDA_STATIC` | removed — use route presets or `WingConfig.Presets` |
| `github.com/go-kruda/kruda/transport/wing` | removed — import `github.com/go-kruda/kruda` (tags up to `transport/wing/v1.2.0` keep working for pinned users) |

### Added
- Wing string response fast lane: `c.Text` and `c.HTML` now serialize
  status + Date + Content-Type + Content-Length + body in one zero-copy pass
  (twin of the JSON fast lane), with automatic fallback to the standard path
  when headers, cookies, or secure headers are configured.
- `transport.StringResponder` optional transport interface.
- Blocking advisor: a route left on inline dispatch that blocks the event
  loop (>100µs wall time, 10 times) logs one warning per route per process
  suggesting `kruda.DB` or `kruda.Spear`. Observation only — no dispatch mode
  is ever switched automatically.
- Registration-time `slog.Debug` line for preset-annotated routes.

### Fixed
- `c.Text` responses on Wing no longer share a global static cache: the Date
  header is always current, the background Date-patcher data race is gone,
  and dynamic text bodies no longer grow an unbounded cache.
- `c.HTML` now returns `ErrAlreadyResponded` on double-respond, matching
  `c.Text`/`c.JSON`.

## [1.2.5] — 2026-06-06

### Added
- Added reproducible Wing CPU, JSON, DB, resource, read-buffer, and pipelined syscall benchmark evidence after the v1.2.4 baseline.
- Added benchmark controls for default Kruda JSON encoder runs, DB dispatch sweeps, framework-specific DB DSNs, and pipelined syscall diagnostics.

### Changed
- Streamed Wing stdjson responses into pooled buffers and pre-sized JSON response buffers on the JSON response path.
- Preferred the Wing JSON responder for static JSON route hints.
- Clarified public benchmark wording, Wing query-profile guidance, and workload-specific read-only DB evidence without broadening CPU-bound handler-path claims.
- Documented `KRUDA_READ_BUF_SIZE=2048` as an optional short-header memory profile candidate while keeping the framework default unchanged.

### Fixed
- Skipped duplicate Wing accept re-arms on Linux epoll listeners after successful accept events, reducing accept hot-path churn without changing normal handler behavior.
- Stopped suggesting the nonexistent `WingStream` hint in public documentation.
- Normalized benchmark module local replace paths for current Go toolchain validation.

## [1.2.4] — 2026-05-28

### Added
- Added opt-in Wing static response route options for public static hot paths: `WingStaticText` and `WingStaticJSON`.
- Added reproducible CPU-bound benchmark evidence for Kruda, Fiber, and Actix, including latency percentiles, error counts, non-2xx counts, CPU/RAM resource data, syscall traces, and CPU profiles.
- Added Wing flight model documentation defining Transport, Wing, Feather, and Bone terminology.

### Changed
- Improved Wing CPU-bound handler paths with single-handler dispatch, inline JSON response writing, common-header parsing, route Feather caching, clean path caching, lazy peer address lookup, read buffer tuning, and lower fair-handler overhead.
- Updated public performance documentation to use balanced CPU-bound claim gates for throughput, p99 latency, socket errors, and non-2xx responses.
- Hardened the reproducible benchmark harness to run CPU-only routes for Kruda, Fiber, and Actix with warmups, multiple rounds, latency profiles, throughput profiles, raw logs, and summaries.

### Fixed
- Fixed static response cache keys to include status codes and avoid cross-status cache reuse.
- Preserved normal handler, middleware, lifecycle, cookie, CORS, and secure-header behavior outside explicitly documented static bypass routes.

## [1.2.3] — 2026-05-23

### Fixed
- Preserved response headers on fast response paths, including secure-header middleware output and net/http responses.
- Preserved overflow Wing headers so responses with more than the fixed inline header slots still serialize all configured headers.
- Preserved session cookie attributes when destroying sessions so deletion cookies keep the configured security policy.

### Changed
- Updated the documented Go patch baseline for currently supported secure toolchains.
- Tightened CI and benchmark reporting guardrails, including workflow timeouts, docs-only benchmark skips, standalone module coverage, and summarized benchmark output.

## [1.2.2] — 2026-05-20

### Fixed
- Replaced deprecated `reflect.Ptr` aliases with `reflect.Pointer` so the Linux Go 1.25.8 CI lint gate passes with current `golangci-lint`.

## [1.2.1] — 2026-05-19

### Fixed
- `App.Listen` now uses the full compile path, including lifecycle flag preparation and OpenAPI route registration.
- App-level DI containers now run `OnInit` before serving requests through `Listen`.
- `Ctx.Context()` now uses the underlying request context for net/http, test, and FastHTTP paths.
- FastHTTP direct serving now exposes `Ctx.Request()` and `Ctx.ResponseWriter()` for contrib middleware.
- Multipart parsing now enforces the configured body limit as a hard cap and maps oversized requests to HTTP 413.
- JWT HMAC signing and middleware reject empty secrets, and middleware enforces the configured algorithm.
- Session memory store now copies session data on save and get to avoid shared mutable map state.
- Cache default keys now include query parameters and hashed Authorization/Cookie headers to avoid cross-request leakage.

### Changed
- WebSocket defaults now set a 1 MiB max message size, 30 second fragmented message timeout, and 10 ping frames per second.
- `scripts/pre-release.sh` now tests standalone contrib modules, the Wing alias module, and the CLI module with temporary local replaces.

### Documentation
- Corrected API docs for `Use`, `Group`, DI lifecycle, health checks, resource route filters, HTTP/3 configuration, contrib README tables, and example metadata.
- Removed stale internal launch planning docs from public documentation.

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
