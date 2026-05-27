# Wing Performance Phase 3 Design

Date: 2026-05-25
Status: Draft for `perf/wing-linux-profile-guided-phase3`

## Goal

Build the next Wing performance step from Linux profiling evidence rather than speculative hot-path edits. Phase 3 targets fair handler-path CPU-bound routes and keeps the existing default framework contract intact.

The working target is:

- At least 5% median RPS improvement over current `main` on one or more fair handler routes.
- p99 latency no worse than 10% above current `main` on the same route/profile.
- Zero socket errors and zero non-2xx responses.
- No allocation or B/op increase on CPU-bound hot-path microbenchmarks.

If a change improves only a microbenchmark but does not hold up on tiger route evidence, it should not ship in this phase.

## Scope

Phase 3 covers Wing handler-path performance for:

- `GET /plaintext-handler`
- `GET /json-static`
- `GET /json-serialize`

The routes must still execute normal handlers. Static bypass routes such as `WingStaticText` and `WingStaticJSON` remain separate evidence and must not be used for fair handler-path claims.

## Performance Wing Direction

The preferred direction is a specialized Wing profile or internal Wing fast path, not a behavior change to the default framework contract.

Valid work includes:

- Linux profile-guided response writing improvements.
- Reducing response copy/build overhead in inline Wing routes.
- Faster safe parsing for common headers while preserving request-smuggling defenses.
- Route/Feather lookup specialization for exact hot routes.
- Optional Linux-only tuning when it is clearly scoped, benchmark-backed, and not enabled silently for all users.

Invalid work includes:

- Skipping middleware, lifecycle hooks, panic recovery, error handling, or security header behavior on fair handler routes.
- Weakening `Content-Length`, `Transfer-Encoding`, CRLF, header count, header size, body limit, timeout, or path traversal safety.
- Adding public benchmark wording that claims an Actix win before the balanced gate passes.
- Shipping tiger-only tuning that regresses macOS, Windows compatibility, or normal fallback transports.

## Measurement Loop

Every candidate change should follow this loop:

1. Profile current `main` on tiger for the target route and profile.
2. Identify the top cost center: syscall, parser, route lookup, context lifecycle, handler dispatch, response build, or response write.
3. Make one focused change.
4. Run local microbenchmarks with `-benchmem -count=5`.
5. Run tiger paired route benchmarks with `wrk --latency`.
6. Keep only changes that pass both microbench and route evidence gates.

The default tiger profiles are:

```bash
wrk -t4 -c128 -d15s --latency http://127.0.0.1:13000/<route>
wrk -t4 -c256 -d15s --latency http://127.0.0.1:13000/<route>
```

Use one warmup plus at least five measured rounds for final evidence. Shorter three-round route diagnostics are acceptable only for rejecting weak candidates early.

## Evidence Required In The PR

The Phase 3 PR should include:

- Local test and race commands.
- Windows cross-target vet if Wing concrete types are moved or referenced.
- Microbench `benchstat` for current `main` versus branch.
- Tiger route summary for affected routes and profiles.
- A rejected-candidate note for changes that looked plausible but failed route evidence.

The PR body must keep claims conservative. If the route evidence is mixed or below the public Actix gate, the PR should describe the result as a targeted Wing handler-path improvement, not a public benchmark win.

## Initial Profiling Notes

Tiger profiling of `main` after PR #64 showed that the fair handler routes are dominated by socket syscalls rather than parser or JSON work:

- `plaintext-handler`: `internal/runtime/syscall/linux.Syscall6` was about 85% flat CPU.
- `json-static`: `internal/runtime/syscall/linux.Syscall6` was about 86% flat CPU.
- `json-serialize`: `internal/runtime/syscall/linux.Syscall6` was about 80% flat CPU, with JSON serialization visible but smaller than I/O.

The first rejected candidate replaced the per-request `time.Now().UnixNano()` idle-clock update in `tryParse` with the cached event-loop timestamp. It was correct enough for targeted tests, but tiger route diagnostics did not improve median throughput and showed regressions on plaintext and static JSON, so the runtime change was removed.

The second rejected candidate gave each Wing worker an inline `wingResponse` scratch value to avoid `sync.Pool` acquire/release on `Dispatch=Inline`. The first version regressed local response microbenchmarks by splitting the pooled response reset path into helpers. A scoped version restored the pooled path and kept allocation counts flat, but tiger throughput diagnostics still regressed versus `main`: `plaintext-handler` -1.28%, `json-static` -1.17%, and `json-serialize` -0.28% median RPS with zero socket errors and zero non-2xx responses. The runtime change was removed because it did not clear the route evidence gate.

