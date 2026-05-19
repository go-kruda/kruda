# Framework Hardening Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden Kruda's release readiness across runtime lifecycle, security defaults, docs accuracy, and release verification.

**Architecture:** Keep the public v1 API stable where practical. Add focused regression tests for each behavior, then implement the minimum runtime/security changes needed to make the tests pass. Treat docs and release tooling as first-class release artifacts.

**Tech Stack:** Go 1.25+, `go test`, `kruda_stdjson` build tag, standalone Go modules under `contrib/*`, `cmd/kruda`, and `transport/wing`.

---

## File Map

- `kruda.go`, `app_serve.go`, `test_client.go`: canonical startup, compile, and lifecycle flow.
- `container.go`: DI lifecycle semantics.
- `ctx_lifecycle.go`, `ctx_state.go`, `nethttp_adapter.go`, `serve_fast.go`, `transport/fasthttp.go`: request context and transport adapters.
- `contrib/jwt/*`: JWT secret and algorithm enforcement.
- `bind.go`, `transport/nethttp.go`, `transport/fasthttp.go`: request body limit behavior.
- `contrib/cache/*`, `contrib/session/*`, `contrib/ws/*`, `contrib/ratelimit/*`: safer contrib defaults and regression tests.
- `docs/api/*`, `docs/guide/*`, `contrib/*/README.md`, `examples/README.md`: docs drift fixes.
- `scripts/pre-release.sh`: local release verification parity.

## Task 1: Startup and DI Lifecycle

- [ ] Write failing tests showing `Listen` startup performs full compile-time route preparation and starts lazy DI lifecycle hooks exactly once.
- [ ] Run the narrow tests and confirm they fail because startup preparation is missing.
- [ ] Refactor startup into a canonical internal method used by `Listen`, test client startup, and direct compile paths.
- [ ] Wire `Container.Start(ctx)` into app startup with shutdown preserving existing callback order.
- [ ] Run the narrow tests and full root package tests.

## Task 2: Request Context and Transport Adapter Consistency

- [ ] Write failing tests showing `Ctx.Context()` observes request cancellation/deadline in net/http and test paths.
- [ ] Write failing tests for FastHTTP public request/writer accessors where middleware needs headers or writer behavior.
- [ ] Populate request context and public adapters consistently during context reset/serve paths.
- [ ] Run root tests and affected contrib tests.

## Task 3: JWT Security Hardening

- [ ] Write failing tests for empty HMAC secret rejection and configured algorithm enforcement.
- [ ] Reject empty HMAC secrets during middleware creation or first token parse with a clear configuration error.
- [ ] Enforce `Config.Algorithm` during token validation.
- [ ] Update `contrib/jwt/README.md` examples and warnings.
- [ ] Run standalone `contrib/jwt` tests with `GOWORK=off`.

## Task 4: Body Limit and Multipart Guard

- [ ] Write failing tests proving oversized multipart requests are rejected before full parsing.
- [ ] Add a framework-level hard cap for request bodies in net/http and fasthttp paths.
- [ ] Preserve existing bind behavior for valid multipart and JSON requests.
- [ ] Run root tests and upload-related examples if present.

## Task 5: Contrib Safety Defaults

- [ ] Add cache key tests covering query-sensitive and auth-sensitive requests.
- [ ] Add session store tests proving callers cannot mutate stored map state through returned values.
- [ ] Add websocket tests for max message size and origin/subprotocol behavior.
- [ ] Add rate-limit cleanup regression tests for active rejected clients.
- [ ] Implement the smallest compatible defaults and extension points needed for those tests.
- [ ] Run standalone tests for cache, session, ws, and ratelimit.

## Task 6: Docs and Example Accuracy

- [ ] Correct `Group`, `Use`, DI lifecycle, health, resource options, TLS/HTTP3, contrib README, and example count drift.
- [ ] Remove or rewrite public Thai/internal launch notes from public docs.
- [ ] Add or update compile-backed snippets where practical.
- [ ] Run documentation grep checks for stale API names found in research.

## Task 7: Release Gate Parity

- [ ] Update `scripts/pre-release.sh` to test root, `cmd/kruda`, every `contrib/*` module, and `transport/wing` alias module.
- [ ] Ensure module file mutations are temporary or restored by traps.
- [ ] Run the updated pre-release script.
- [ ] Run a fresh external-module import check for `github.com/go-kruda/kruda` and selected contrib modules.

## Task 8: Final Verification and Release Prep

- [ ] Run root `go test -tags kruda_stdjson ./...`.
- [ ] Run standalone module tests for contrib, CLI, and transport alias.
- [ ] Run `scripts/pre-release.sh`.
- [ ] Review `git diff` for public language, docs accuracy, and no AI attribution.
- [ ] Prepare release notes covering runtime lifecycle, security hardening, docs corrections, and release verification changes.
