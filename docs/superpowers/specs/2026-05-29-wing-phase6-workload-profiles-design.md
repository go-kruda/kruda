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

- Dispatch mode: `Inline`, `Pool`, `Spawn`, and `Takeover`, applied with `WingFeather`.
- Response mode: plaintext and JSON-specialized response handling.
- Static response: prebuilt full HTTP response for explicit bypass paths.
- Feather table: route-specific lookup and dispatch hints.
- Worker count: `KRUDA_WORKERS`.
- Handler pool size: `KRUDA_POOL_SIZE`.
- Read buffer size: `KRUDA_READ_BUF_SIZE`.
- Per-route env dispatch hints: `KRUDA_POOL_ROUTES` and `KRUDA_SPAWN_ROUTES`.

These are enough to design profiles before adding new public APIs.

## Profile Inventory

| Feather | Current Surface | Profile Fit | Preserves Normal Handler Contract | Notes |
|---|---|---|---:|---|
| Inline dispatch | `Bolt`, `WingFeather(Bolt)`, default Wing dispatch | CPU handler, JSON handler | Yes | Best measured fit for CPU-bound fair-handler routes. |
| Pool dispatch | `Arrow`, `WingFeather(Arrow)`, `KRUDA_POOL_ROUTES`, `KRUDA_ASYNC=1` | Query, short external I/O | Yes | Adds dispatch overhead; use only when handler wait time dominates. |
| Takeover dispatch | `Spear`, `WingFeather(Spear)`, `WingQuery`, `WingRender` | Query, render, blocking I/O | Yes | Rejected for CPU-bound routes; may still fit DB/Redis/render workloads. |
| Spawn dispatch | `WingFeather(Feather{Dispatch: Spawn})`, `KRUDA_SPAWN_ROUTES` | Variable-latency handlers | Yes | Higher overhead than Pool; requires workload evidence. |
| Plaintext response mode | `WingPlaintext` | CPU handler | Yes | Handler still runs. |
| JSON response mode | `WingJSON` | JSON handler | Yes | Handler still runs. Route-level JSON serializer gains are currently small. |
| Static full response | `WingStaticText`, `WingStaticJSON`, `Static` | Static bypass | No | Bypasses handler, middleware, lifecycle hooks, cookies, CORS, secure headers, and normal error handling. |
| Worker count | `KRUDA_WORKERS` | CPU handler, memory-sensitive | Yes | Tiger CPU-bound evidence currently supports 4 workers with 4 wrk threads. |
| Read buffer size | `KRUDA_READ_BUF_SIZE` | Memory-sensitive | Yes, with request-size assumptions | Smaller buffers reduce RSS for short-header workloads but reject oversized request lines/headers. |
| Handler pool size | `KRUDA_POOL_SIZE` | Query, render | Yes | Must be measured with saturation and p99 evidence. |

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

Do not repeat without new evidence:

- CPU affinity prototype.
- Epoll idle-spin prototype.
- Removing post-send speculative read.
- Inline scratch response candidate.
- Parser skip-extra-header candidate.
- `Takeover` for CPU-bound routes.
- Worker count increase from 4 to 8 for the current tiger throughput profile.
- Lowering `GOMAXPROCS` from 8 to 4 for the current tiger throughput profile.
- Concrete Wing responder assertion in `Ctx.Text` or `Ctx.JSON`; local microbenchmarks rejected it on 2026-05-30.
- Moving the Wing `JSONResponder` check before the fasthttp static-body path in `SendStaticWithTypeBytes`; local microbenchmarks on 2026-06-02 showed no meaningful improvement for `BenchmarkCPUHandlerJSONStaticBytesFeather`.

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

After JSON route evidence, the next non-Phase-5 workload is Query Profile evidence:

- Use `BENCH_ENABLE_DB=1 ./profile-kruda.sh db queries fortunes updates` for Kruda-only CPU profiles.
- Use `./sweep-kruda-db-dispatch.sh db queries fortunes updates` for repeatable Kruda-only dispatch matrix runs.
- Use explicit DB routes rather than the default CPU-only route set.
- Compare `BENCH_KRUDA_DB_DISPATCH=takeover`, `pool`, `spawn`, and `inline` only as route-level A/B evidence.
- For Pool dispatch, sweep `KRUDA_POOL_SIZE` and report it in the environment metadata.
- Keep DB and fortunes evidence out of CPU-bound public benchmark claims.
- A 2026-05-30 tiger smoke run verified that `profile-kruda.sh` can profile `/db` with `BENCH_ENABLE_DB=1`; treat that run as tooling verification only because it was Kruda-only, short, and pprof-enabled.
- Do not use the earlier `phase6-db-dispatch-*` tiger runs as dispatch evidence; they were invalid for `pool` and `spawn` because env Feathers did not carry prebuilt route handlers and fell back to the full router path.

2026-05-30 corrected tiger `/db` route-option sweep:

- Command shape: `BENCH_FRAMEWORKS=kruda BENCH_ENABLE_DB=1 BENCH_ROUNDS=3 BENCH_DURATION=10s ./bench.sh db`.
- `takeover`: median RPS 102673.71 on latency profile and 97064.64 on throughput profile; median p99 7.45 ms and 8.41 ms; zero socket errors and zero non-2xx.
- `inline`: median RPS 43322.24 on latency profile and 43119.28 on throughput profile; median p99 3.98 ms and 7.32 ms; zero socket errors and zero non-2xx.
- `pool` with `KRUDA_POOL_SIZE=64`: median RPS 2117.26 on latency profile and 1117.03 on throughput profile; median p99 2.17 ms and 3.74 ms; zero socket errors and zero non-2xx.
- `spawn`: median RPS 873.37 on latency profile and 2727.92 on throughput profile; median p99 2.10 ms and 3.47 ms; zero socket errors and zero non-2xx.
- Current conclusion: `takeover` remains the DB throughput candidate. Pool and Spawn are not balanced DB throughput candidates for this route; they may only be revisited for a deliberately low-throughput tail-latency profile with a different saturation target.

2026-06-01 tiger `takeover` vs `inline` DB route sweep:

- Command shape: `BENCH_KRUDA_DB_DISPATCH_MODES="takeover inline" BENCH_ROUNDS=3 BENCH_DURATION=10s ./sweep-kruda-db-dispatch.sh queries fortunes updates`.
- `queries`: `takeover` median throughput RPS 92113.21 with p99 5.42 ms; `inline` median throughput RPS 42243.36 with p99 8.12 ms. Both had zero socket errors and zero non-2xx.
- `fortunes`: `takeover` median throughput RPS 83346.68 with p99 5.70 ms; `inline` median throughput RPS 40581.52 with p99 8.15 ms. Both had zero socket errors and zero non-2xx.
- `updates`: `takeover` median throughput RPS 2785.14 with p99 522.42 ms and zero errors; `inline` median throughput RPS 146.16 with p99 1930.00 ms and 320 socket errors.
- Current conclusion: `WingQuery` and `WingRender` should continue to map to `Takeover` for these DB benchmark routes. `Inline` is not a DB profile candidate.

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
