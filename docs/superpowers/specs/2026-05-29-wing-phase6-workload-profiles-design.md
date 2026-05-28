# Wing Phase 6 Workload Profiles Design

Date: 2026-05-29
Status: Draft

## Goal

Phase 6 starts a new performance track after Phase 5 closed with no accepted fair-handler runtime candidate.

The goal is to design workload-specific Wing profiles that compose multiple Feathers behind clear, opt-in boundaries. This is not a new benchmark claim and not a runtime change by itself.

## Context

Phase 5 showed that the fair-handler CPU-bound path is already dominated by Linux socket syscalls on the tiger benchmark server. Small hot-path edits did not produce a broad balanced win across `plaintext-handler`, `json-static`, and `json-serialize`.

Phase 6 should therefore stop treating "one more fair-handler micro-optimization" as the main route. The better direction is workload fit:

- Public static hot paths.
- CPU-bound handler paths.
- JSON serialization paths.
- Short DB/Redis query paths.
- Render/template paths.
- Memory-sensitive short-header services.
- Real-world mixed API services.

## Vocabulary

Phase 6 keeps the existing Wing flight model:

- Transport is the HTTP backend adapter, such as `net/http`, `fasthttp`, or `Wing`.
- Wing is the performance-oriented transport family/profile surface.
- Feather is a tunable component inside a Wing.
- Bone is non-negotiable correctness and security structure.

One route should select at most one Wing profile. That profile may contain many Feathers. A route should not stack several Wing profiles because that makes dispatch, fallback behavior, docs, and benchmark evidence hard to reason about.

## Existing Feathers

Current code already exposes or contains these Feather concepts:

- Dispatch mode: `Inline`, `Pool`, `Spawn`, and `Takeover`.
- Response mode: plaintext and JSON-specialized response handling.
- Static response: prebuilt full HTTP response for explicit bypass paths.
- Feather table: route-specific lookup and dispatch hints.
- Worker count: `KRUDA_WORKERS`.
- Handler pool size: `KRUDA_POOL_SIZE`.
- Read buffer size: `KRUDA_READ_BUF_SIZE`.
- Per-route env dispatch hints: `KRUDA_POOL_ROUTES` and `KRUDA_SPAWN_ROUTES`.

These are enough to design profiles before adding new public APIs.

## Profile Boundary

A Wing profile is a named bundle of Feathers for a workload. It should answer:

- Which workload it targets.
- Which Feathers it enables.
- Which Bones it preserves.
- Which behavior is different from default Wing.
- Which benchmark command proves it helps.
- Which routes should not use it.

Profiles must be opt-in. Default Kruda behavior must remain unchanged.

## Proposed Profile Families

### CPU Handler Profile

Target:

- CPU-only handlers with no external I/O.
- Current examples: plaintext, static JSON handler, JSON serialization handler.

Likely Feathers:

- Inline dispatch.
- Existing route handler fast dispatch.
- Response mode specialization.
- Current balanced worker/read-buffer settings only when the workload proves them safe.

Boundary:

- Must preserve handler, middleware, lifecycle hooks, panic recovery, CORS, cookies, secure headers, parser safety, and safe-copy semantics.
- Must not use `WingStaticText` or `WingStaticJSON` evidence for fair handler claims.

Current state:

- Phase 5 closed this path for broad fair-handler CPU wins. Do not restart it unless the workload, kernel, Go runtime, or benchmark topology materially changes.

### Static Bypass Profile

Target:

- Public, immutable, static hot paths such as health checks, plaintext, or constant JSON where bypassing the framework pipeline is acceptable.

Likely Feathers:

- `WingStaticText`.
- `WingStaticJSON`.
- Prebuilt response cache.

Boundary:

- Bypasses handler, middleware, lifecycle hooks, cookies, CORS, secure headers, and normal error handling.
- Must never be mixed into fair handler-path benchmark claims.
- Must be documented as a bypass profile.

