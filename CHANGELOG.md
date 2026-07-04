# Changelog

All notable changes to Kruda are documented in this file.
Format based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Startup warning when `Use()` is called after routes are registered (the
  middleware would silently not apply to the already-registered routes).
- Docs: Wing protocol support matrix (`docs/guide/wing-protocol-support.md`) and
  production TLS deployment guidance (terminate in front of Wing).

### Changed

- Removed the stale `transport/wing v1.1.3` require from `go.mod` (the shim
  directory was removed in v1.3.0; nothing imports it).
- Docs: corrected the stale "Wing doesn't support SSE" transport limitation —
  SSE works via the `kruda.Stream` preset since v1.5.0.
- Docs: corrected `App.Serve` — it claimed systemd socket activation
  unconditionally, but Wing (default on Linux) closes the passed listener and
  re-binds one `SO_REUSEPORT` socket per worker, so it cannot serve an inherited
  fd. The net/http and fasthttp transports serve the supplied fd directly; the
  doc comment and transport guide now say which transports honor fd-passing.
- MCP server (`kruda mcp`): corrected stale AI-facing guidance that predated the
  streaming/WebSocket presets. `kruda_docs` gains a `websocket` topic and its
  `sse`/`wing` topics now document the `kruda.Stream` preset (v1.5.0+) instead of
  claiming SSE/streaming has no Wing preset; `kruda_suggest_wing` now suggests
  `kruda.Stream` for SSE/streaming routes and `kruda.Hijack` for WebSocket routes
  instead of "none". Keeps AI-generated code aligned with real-time on Wing.

## [1.6.0] — 2026-07-04

### Security

- **`contrib/ws` frame-parser hardening (RFC 6455 §5).** `readFrame` now rejects
  undefined/reserved opcodes, fragmented control frames, and control frames whose
  declared payload exceeds 125 bytes (the last closes an unbounded-allocation DoS
  that was reachable before the payload buffer was sized), and rejects non-minimal
  length encodings. `Conn.ReadMessage` now rejects unmasked client frames
  (RFC 6455 §5.1). These gaps were latent while WebSocket ran only on net/http;
  they are fixed here as WebSocket support extends to the Wing transport.

### Fixed

- Wing no longer drops request headers: requests carrying more than 8
  non-fast-path headers now spill into a heap slice instead of being silently
  dropped past the 8th. Real browser requests (e.g. a Chrome WebSocket upgrade
  carries ~10-12 non-fast headers) previously lost every extra header past the
  8th — `c.Header()` returned "" for them. The inline fast path (≤8 extra
  headers) is unchanged, so the zero-extra-header hot path keeps its footprint.
- Wing now retains the `Connection` request header value, so
  `c.Header("Connection")` returns it. Previously Wing parsed `Connection` only
  to derive keep-alive and dropped the raw value (always returning ""), which
  broke the WebSocket upgrade handshake (`Connection: Upgrade`) on Wing. The fix
  uses the same zero-copy path as `Host`/`Accept`, so the hot path stays zero-alloc.

### Added

- **WebSocket on the Wing transport.** `contrib/ws` now works on Wing (the
  default on Linux), previously net/http-only. Register with
  `ws.HandleFunc(app, "/ws", handler)` — identical on every transport. Internally
  Wing hands the taken-over connection to `contrib/ws` via the standard
  `http.Hijacker` contract (a new generic `kruda.Hijack` route preset); the
  RFC 6455 frame code is reused unchanged. Dispatches via Takeover, so the inline
  hot path is untouched. A pipelined first frame is preserved, a rejected upgrade
  returns a clean 4xx, and on shutdown Wing wakes I/O-blocked handlers
  (`SHUT_RDWR`) and signals `conn.Done()` so a handler blocked in application
  logic can exit cooperatively. The fasthttp transport still does not support
  WebSocket.
