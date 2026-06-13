# Runtime Footprint (P3)

Date: 2026-06-13
Host: tiger Linux dev server (`tiger-linux`, the deploy target), Go 1.25.10;
cross-checked on local macOS. Measured with `bench/reproducible/footprint.sh`.
Scope: perf-track wave **P3** — measure-first on startup time, binary size, and
RSS; optimize only where real headroom exists.

## Question

Beyond throughput, is there framework-side headroom in startup time, binary
size, or memory footprint?

## Results

| Metric | tiger (Linux, deploy target) | macOS (cross-check) |
|---|---:|---:|
| startup → first 200 | **5 ms** | 535 ms |
| idle RSS | **13.79 MB** | 19.33 MB |
| binary, `kruda_stdjson` | 21.87 MB | 21.39 MB |
| binary, default (Sonic) | 26.64 MB | 22.56 MB |

The macOS startup (535 ms) is process-spawn / dyld overhead on the dev laptop,
**not** the framework — on the Linux deploy target the app is serving in **5 ms**
(route tree is AOT-compiled at `New`/`Compile`, no eager DB connect when
`BENCH_ENABLE_DB=0`).

## Findings

- **Startup: 5 ms on Linux** — already excellent; nothing to optimize.
- **Idle RSS: 13.79 MB** — Go-runtime baseline plus the Wing/Ctx pools; lean and
  Go-runtime-dominated. No framework-specific allocation stands out.
- **Binary size: 21.9 MB (stdjson) / 26.6 MB (Sonic)** — dominated by the Go
  runtime and the app's deps (the bench app links pgx). Sonic adds ~4.8 MB on
  Linux (its amd64 assembly/JIT). The size knob already exists and is documented:
  the `kruda_stdjson` build drops Sonic for ~4.8 MB smaller binaries.

## Decision

**Document; no optimization.** Footprint on the deploy target is already lean
(5 ms startup, 13.8 MB idle RSS) and the remaining size is Go-runtime + app-dep
dominated. The two real knobs already exist and are documented: `kruda_stdjson`
for binary size, and `GOGC`/`GOMEMLIMIT` for RSS under load (see
`docs/guide/performance.md`). `footprint.sh` is added as the regression-checking
tool. No production code changes, so no RPS/p99 regression risk and no tiger A/B
is required for this wave.
