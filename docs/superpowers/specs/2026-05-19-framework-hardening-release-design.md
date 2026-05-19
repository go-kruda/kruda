# Kruda Framework Hardening Release Design

## Goal

Ship one coordinated hardening release that improves Kruda's runtime correctness, security defaults, documentation accuracy, and release verification without introducing a v2-level API break.

## Scope

This release focuses on defects found during the May 2026 framework review:

- Startup and lifecycle consistency: `Listen` must perform the same compile/startup preparation expected by the framework, and DI lifecycle startup must be explicit and reliable.
- Request context and transport contract: `Ctx.Context`, request accessors, and response writer access must behave consistently across net/http, fasthttp, and test paths.
- Security defaults: JWT, multipart body handling, cache/session/websocket defaults, and error detail behavior must fail safely.
- Documentation accuracy: public docs and contrib READMEs must describe APIs that compile against the current code.
- Release gates: local pre-release checks must match the standalone module coverage already present in CI.

## Non-Goals

- No v2 import removals.
- No broad router rewrite.
- No performance micro-optimization before correctness and release confidence are fixed.
- No public docs in Thai and no AI attribution in public artifacts.

## Design

### Runtime Lifecycle

`App.Listen` should run a single canonical startup path. That path compiles routes, prepares generated metadata, starts DI lifecycle hooks once, and registers shutdown callbacks once. Startup should be idempotent enough for tests to call compile-like paths safely, while still preventing route mutation after compile.

### Context and Transport Contract

Every transport path should populate the same public context surface. `Ctx.Context()` should return the request context when available and fall back to an app/background context only when the transport has no request context. FastHTTP should expose request and writer adapters consistently enough for contrib middleware that relies on headers, remote address, or response writer capabilities.

### Security Defaults

JWT HMAC modes must reject empty secrets. Token parsing must enforce the configured algorithm. Multipart uploads must have a framework-level request size guard before parsing. Cache/session/websocket defaults should avoid cross-user leakage, shared mutable state, and unbounded input.

### Documentation and Release Gates

Docs should be corrected to match current code or code should be changed to match documented behavior only when that behavior is clearly intended and covered by tests. `scripts/pre-release.sh` should verify root, contrib, CLI, and transport alias modules in the same spirit as CI, using temporary replace directives only inside copied or restored module files.

## Testing Strategy

- Add regression tests before runtime/security fixes.
- Run narrow package tests after each fix.
- Run full root tests with `kruda_stdjson`.
- Run standalone contrib and CLI module tests with `GOWORK=off`.
- Run `scripts/pre-release.sh` after release gate updates.
