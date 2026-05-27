# Wing CPU Profile Phase 4 Design

Date: 2026-05-27
Status: Draft

## Goal

Design the next Wing performance step as a CPU-bound architecture prototype, not another isolated hot-path tweak. Phase 4 targets fair handler-path routes only and keeps the default Kruda framework contract intact.

The working target is deliberately higher than Phase 3:

- At least 5% median RPS improvement over current `main` on all three fair CPU-bound routes, or at least 10% on one route with no regression on the others.
- p99 latency no worse than 10% above current `main`.
- Zero socket errors and zero non-2xx responses.
- No allocation or B/op increase on CPU-bound hot-path microbenchmarks.
- If the goal is an Actix public win, Kruda still needs to meet the existing public claim gate: median RPS at least 3% above Actix and p99 no worse than 10% above Actix with zero errors/non-2xx.

## Current Reality

Phase 3 and Phase 3B showed that the current Wing CPU-bound handler path is already too optimized for small local edits to move the route-level benchmark reliably. Parser changes, response scratch changes, worker scaling, GOMAXPROCS changes, speculative-read changes, event-loop idle changes, exact-route lookup changes, and blocking wait changes either regressed or stayed below the route-evidence threshold.

The dominant cost center on tiger remains socket/syscall behavior, not JSON serialization, parser work, or route lookup. For non-pipelined HTTP/1.1 keep-alive traffic, each request still requires at least a read-side readiness/read cycle and one response write. Actix pays a similar kernel budget, so a large fair-handler win is unlikely to come from small Go-level cleanups.

## Vocabulary Boundary

- Transport: the HTTP backend adapter, such as `net/http`, `fasthttp`, or Wing.
- Wing: Kruda's performance-oriented transport family/profile surface.
- Feather: a tunable component inside a Wing, such as dispatch policy, response strategy, buffer policy, event-loop policy, or syscall policy.
- Bone: internal vocabulary for correctness/security structure. Bones are not optional and are not public API at this stage.

Phase 4 may create new internal Feathers, but it must not weaken Bones.

## Phase 4 Direction

The next credible path is an internal CPU Wing profile prototype that composes multiple Feathers behind one env-gated profile before any public API is considered.

Candidate Feathers:

- `CPUEventLoopPolicy`: compare current adaptive non-blocking wait, immediate blocking wait, and runtime-visible syscall mode under the same route harness.
- `CPUWritePolicy`: compare current contiguous response build/write with a Linux `writev` experiment for JSON/body split responses.
- `CPUConnStatePolicy`: measure connection-local response/context reuse as a profile-level change, not a one-off scratch variable.
- `CPURouteExecutor`: explore precompiled exact-route executor metadata for single-handler routes while still running the normal handler function and panic/error handling.

The profile must remain internal until it wins route evidence. Possible internal switch:

```text
KRUDA_WING_EXPERIMENT=cpu-profile-v1
```

This should enable a bundle of measured Feathers only after each component has individual evidence. It must not silently change default Wing behavior.

## Rejected Shortcuts

Do not pursue these as Phase 4 wins:

- Static bypass for fair handler claims. `WingStaticText` and `WingStaticJSON` are valid public hot-path Feathers, but they bypass handler/middleware/lifecycle and are separate evidence.
- DB or fortunes benchmark wins. Those are driver/pool/application comparisons, not CPU-bound framework-core evidence.
- Public `WingCPU*` APIs before an internal profile wins.
- Removing safe-copy semantics for path, query, body, content type, cookies, or retained header values.
- Weakening header count/size limits, CRLF rejection, duplicate `Content-Length` rejection, or `Transfer-Encoding` plus `Content-Length` rejection.
- Benchmark-only tuning that increases p99 or error rate.

## Prototype Gate

Every Phase 4 candidate must use this sequence:

1. Implement behind an internal env gate.
2. Run focused correctness tests for parser safety, response ordering, pipelining, short writes, middleware/lifecycle behavior, panic recovery, and route fast paths.
3. Run microbenchmarks with `-benchmem -count=5`.
4. Run tiger paired route diagnostics for `plaintext-handler`, `json-static`, and `json-serialize`.
5. Keep only candidates that clear route evidence. Remove and document rejected candidates immediately.

Final evidence must use at least five measured rounds and both benchmark profiles:

```bash
wrk -t4 -c128 -d15s --latency http://127.0.0.1:<port>/<route>
wrk -t4 -c256 -d15s --latency http://127.0.0.1:<port>/<route>
```

Short three-round diagnostics are acceptable only for early rejection.

## First Phase 4 Candidate

