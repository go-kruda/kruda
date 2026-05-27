# Wing BigBang Phase 5 Design

Date: 2026-05-28
Status: Draft

## Goal

Phase 5 is a research branch for finding a credible path toward a larger fair-handler CPU-bound win after the `v1.2.4` release. The working stretch target is a 20% median RPS advantage over Actix on fair handler-path CPU-bound routes, but that is not a current claim.

The first objective is not to ship a runtime change. The first objective is to identify whether any architecture-level Wing profile can move beyond the current syscall-dominated ceiling without weakening correctness, security, or default framework behavior.

## Baseline

`v1.2.4` is the baseline for Phase 5.

The current public evidence supports a balanced CPU-bound fair-handler claim for the benchmarked routes, but it does not support a broad 20% Actix win claim. The most recent Go CPU profiles on tiger showed `internal/runtime/syscall/linux.Syscall6` as the dominant flat CPU cost:

- `plaintext-handler`: 83.27% flat
- `json-static`: 86.96% flat
- `json-serialize`: 82.51% flat

The only visible route-specific user-space pocket was JSON serialization on `json-serialize`, with `Ctx.JSON` at 4.14% cumulative and `encoding/json.Marshal` at 3.81% cumulative.

That means small user-space cleanups are unlikely to produce a broad fair-handler 20% jump across plaintext, static JSON, and serialized JSON.

## Scope

Phase 5 covers fair handler-path CPU-bound routes only:

- `GET /plaintext-handler`
- `GET /json-static`
- `GET /json-serialize`

Static bypass routes remain out of scope for fair handler claims. DB, fortunes, TLS, HTTP/2, HTTP/3, file serving, SSE, and WebSocket workloads are also out of scope for this phase.

## Non-Negotiable Bones

Phase 5 must not weaken:

- HTTP parser safety, including header count and size limits.
- CRLF rejection.
- Duplicate `Content-Length` rejection.
- `Transfer-Encoding` plus `Content-Length` rejection.
- Request path, query, body, content type, cookie, and retained header safe-copy semantics.
- Middleware, lifecycle, panic recovery, error handling, cookie, CORS, and secure-header behavior on normal handler routes.
- Response ordering, pipelining behavior, fd ownership, timeout behavior, and backpressure behavior.

Any candidate that weakens one of these is not a fair-handler candidate. If it is ever useful as an opt-in bypass, it needs separate naming, docs, tests, and evidence.

## Success Gate

A Phase 5 runtime candidate can be kept only if it clears all of these:

- At least 5% median RPS improvement over `v1.2.4` on all three fair CPU-bound routes, or at least 10% on one route with no median RPS regression on the others.
- p99 latency no worse than 10% above `v1.2.4`.
- Zero socket errors.
- Zero non-2xx responses.
- No allocation or B/op increase on CPU-bound hot-path microbenchmarks unless isolated to an explicit opt-in path.
- No default behavior change.

The 20% Actix stretch target requires a separate public claim review. It cannot be inferred from a single microbenchmark, a single route, or an opt-in bypass.

## Candidate Tracks

### Track A: Kernel And Event-Loop Architecture

This is the only track likely to move all three fair-handler routes if syscall cost remains dominant.

Candidates:

- Thread-per-core Wing profile with fixed worker ownership and less cross-worker scheduler movement.
- CPU affinity experiment for Wing workers and benchmark client isolation.
- Accept and connection distribution policy review under `SO_REUSEPORT`.
- Event batch sizing and wake policy review using kernel-level `perf` evidence, not only Go pprof.

Risks:

- Higher operational complexity.
- Lower portability.
- Worse p99 if affinity or worker ownership interacts poorly with Go scheduling.
- Bigger code surface around fd ownership and shutdown.

Gate:

- Must start as Linux-only and env-gated.
- Must include p99 and error evidence before any API naming.

### Track B: Go Toolchain And PGO Profile

This track may provide low-risk gains without runtime code changes, but it is not a framework default behavior change.

Candidates:

- Generate a benchmark PGO profile from fair CPU-bound routes.
- Compare `go build` with and without PGO on the same tiger harness.
- Keep PGO evidence separate from default binary claims unless a reproducible shipping story exists.

Risks:

- Workload-specific profile may overfit.
- PGO may help JSON more than plaintext/static JSON.

Gate:

- Must report all three routes and both latency and throughput profiles.
- Must not become the public benchmark default unless the build process is documented and reproducible.

### Track C: JSON-Specific Route Work

This track is useful but cannot support a broad fair-handler 20% claim by itself.

Candidates:

- Compare stdlib JSON, Sonic, and any current build-tag behavior on `json-serialize`.
- Inspect whether handler-level JSON response writing still pays avoidable interface or marshal overhead.
- Explore typed/static encoder hints only if API semantics stay normal.

Risks:

- Does not improve plaintext or static JSON.
- Can create misleading benchmark messaging if mixed into broad claims.

Gate:

- Must be labeled JSON-specific.
- Must not be used to claim a broad Actix win.

### Track D: Harness And Evidence Improvements

This track improves decision quality before touching runtime behavior.

Candidates:

- Add a repeatable tiger command profile for `perf stat`, `perf record`, and Go pprof on `v1.2.4`.
- Add a compact comparison summary for `v1.2.4` versus candidate branches.
- Record CPU model, kernel, Go version, GOMAXPROCS, KRUDA_WORKERS, read buffer size, route, profile, and timestamp.

Risks:

- Does not improve performance directly.

Gate:

- Must reduce false positives and make rejections cheaper.

## First Phase 5 Move

The first move should be Track D plus a narrow Track A feasibility probe:

1. Capture a fresh `v1.2.4` tiger baseline with `bench/reproducible/bench.sh`, `profile-kruda.sh`, and `syscall-profile.sh`.
2. Add a small evidence note summarizing whether kernel/syscall dominance remains true after the release tag.
3. Only if the evidence still points at kernel/event-loop cost, prototype a Linux-only env-gated thread-per-core or affinity candidate.

This keeps Phase 5 honest: measure the released baseline first, then prototype one architecture change at a time.

## PR Strategy

Phase 5 should not be one large runtime PR.

Expected PR sequence:

1. Design and baseline evidence PR.
2. One prototype PR per env-gated candidate.
3. Rejection notes or keep notes based on route evidence.
4. Public docs update only after a candidate clears the gate.

Commit messages, PR title/body, docs, and benchmark evidence must remain English-only and contain no AI attribution.

## Stop Rule

Stop Phase 5 runtime work if the fresh `v1.2.4` tiger profile still shows syscall/kernel cost above 80% flat CPU and no Track A candidate can improve all three routes without p99 or error regressions.

In that case, the credible next product direction is not more fair-handler micro-optimization. It is either JSON-specific optimization, workload-specific Wing profiles, or broader framework features where Kruda can win real application credibility.