- `App.Serve(ln net.Listener)` — run the app on a pre-created listener instead of
  binding an address, for graceful restart, systemd socket activation, or tests
  that need the bound address before the server accepts (no signal handler; the
  caller owns the lifecycle via `Shutdown`).
- `WingHeaderSpills()` — process-wide counter of requests whose extra headers
  spilled past the inline capacity (observability for header-heavy traffic).

## [1.5.0] — 2026-06-29

### Added

- **Server-Sent Events and streaming on the Wing transport.** `c.SSE()` and `c.Stream()`
  now work on Wing (the default on Linux) via the new `kruda.Stream` route preset —
  `app.Get("/events", h, kruda.Stream)`. Previously they returned an error unless the route
  ran on net/http. Streaming dispatches via Takeover and does not affect the inline hot
  path; a slow/stuck client is bounded by `WriteTimeout`, and a client disconnect cancels
  the handler context (`SSEStream.Done()` fires). A `TestClient.SSE(path)` helper decodes the
  emitted events for unit tests. The fasthttp transport (macOS dev default) does not support
  streaming — `c.SSE()` there now returns an actionable error pointing to `kruda.NetHTTP()`.
- **`WithHeaderLimit(n)` option.** Configures the maximum total request-header size
  (default 8 KB → HTTP 431). Clients sending large `Authorization`/`Cookie` headers (big
  JWTs) previously hit a spurious 431 with no escape hatch.
- **Environment configuration for the Wing safety bundle.** `WithEnvPrefix` now also reads
  `<PREFIX>_HEADER_LIMIT`, `<PREFIX>_TRUST_PROXY`, `<PREFIX>_MAX_CONNS`,
  `<PREFIX>_MAX_CONNS_PER_IP`, `<PREFIX>_ACCEPT_RATE_PER_SEC`, and
  `<PREFIX>_ACCEPT_RATE_BURST`, so Kubernetes/ConfigMap deployments can tune the v1.4.0
  accept-side DoS limits and proxy trust without code changes.

### Security

- Raised the minimum Go to **1.25.11** (1.25 line) or **1.26.4** (1.26 line). go1.25.10
  and go1.26.0–1.26.3 carry stdlib CVEs GO-2026-5037 (crypto/x509 quadratic verify) and
  GO-2026-5039 (net/textproto error-message injection); the new floor clears both.

### Breaking

- **Removed the no-op `WithHTTP3` option and the `Config.HTTP3` field.** They advertised
  HTTP/3 (QUIC) serving, but it was never implemented — nothing consumed the flag, so a
  caller got the standard net/http fallback (HTTP/1.1 + HTTP/2), not QUIC. HTTP/3 is not on
  the roadmap, so the dead API is removed rather than left as a misleading promise
  (rationale: `docs/decisions/0001-break-api-in-v1-minor.md`). TLS is unaffected — use
  `WithTLS(certFile, keyFile)`.

## [1.4.0] — 2026-06-27

### Breaking

The Wing accept-limit work removed the now-dead per-worker connection field in favor
of server-wide options. With effectively zero external adopters, the removal ships in a
v1 minor release — rationale in `docs/decisions/0001-break-api-in-v1-minor.md`. Migration:

| Removed | Replacement |
|---------|-------------|
| `WingConfig.MaxConnsPerWorker` | `kruda.WithMaxConns(n)` (total cap) — plus `kruda.WithMaxConnsPerIP(n)` and `kruda.WithMaxAcceptRate(perSec, burst)` for per-IP and accept-rate limits |

The semantics also change from a **per-worker** cap to a **server-wide total** cap, so size
the new value as the old per-worker limit times the worker count (workers default to
`runtime.NumCPU()`):

```go
// before — per-worker cap (effective total ≈ MaxConnsPerWorker × worker count)
kruda.New(kruda.Wing(kruda.WingConfig{MaxConnsPerWorker: 1024}))
// after — single server-wide total cap (e.g. 1024 × 8 workers)
kruda.New(kruda.Wing(), kruda.WithMaxConns(8192))
```

### Fixed