The third rejected candidate disabled the Linux post-send speculative read and relied on edge-triggered epoll readiness after direct writes. It passed targeted Linux Wing tests and produced zero socket errors/non-2xx in tiger diagnostics, but the result was mixed and below target: `plaintext-handler` -1.52%, `json-static` +2.94%, and `json-serialize` +1.23% median RPS. The runtime change was removed because it was not a broad enough improvement and did not clear the +5% route gate.

The fourth rejected candidate added an opt-in parser profile that validated unknown headers but skipped retaining them for `Request.Header` lookup. The parser microbenchmark improved for requests with extra headers (`CPUParseGETExtraHeaders` 175.6ns, 56 B/op, 4 allocs/op versus skip-extra 147.1ns, 40 B/op, 2 allocs/op), but tiger route diagnostics did not improve throughput: `plaintext-handler` -1.68%, `json-static` +0.06%, and `json-serialize` -1.03% median RPS with zero socket errors and zero non-2xx responses. The runtime/API change was removed because it improved a microbenchmark but not route evidence.

The fifth rejected diagnostic changed the benchmark app to use the existing `Spear`/`Takeover` dispatch mode for CPU-bound routes. This kept handlers active but moved each connection to a blocking goroutine. It was a poor fit for the CPU-bound fair-handler workload: tiger diagnostics regressed by about 62% median RPS on all three routes (`plaintext-handler` -61.61%, `json-static` -62.23%, `json-serialize` -62.08%). The benchmark change was removed; `Takeover` remains positioned for routes where handler I/O latency dominates.

Worker-scaling diagnostics on current `main` also rejected increasing the benchmark worker count from 4 to 8 for the throughput profile. With `wrk -t4 -c256 -d15s --latency`, 8 workers regressed median RPS on all target routes: `plaintext-handler` -2.80%, `json-static` -2.55%, and `json-serialize` -2.37%. The existing benchmark alignment of `KRUDA_WORKERS=4` with four load-generator threads remains the better measured default for this tiger profile.

GOMAXPROCS scaling diagnostics also rejected lowering the benchmark process count from 8 to 4 while keeping `KRUDA_WORKERS=4`. With the same throughput profile, `GOMAXPROCS=4` was mixed versus 8: `plaintext-handler` -0.70%, `json-static` -1.83%, and `json-serialize` +0.31% median RPS. The current `GOMAXPROCS=8` benchmark setting remains reasonable for this tiger profile.

Phase 3B tracing found that same-connection draining already exists in the default direct-send path: after a full response write, Wing performs a speculative read and recursively processes the next keep-alive request until the socket returns `EAGAIN`. A bounded variant, `KRUDA_WING_EXPERIMENT=same-conn-drain-v1`, limited this continuation to four speculative same-connection reads before returning to epoll. It passed focused Linux tests but did not clear tiger route evidence: `plaintext-handler` -0.43%, `json-static` -0.81%, and `json-serialize` +0.50% median RPS versus clean `origin/main`. The runtime experiment was removed because it did not produce a defensible CPU-bound handler-path win.

Phase 3B also rejected an event-loop idle policy variant, `KRUDA_WING_EXPERIMENT=busy-poll-v1`, that skipped the short-idle `runtime.Gosched()` path before the worker returned to blocking epoll. It passed focused Linux tests but did not clear tiger route evidence versus clean `origin/main`: `plaintext-handler` -2.44% median RPS with median p99 rising from 0.97ms to 1.08ms, `json-static` +0.92% median RPS, and `json-serialize` +0.90% median RPS. The small JSON gains did not justify the plaintext regression or higher idle CPU risk, so the runtime experiment was removed.

Phase 3B also rejected `KRUDA_WING_EXPERIMENT=short-read-skip-spec-v1`, which skipped the post-response speculative read when the previous socket read was shorter than the remaining read buffer capacity. The hypothesis was that a short read had already drained the socket on Linux, making the usual speculative read likely to return `EAGAIN`. It passed focused Linux response-ordering and partial-write tests but did not clear tiger route evidence versus clean `origin/main`: `plaintext-handler` -1.80% median RPS with median p99 rising from 0.98ms to 1.45ms, `json-static` -2.21% median RPS, and `json-serialize` +0.05% median RPS. The p99 improvements on some JSON rounds did not justify the throughput regressions, so the runtime experiment was removed.

