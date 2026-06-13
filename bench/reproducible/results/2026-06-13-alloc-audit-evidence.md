# Allocation / GC Audit (P2)

Date: 2026-06-13
Measured locally (allocs/op is deterministic and machine-independent;
`go test -benchmem` + `pprof -alloc_objects`, Go 1.25.10, `kruda_stdjson`).
Scope: perf-track wave **P2** — audit per-request allocations on the Wing hot
path and cut what is safely cuttable.

## Question

Lower allocs/op on the hot path reduces GC pressure (and therefore tail
latency). Where do the per-request allocations live, and which can be removed
without breaking Kruda's safe-string contract?

## Findings — hot path is already allocation-optimal

| Bench (path) | B/op | allocs/op | reading |
|---|---:|---:|---|
| `ContextPoolAcquireRelease` | 0 | **0** | Ctx pool — zero alloc |
| `WingStringLaneFortuneSize` | 0 | **0** | string fast lane — zero alloc |
| `CPUHandlerPlaintextPreset` | 8 | **1** | plaintext via lane — 1 (the payload) |
| `CPUHandlerJSONSerializePreset` | 160 | **1** | JSON via pooled buffer — 1 (the output) |
| `CPUHandlerInline` (generic/header-write) | 264 | 7 | see breakdown |
| `CPUFullCycle` (generic) | 264 | 7 | see breakdown |

The **benchmark hot path** (plaintext/JSON through the string lane, what the
TFB-style routes use) is already 0–1 alloc/op — the single alloc is the response
payload itself. `Ctx` and the string lane are zero-alloc. This is the v1.3.0
lane + pool work; the alloc dimension on the hot path is already won.

## The 7-alloc generic path, broken down (`pprof -alloc_objects`)

```text
50.4%  (*wingResponse).buildZeroCopy
49.5%  copyOrUnsafeString  (via parseHTTPRequest → parseHTTPRequestInternal)
```

Both are **not** safe to "fix":

1. **`copyOrUnsafeString`** is Kruda's safe-string contract. `parseHTTPRequest`
   copies the request strings so handler-held values never alias a reused read
   buffer (the explicit anti-Fiber "no context-reuse bugs" guarantee). The Fast
   parse variant skips the copy with `unsafe` and is used only where the buffer
   provably outlives the request. Forcing the unsafe path everywhere would
   reintroduce the aliasing class of bug Kruda exists to prevent — rejected.
2. **`buildZeroCopy`** allocs here only because the microbench sets
   `resp.buf = nil` before `releaseResponse`, defeating buffer pooling. In
   production `releaseResponse` returns the buffer to the pool and the next
   `acquireResponse` reuses it — no per-request alloc. Bench artifact, not a
   real hot-path alloc.

## Decision

**Document + guard; no production change.** The Wing hot path is already
allocation-optimal, and the remaining allocations are either the deliberate
safe-string contract (must not break) or pooled in production (bench artifact).
There is no safe alloc to cut on the benchmark hot path.

Added `TestStringLaneZeroAlloc` (`wing_bench_test.go`) — a
`testing.AllocsPerRun` guard asserting the string lane stays zero-alloc, so a
future change cannot silently regress the win.

Because P2 changes no production code (a test only), there is no RPS/p99
regression risk and no tiger A/B is required for this wave.

## Out of scope / future (not the benchmark hot path)

- Typed handlers (`C[T]`): `TypedHandler` 5 allocs, `TypedHandlerValidation` 6 —
  reflection + bind + validate. Real for typed-handler users but off the
  benchmark hot path; a candidate for the DX-track if typed-handler alloc DX
  becomes a goal.
- `net/http` generic `c.Text` path: 1 alloc (the `[]byte(s)` copy). The Wing
  lane already avoids it; net/http is the fallback transport, not the perf focus.