- **Wing now honors the request-size contract on both read paths.** The default
  Wing transport previously ignored `BodyLimit`/`HeaderLimit`/`TrustProxy` and
  silently dropped any request body larger than its fixed read buffer (~8 KB).
  Wing now accepts legal bodies of any size up to `BodyLimit` (incrementally
  accumulated, capped by `BodyLimit` plus a per-worker in-flight budget) and
  returns deterministic **413** (body over limit), **431** (headers over limit),
  and **501** (chunked request bodies — Wing does not dechunk) before the handler
  runs, on both the event-loop and Takeover paths. `Expect: 100-continue` is
  answered (or rejected with 413) before the body is read, read deadlines bound
  the whole request read, and `TrustProxy` reads `X-Forwarded-For`/`X-Real-IP`
  with the same boolean semantics as net/http. The no-body hot path is unchanged
  (parser fast path frozen, 0-alloc guard; tiger A/B shows no regression).
- **Wing no longer double-closes a Takeover connection's fd on shutdown.**
  Graceful shutdown discarded the `*os.File` that owns a Takeover connection's
  fd without closing it while also raw-closing the same fd; the leaked File's
  finalizer then closed the fd after the kernel had recycled the number for a
  new socket, surfacing as intermittent `bad file descriptor` errors on a
  subsequent `net.Listen`/`net.Dial` when Takeover-preset servers were restarted
  under load. Shutdown now closes each Takeover fd exactly once.
- **Wing bounds total in-flight body memory across both read paths.** The Takeover
  path now charges and refunds the per-worker in-flight body budget
  (`MaxInflightBodyBytes`), returning **503** when the budget is exceeded, so the
  event-loop and Takeover paths share one bound on concurrent body memory. Pipelined
  bytes that arrive in the same read as a request body are preserved — surplus past
  `Content-Length` is relocated, and already-buffered pipelined bytes are drained once
  the body completes (edge-triggered epoll does not re-notify for bytes already in the
  read buffer).
- **Logger middleware records real request latency.** The built-in Logger reported a
  latency of `0` for every request; it now measures and logs the actual handler
  duration.
- **Prometheus in-flight gauge no longer leaks on panic.** The in-flight request gauge
  `Dec` is now deferred, so a panic inside a handler can no longer leave the gauge
  permanently incremented.

### Added

- **Typed test-client helpers for typed handlers.** `GetTyped`, `PostTyped`,
  `PutTyped`, `PatchTyped`, `DeleteTyped`, and `SendTyped` wrap the existing
  in-memory `TestClient` and decode JSON responses into a typed
  `TypedTestResponse[T]`, while body helpers accept typed request values.
- **Richer OpenAPI metadata for typed routes.** Generated OpenAPI 3.1 specs can
  now include explicit security schemes, per-route security requirements,
  request/response examples, and error response content schemas aligned with
  the default Kruda error shape or `WithProblemJSON()`.
- **Opt-in RFC 9457 problem+json error responses.** `kruda.New(kruda.WithProblemJSON())`
  renders errors as `application/problem+json` (standard members plus a field-level
  `errors` array for validation failures). Off by default — the standard error shape is
  unchanged; `WithErrorHandler` still takes precedence.
- **Fluent `KrudaError` builders** — `WithType`, `WithDetail`, `WithInstance`, and
  `With(key, value)` for problem `type`/`detail`/`instance`/extension members, e.g.
  `kruda.NotFound("…").WithType("https://…").With("userId", id)`.