The first candidate was `CPUWritePolicy`, because the current profile shows the route is syscall/write dominated and because the current handler path already contains:

- Exact-route Feather cache.
- Single-handler Wing fast path.
- Plaintext and JSON response fast paths.
- Same-connection speculative draining after successful writes.
- SO_REUSEPORT workers and Linux edge-triggered epoll.

The narrow experiment was:

- Keep handlers, middleware, lifecycle hooks, panic recovery, path safety, and request parser defenses unchanged.
- Add an internal Linux/Unix `writev` response writer path for `responseJSON` first.
- Use it only when the response has exactly the fast JSON shape: status, Date, JSON content type, content length, and body.
- Fall back to the current contiguous build path for custom headers, cookies, security headers, sendfile, partial writes, or non-Linux platforms.
- Reject it if route evidence does not beat the current contiguous path.

Expected risk:

- `writev` may cost more than the body copy for tiny payloads.
- Partial writes need careful fallback so response ordering and fd ownership stay correct.
- It may help larger JSON but fail the tiny benchmark body; if so, it is not a Phase 4 fair-handler win.

Result: rejected. `KRUDA_WING_EXPERIMENT=writev-json-v1` passed focused local and Linux response-ordering tests, and median p99 improved on both JSON routes, but tiger median RPS regressed versus clean `origin/main`: `plaintext-handler` -0.22%, `json-static` -0.25%, and `json-serialize` -0.72%. For the tiny CPU-bound benchmark bodies, `writev` costs more than the body copy it avoids.

## Second Phase 4 Candidate

The second candidate was `CPURouteExecutor`, not another syscall wrapper. The target was precompiled exact-route executor metadata for the common single-handler Wing route while preserving the handler call and panic/error behavior.

Result: rejected. Caching the `wingFastSingleHandler`, `wingSingleHandler`, and `wingRouteHandler` interfaces on each worker preserved semantics and passed focused tests, but tiger median RPS did not clear the gate versus clean `origin/main`: `plaintext-handler` -0.24%, `json-static` +0.95%, and `json-serialize` -0.17%. The improvement was below threshold and p99 regressed on plaintext and static JSON.

## Third Phase 4 Candidate

The third candidate was `CPUConnStatePolicy`: a connection-owned `Ctx` slot for inline single-handler Wing routes.

Result: rejected. `KRUDA_WING_EXPERIMENT=conn-ctx-v1` preserved cleanup, safe-copy behavior, lifecycle fallback, panic recovery, and response ordering in focused tests, but tiger median RPS versus clean `origin/main` did not clear the gate: `plaintext-handler` -0.40%, `json-static` -2.66%, and `json-serialize` +0.35%. Median p99 improved on both JSON routes, but static JSON throughput regressed enough to reject the runtime change.

## Fourth Phase 4 Candidate

The fourth candidate was `CPUWriteScheduling`: queue inline response writes until the current event batch has been processed, then flush queued writes while preserving per-connection response ordering.

Result: rejected. `KRUDA_WING_EXPERIMENT=batch-send-v1` passed focused local and Linux tests, but tiger median RPS versus clean `origin/main` did not clear the gate: `plaintext-handler` -1.38%, `json-static` +0.30%, and `json-serialize` +0.25%. Median p99 improved on JSON routes, but the plaintext throughput regression and sub-1% JSON gains were not enough to justify changing the current direct-send policy.

## Next Candidate

The next step should be profiling-led rather than another blind runtime patch. Capture a fresh `perf record` or Go CPU profile for current `origin/main` on tiger under the same route mix, then pick the next candidate only if the profile shows a non-syscall user-space cost center above roughly 3-5% flat CPU. If the profile still shows syscall/kernel cost dominating, Phase 4 should stop at evidence and avoid shipping runtime changes.

Result: the tiger Go CPU profiles in `bench/reproducible/results/profile-20260527T121730Z/` confirmed syscall dominance. `internal/runtime/syscall/linux.Syscall6` was 83.27% flat for `plaintext-handler`, 86.96% flat for `json-static`, and 82.51% flat for `json-serialize`. `json-serialize` has a visible but route-specific JSON pocket (`Ctx.JSON` 4.14% cumulative, `encoding/json.Marshal` 3.81% cumulative). This is useful if the goal narrows to JSON serialization, but it does not explain a broad fair-handler win across plaintext and static JSON.

## PR Strategy

Phase 4 should be a new PR only after an internal candidate survives the gate. If no runtime candidate survives, the correct artifact is a docs/evidence PR that records the rejected path and the next hypothesis.

Public docs should not claim an Actix win from Phase 4 until the balanced public claim gate is met.
