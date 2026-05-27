# Phase 5 v1.2.4 Baseline Evidence

This evidence captures a fresh `v1.2.4` CPU-bound benchmark baseline on the
tiger development server before starting Phase 5 runtime experiments. It is
evidence only and does not change runtime behavior.

## Environment

- Host: tiger development server
- CPU: 13th Gen Intel(R) Core(TM) i5-13500, 8 vCPU exposed by KVM
- OS: Linux `dev-server` 6.8.0-111-generic x86_64
- Go: go1.26.3 linux/amd64
- Rust: rustc 1.93.1
- wrk: 4.1.0, epoll build
- Kruda tag: `v1.2.4`
- Go tags: `kruda_stdjson`
- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- `KRUDA_READ_BUF_SIZE=4096`

## Evidence Directories

- `phase5-v124-bench-20260527T191419Z/`: five measured rounds plus warmup for
  Kruda, Fiber, and Actix across the latency and throughput profiles.
- `phase5-v124-profile-20260527T194350Z/`: Kruda CPU profiles for the three
  fair handler-path CPU routes.
- `phase5-v124-syscall-20260527T194537Z/`: comparable perf-counter wrk passes
  plus intrusive strace syscall-count diagnostics for Kruda and Actix.

## Median Benchmark Comparison

The benchmark harness used fair handler-path routes only:

- `GET /plaintext-handler`
- `GET /json-static`
- `GET /json-serialize`

| Profile | Route | Kruda median RPS | Actix median RPS | Kruda vs Actix RPS | Kruda p99 | Actix p99 | Socket errors | Non-2xx |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| latency | plaintext-handler | 818118.48 | 702291.12 | +16.49% | 0.980 ms | 2.970 ms | 0 | 0 |
| latency | json-static | 799852.32 | 705865.87 | +13.32% | 0.782 ms | 2.850 ms | 0 | 0 |
| latency | json-serialize | 788577.68 | 700818.55 | +12.52% | 0.920 ms | 2.720 ms | 0 | 0 |
| throughput | plaintext-handler | 810134.56 | 737663.70 | +9.82% | 1.050 ms | 3.270 ms | 0 | 0 |
| throughput | json-static | 815402.62 | 729178.99 | +11.82% | 0.880 ms | 2.990 ms | 0 | 0 |
| throughput | json-serialize | 794857.94 | 717325.28 | +10.81% | 0.930 ms | 3.010 ms | 0 | 0 |

This run supports the current public claim gate for these CPU-bound fair
handler-path routes: Kruda median RPS is more than 3% higher than Actix, p99 is
not worse than Actix, and socket errors/non-2xx counts are zero.

It does not support a broad claim that Kruda is 20% faster than Actix. The
throughput profile gap is roughly +9.82% to +11.82% across the measured routes.

## Profile Findings

Kruda CPU profiles remain dominated by the Linux syscall path:

| Route | Top flat sample | Share |
|---|---|---:|
| plaintext-handler | `internal/runtime/syscall/linux.Syscall6` | 84.84% |
| json-static | `internal/runtime/syscall/linux.Syscall6` | 85.52% |
| json-serialize | `internal/runtime/syscall/linux.Syscall6` | 81.46% |

For `json-serialize`, `encoding/json.Marshal` accounts for 3.20% cumulative and
`Ctx.JSON` accounts for 3.54% cumulative. JSON-specific work may improve that
route, but it is not the likely path to a broad +20% fair-handler win.

## Syscall Diagnostics

Perf rows are the comparable wrk pass. Strace rows are intrusive and should be
used only for syscall-count diagnostics, not RPS or latency conclusions.

| Route | Kruda perf RPS | Actix perf RPS | Kruda vs Actix | Kruda p99 | Actix p99 | Kruda context switches | Actix context switches |
|---|---:|---:|---:|---:|---:|---:|---:|
| plaintext-handler | 830630.04 | 744213.92 | +11.61% | 1.41 ms | 3.36 ms | 374509 | 572216 |
| json-static | 817824.53 | 733955.91 | +11.43% | 1.53 ms | 3.27 ms | 358534 | 693335 |
| json-serialize | 794603.28 | 721038.24 | +10.20% | 1.36 ms | 3.21 ms | 207811 | 719473 |

The syscall-count diagnostics show Kruda with fewer context switches than Actix
in these runs, while CPU profiles still show most sampled time under syscall
execution. The next Phase 5 prototype should therefore prioritize the
kernel/event-loop architecture track, such as a Linux-only thread-per-core or
CPU-affinity feasibility experiment behind an explicit opt-in flag.

## Conclusion

The v1.2.4 baseline is strong enough for the existing Actix win gate on these
CPU-bound fair-handler benchmark routes, but it is not a 20% win. Phase 5 should
continue as research-backed runtime experimentation, with correctness and
security Bones unchanged by default.