### JSON Handler Profile

Target:

- Normal handler-path JSON APIs where serialization cost is meaningful.

Likely Feathers:

- JSON response mode specialization.
- Typed/static encoder experiments.
- Build-tag-aware serializer comparison.

Boundary:

- Must remain handler-path.
- Must preserve middleware/lifecycle behavior.
- Can support JSON-specific claims only, not broad plaintext/static JSON claims.

### Query Profile

Target:

- Short DB, Redis, or network I/O handlers where handler wait time dominates dispatch overhead.

Likely Feathers:

- `Pool` dispatch for bounded concurrency.
- `Takeover` only when route evidence shows it improves p99 and throughput.
- Handler pool sizing.
- Timeout and backpressure settings.

Boundary:

- Must not be reused for CPU-bound fair-handler routes unless evidence changes.
- Must report saturation behavior, timeout behavior, and error rate.

### Render Profile

Target:

- DB plus template/HTML rendering.

Likely Feathers:

- `Takeover` or `Pool` dispatch.
- Response builder profiling.
- Backpressure-focused settings.

Boundary:

- Must preserve response ordering and pipelining semantics.
- Must not be measured with CPU-only routes.

### Memory-Sensitive Profile

Target:

- High-connection-count services with short headers where RSS matters more than absolute peak RPS.

Likely Feathers:

- `KRUDA_READ_BUF_SIZE` tuning.
- Worker count tuning.
- Pool sizing.

Boundary:

- Smaller read buffers can reject requests whose request line and headers do not fit the configured buffer.
- This profile needs request-size assumptions and should not become the default.
- Evidence must report RSS, RPS, p99, socket errors, and non-2xx responses.

## Non-Negotiable Bones

No Phase 6 profile may weaken these by default:

- HTTP parser safety, header count limits, and header size limits.
- CRLF rejection.
- Duplicate `Content-Length` rejection.
- `Transfer-Encoding` plus `Content-Length` rejection.
- Path, query, body, content type, cookie, and retained header safe-copy semantics.
- Middleware, lifecycle hooks, panic recovery, CORS, cookies, secure headers, and error handling on normal handler routes.
- Response ordering, pipelining behavior, fd ownership, timeout behavior, and backpressure behavior.

If a profile bypasses any of these, it must be explicitly named as a bypass profile and kept out of fair handler-path claims.

## Implementation Strategy

Phase 6 should start with documentation and evidence, then move to runtime only after the boundaries are stable.

Suggested PR sequence:

1. Add this Phase 6 design spec.
2. Add an internal profile inventory that maps current route options and env knobs to profile families.
3. Add targeted benchmark scripts or notes for one non-Phase-5 workload.
4. Prototype one profile at a time behind existing options or an env-gated internal setting.
5. Add public APIs only after evidence shows the profile is useful and the behavior boundary is clear.

## First Runtime Candidate

The first runtime candidate should not be another fair-handler CPU-bound change.

The recommended first candidate is a JSON handler profile investigation:

- Compare stdlib JSON, Sonic, and the current build-tag behavior on `json-serialize`.
- Measure handler-path JSON serialization microbenchmarks with `-benchmem -count=5`.
- Run tiger route evidence only for JSON routes.
- Keep claims JSON-specific.

This is narrow enough to avoid reopening Phase 5, but still useful for real APIs.

## Evidence Gate

A Phase 6 profile can move from research to runtime only if it includes:

- Workload name and route set.
- Exact build flags and env settings.
- Before/after microbenchmarks where CPU hot paths are touched.
- End-to-end benchmark with RPS, p50, p90, p99, max latency, socket errors, and non-2xx counts.
- Resource evidence when the profile claims memory or CPU-efficiency improvement.
- Clear statement of which Bones are preserved or which bypasses are intentional.

## Non-Goals

- No runtime behavior change in this PR.
- No public API change in this PR.
- No new Actix claim in this PR.
- No release, tag, or version bump in this PR.