Phase 3B also rejected removing the repeated exact-path equality check from `finalizeRequestPath` for exact Feather routes. The invariant was defensible because a non-empty internal `Feather.path` is assigned only after exact route lookup, but tiger route evidence did not clear the gate versus clean `origin/main`: `plaintext-handler` -0.53% median RPS with median p99 rising from 0.95ms to 1.05ms, `json-static` +2.07% median RPS, and `json-serialize` -2.45% median RPS. The improvement was limited to one route and stayed below the 5% candidate threshold, so the runtime change was removed.

Phase 3B also rejected `KRUDA_WING_EXPERIMENT=block-wait-v1`, which forced Linux `epoll_pwait` to block immediately instead of doing the existing short non-blocking wait sequence before the worker becomes idle. The hypothesis was that this might remove empty polling syscalls in request/response keep-alive traffic. It passed focused Linux response-ordering and partial-write tests but did not clear tiger route evidence versus clean `origin/main`: `plaintext-handler` -0.19% median RPS, `json-static` -0.63% median RPS, and `json-serialize` -0.48% median RPS. Median p99 improved for plaintext but regressed on both JSON routes, and throughput stayed below the candidate threshold, so the runtime experiment was removed.

Phase 4 exploration also rejected `KRUDA_WING_EXPERIMENT=writev-json-v1`, a Linux/Unix JSON fast-response path that used `writev` to split JSON response headers and body instead of copying the small body into the contiguous send buffer. It passed focused local and Linux response-ordering tests, but tiger route evidence versus clean `origin/main` did not clear the throughput gate: `plaintext-handler` -0.22% median RPS, `json-static` -0.25% median RPS, and `json-serialize` -0.72% median RPS. Median p99 improved on both JSON routes, but the throughput regression showed that `writev` costs more than the tiny-body copy in this benchmark, so the runtime experiment was removed.

Phase 4 exploration also rejected a `CPURouteExecutor` adapter-cache candidate that stored the `wingFastSingleHandler`, `wingSingleHandler`, and `wingRouteHandler` interfaces on each worker instead of doing type assertions inside `serveRoute` on every request. It preserved semantics and passed focused local and Linux tests, but tiger route evidence versus clean `origin/main` did not clear the gate: `plaintext-handler` -0.24% median RPS, `json-static` +0.95% median RPS, and `json-serialize` -0.17% median RPS. The improvement was limited to one route, below threshold, and p99 regressed on plaintext and static JSON, so the runtime change was removed.

Phase 4 exploration also rejected `KRUDA_WING_EXPERIMENT=conn-ctx-v1`, a connection-owned `Ctx` candidate for inline single-handler Wing routes. The candidate preserved cleanup, lifecycle fallback, panic recovery, safe-copy behavior, and response ordering in focused local and Linux tests, but tiger route evidence versus clean `origin/main` stayed mixed: `plaintext-handler` -0.40% median RPS, `json-static` -2.66% median RPS, and `json-serialize` +0.35% median RPS. Median p99 improved on `json-static` and `json-serialize`, but the static JSON throughput regression and lack of a broad RPS win meant the runtime change was removed.

Phase 4 exploration also rejected `KRUDA_WING_EXPERIMENT=batch-send-v1`, a scheduling candidate that queued inline response writes until the current event batch finished before flushing per-connection writes. It preserved response ordering and passed focused local and Linux tests, but tiger route evidence versus clean `origin/main` did not clear the gate: `plaintext-handler` -1.38% median RPS, `json-static` +0.30% median RPS, and `json-serialize` +0.25% median RPS. Median p99 improved on the JSON routes, but the plaintext throughput regression and sub-1% JSON gains were not enough to justify changing the direct-send policy, so the runtime change was removed.

The follow-up tiger Go CPU profiles in `bench/reproducible/results/profile-20260527T121730Z/` confirmed that the fair handler routes remain syscall dominated: `internal/runtime/syscall/linux.Syscall6` was 83.27% flat for `plaintext-handler`, 86.96% flat for `json-static`, and 82.51% flat for `json-serialize`. The only visible non-syscall pocket above roughly 3% was JSON serialization on `json-serialize` (`Ctx.JSON` 4.14% cumulative, `encoding/json.Marshal` 3.81% cumulative), which is not broad enough to move plaintext or static JSON. The next runtime candidate should be chosen only if it changes the I/O architecture or if the goal narrows to JSON serialization evidence explicitly.

## Non-Goals

- No release, tag, or version bump.
- No default static bypass behavior.
- No DB, fortunes, TLS, HTTP/2, WebSocket, or production-network benchmark claims.
- No GitHub Actions cross-runtime benchmark in this phase.