- **Wing accept-side DoS limits.** New options `WithMaxConns(n)`,
  `WithMaxConnsPerIP(n)`, and `WithMaxAcceptRate(perSec, burst)` cap accepted
  connections at the accept path: a global total cap (CAS-reserved; when unset, derived
  from `RLIMIT_NOFILE` at Wing startup), an opt-in per-IP concurrent-connection limit,
  and an accept-rate token bucket. Enforcement is accept-only — over-limit connections
  are closed with a TCP **RST**, never an HTTP **503**. The peer IP is captured zero-alloc
  via raw `accept4`. These thread the new `WingConfig` fields `MaxConns`,
  `MaxConnsPerIP`, `AcceptRatePerSec`, and `AcceptRateBurst`. Per-IP caps key on the
  socket peer address, not `X-Forwarded-For`. Linux/epoll only enforces the accept path;
  the no-limit hot path is unchanged (tiger A/B shows no regression).
- **Turnkey observability — new `contrib/observability` module.** A single
  `observability.Enable(app, cfg)` call wires OpenTelemetry tracing (otel v1.44.0),
  RED metrics, trace/log correlation, Kubernetes liveness/readiness probes, and a
  `/metrics` endpoint. The core gains a `WithLogEnricher(fn)` option and
  `App.SetLogEnricher(fn)` seam for attaching per-request log attributes (e.g. the
  active trace/span IDs).
- **Auto-CRUD `Resource` input validation and OpenAPI 3.1 schemas.**
  `kruda.Resource` create/update routes now decode and validate the request body
  through the typed-handler contract (gated identically to typed routes — Validator
  configured and `T` carrying validation tags), return **400** on bad pagination, and
  emit explicit OpenAPI 3.1 operations (list/get/create/update/delete) with correct
  `200`/`201`/`204` status codes, page/limit query params, a request body, a `422` only
  when validation is engaged, and the same default error response as typed routes.
  Generic instantiation component names (e.g. `ResourceList[…/models.User]`) are
  sanitized to valid JSON-pointer `$ref` names.

## [1.3.1] — 2026-06-13

### Changed

- **Wing: adaptive spin before the netpoller park on Takeover keep-alive reads.**
  Recovers the `/db` and `/queries` throughput-p99 wake-hop introduced by the
  v1.3.0 netpoll takeover. On the default build versus Fiber, every DB-route cell
  now beats Fiber on p99 (−6.6% to −28.9%) while matching it on RPS at the pgx
  ceiling — the v1.3.0 "trades a few milliseconds of db/queries p99" caveat is
  removed. The spin is bounded (`takeoverSpinReads`) and reached only by Takeover
  dispatch (DB/Render routes); DB-bound routes have idle CPU, so it trades that
  idle for a shorter tail at no throughput cost. Evidence:
  `bench/reproducible/results/2026-06-13-takeover-spin-p99-evidence.md` and
  `2026-06-13-v1-3-1-consolidated-evidence.md`.

### Added

- `bench/reproducible/footprint.sh` — runtime footprint measurement (binary size,
  startup time, RSS).
- `bench.sh` opt-in low-concurrency profiles (`BENCH_LOWC=1` → c8/c16/c32).
- `TestStringLaneZeroAlloc` — guards the Wing string lane's zero-allocation
  property against regression.

### Notes — performance audits (no code change)

- Allocation: the Wing hot path is already allocation-optimal (Ctx pool and the
  string lane are 0 allocs/op). Footprint: Linux startup 5 ms, idle RSS 13.8 MB.
  Low-concurrency: Kruda leads RPS at every concurrency and leads p99 from c16 up.
  Evidence files under `bench/reproducible/results/2026-06-13-*`.

## [1.3.0] — 2026-06-13

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
| `github.com/go-kruda/kruda/transport/wing` | removed — import `github.com/go-kruda/kruda` (tags up to `transport/wing/v1.1.3` keep working for pinned users) |

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

### Changed
- Takeover dispatch parks connections on the runtime netpoller instead of
  pinning one OS thread per connection (~24 threads instead of ~250 at 256
  connections). DB-route benchmarks gain +6% to +20% RPS at identical CPU
  per request; median latency improves while `db`/`queries` p99 grows by a
  few milliseconds from the extra netpoller wake hop. Forensics and paired
  A/B: `bench/reproducible/results/2026-06-12-wing-netpoll-takeover-evidence.md`.

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
