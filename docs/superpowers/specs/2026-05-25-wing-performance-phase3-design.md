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

## Non-Goals

- No release, tag, or version bump.
- No default static bypass behavior.
- No DB, fortunes, TLS, HTTP/2, WebSocket, or production-network benchmark claims.
- No GitHub Actions cross-runtime benchmark in this phase.
