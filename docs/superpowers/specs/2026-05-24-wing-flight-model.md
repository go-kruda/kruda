# Wing Flight Model

Date: 2026-05-24
Status: Design vocabulary

## Goal

Define the architecture vocabulary for future Wing performance work so runtime changes, route options, benchmark claims, and documentation use the same mental model.

Kruda should keep correctness, security, and normal framework behavior as the default contract. Performance-specific shortcuts can exist only when they are explicit, opt-in, benchmark-backed, and documented with their behavior boundary.

## Core Vocabulary

### Transport

The transport is the HTTP backend adapter that carries requests into Kruda while preserving the framework contract.

Current transports:

- Wing: custom async I/O transport using epoll on Linux and kqueue on macOS.
- fasthttp: optimized HTTP/1.1 transport used by default on macOS.
- net/http: standard transport for Windows, TLS, HTTP/2, SSE, and maximum compatibility.

Transport choices are application-level decisions. They set the broad runtime behavior, protocol support, and portability tradeoff.

### Wing

Wing is Kruda's performance-oriented transport family and profile surface. It is where Kruda can specialize for high-throughput HTTP/1.1 workloads while still preserving the normal handler path by default.

A Wing is composed from multiple Feathers chosen for a workload. One Wing is not one option. For example, a CPU-bound Wing profile may combine inline dispatch, route lookup specialization, response serialization strategy, buffer policy, event-loop policy, and Linux write strategy. A query/render Wing profile may combine blocking-I/O isolation, timeout-aware dispatch, and backpressure-oriented Feathers.

Wing is not a license to weaken framework behavior. Normal Wing handler routes must still run the handler pipeline, middleware, lifecycle hooks, cookies, CORS, secure headers, path traversal checks, HTTP safety limits, panic recovery, and error handling according to their existing contracts.

### Feather

A Feather is a tunable component that helps a Wing fit a workload. Feathers should be composable and explicit. A Feather can be a function, config, route hint, dispatch strategy, response strategy, syscall policy, buffer policy, event-loop policy, or fast-path technique.

Existing Feather concepts include:

- Dispatch mode: inline worker execution, pool execution, spawn execution, or takeover execution.
- Response mode: plaintext or JSON-specialized response handling.
- Static response: an explicit prebuilt response bypass for public static hot paths.
- Route metadata: route-specific hints used by Wing's Feather table.

Future Feathers should be introduced only when they make a specific workload faster without changing the default framework contract.

### Bone

Bone is internal design vocabulary for the non-negotiable correctness and security structure that lets Wing remain a framework, not just a benchmark server. Bones are not public API at this stage, and public docs should keep using direct terms such as framework contract, HTTP safety, and lifecycle behavior.

Bones include:

- HTTP parser safety, including header count/size limits, CRLF rejection, duplicate `Content-Length` rejection, and `Transfer-Encoding` plus `Content-Length` rejection.
- Connection state machine correctness, including keep-alive, close, shutdown, and fd ownership.
- Middleware, lifecycle hook, panic recovery, error handling, cookies, CORS, and secure-header contracts.
- Timeout and backpressure behavior.
- Response ordering and pipelining semantics.
- Safe-copy semantics for request path, query, body, content type, cookies, and retained header values.

Feathers are tunable. Bones are not optional. A candidate Feather that weakens a Bone must be rejected or moved behind a clearly documented bypass boundary that is not used for fair handler-path claims.

## Default Contract

The default Kruda route is a normal framework route. It must preserve:

- Handler execution.
- Middleware and lifecycle hooks.
- Panic recovery and error handling.
- Request safety checks, including path traversal handling where configured.
- Header/body safety limits and request smuggling defenses.
- Cookies, CORS, and secure-header injection where enabled.
- Observability hooks and user-visible behavior.

This default contract is what public handler-path benchmark claims should measure.

## Optional Feather Boundary

An optional Feather may change behavior only when all of these are true:

- The user opted into the route option, environment setting, or documented config.
- The behavior difference is explicit in docs and tests.
- The behavior is not a request-smuggling, header-injection, timeout, or lifecycle safety risk.
- The benchmark evidence separates the optional path from fair normal-handler claims.
- The feature can be disabled or avoided without changing application code outside that route/config.

Examples:

- `WingStaticText` and `WingStaticJSON` are valid optional Feathers for public static hot paths because they explicitly bypass the handler pipeline and document that boundary.
- Handler-level `SendStaticJSON` is not a bypass Feather. It is a normal handler-path optimization and can be used in fair handler benchmarks.
- Single-handler fast dispatch is a normal handler-path Wing optimization because it still calls the handler and falls back when middleware or lifecycle hooks are present.

## Benchmark Vocabulary

Benchmark evidence must say which layer it measures:

- Transport-level: socket/event-loop/parser behavior.
- Wing handler-path: normal Kruda handler pipeline running on Wing.
- Feather-assisted handler-path: normal handler path with route hints such as `WingJSON`.
- Static bypass: prebuilt Wing response that skips handler/middleware/lifecycle.

Public "faster than Actix" claims should use normal handler-path evidence unless the claim explicitly says it is measuring an opt-in bypass path.

## Design Rules For New Feathers

1. Start from the workload: plaintext, static JSON, serialized JSON, DB wait, rendering, streaming, file I/O, or mixed application behavior.
2. Preserve the default route contract unless the Feather is explicitly documented as a bypass.
3. Add tests for both the fast path and the fallback path.
4. Add microbenchmarks before and after the change when the Feather touches a CPU hot path.
5. Re-run reproducible benchmarks before changing public performance wording.
6. Keep names tied to behavior, not implementation accidents.
7. Avoid adding knobs that only benchmark well on one machine unless their workload boundary is narrow and documented.

## Non-Goals

- No new public API is introduced by this document.
- No runtime behavior changes are made by this document.
- No new performance claim is made by this document.
- No release, version bump, or tag is implied.
