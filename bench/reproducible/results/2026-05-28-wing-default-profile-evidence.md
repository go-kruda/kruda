# Wing Default Profiling Evidence

This evidence records a default-runtime profiling pass after the rejected CPU affinity and epoll idle-spin prototypes. It is evidence-only and does not change runtime behavior.

## Scope

- Host: tiger Linux dev server
- Commit: `144e1f6` (`bench: record rejected Wing epoll idle spin prototype`)
- CPU: 13th Gen Intel Core i5-13500, 8 vCPU, KVM
- OS: Linux `6.8.0-111-generic`
- Go: `go1.25.10 linux/amd64`
- wrk: `debian/4.1.0-4build2 [epoll]`
- Routes: `plaintext-handler`, `json-static`, `json-serialize`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- pprof command profile: `-t4 -c256`, 3 second warmup, 15 second CPU profile
- syscall command profile: `-t4 -c256`, 5 second warmup, 10 second measured pass

Raw evidence:

- `phase5-default-profile-20260528T024859Z/`
- `phase5-default-syscall-sudo-20260528T025554Z/`

## pprof Summary

| Route | `Syscall6` flat | `directSend` cumulative | `handleRecv` cumulative | Parser cumulative | Route/JSON cumulative | `epollWait` cumulative |
|---|---:|---:|---:|---:|---:|---:|
| `plaintext-handler` | 85.01% | 71.11% | 92.23% | 2.89% | 1.57% serve route | 3.18% |
| `json-static` | 84.66% | 70.16% | 90.63% | 2.44% | 0.68% JSON append | 3.53% |
| `json-serialize` | 81.75% | 67.72% | 91.63% | 2.71% | 3.84% `encoding/json.Marshal` | 3.86% |

The profiles are dominated by Linux socket syscalls. Parser, router, JSON serialization, and epoll waiting are not large enough in this workload to explain a further broad 20% fair-handler improvement by themselves.

## Comparable wrk Pass

The syscall harness runs a normal `perf stat` wrk pass before the intrusive `strace` pass. These rows are the comparable throughput and latency measurements.

| Route | Kruda RPS | Kruda p99 | Actix RPS | Actix p99 | RPS delta | Socket errors | Non-2xx |
|---|---:|---:|---:|---:|---:|---:|---:|
| `plaintext-handler` | 851279.50 | 0.87ms | 730025.29 | 3.27ms | +16.61% | 0 | 0 |
| `json-static` | 837252.19 | 1.10ms | 739090.25 | 3.55ms | +13.28% | 0 | 0 |
| `json-serialize` | 813073.61 | 1.05ms | 723721.51 | 3.17ms | +12.35% | 0 | 0 |

These are short profiling passes, not the five-round publication benchmark. They are useful for direction, not for public benchmark wording.

## Syscall Diagnostics

`strace` is intrusive and distorts RPS and latency. Use these rows for syscall shape only.

| Framework | Route | read/recv | write/send | epoll wait | epoll_ctl | futex |
|---|---|---:|---:|---:|---:|---:|
| Kruda | `plaintext-handler` | 695908 | 692489 | 700 | 1285 | 15209 |
| Kruda | `json-static` | 689762 | 686638 | 444 | 1276 | 14141 |
| Kruda | `json-serialize` | 686172 | 682532 | 473 | 1262 | 15519 |
| Actix | `plaintext-handler` | 895616 | 895630 | 28171 | 514 | |
| Actix | `json-static` | 892224 | 892259 | 28079 | 507 | |
| Actix | `json-serialize` | 894546 | 894583 | 29300 | 509 | |

The Kruda default path is already near one receive syscall and one send syscall per request in this workload. Epoll wait count is low for Kruda, which matches the rejected epoll idle-spin result.

## Decision

No runtime change should be taken from this evidence alone. The next candidate needs to target request/response syscall shape, connection scheduling, or response emission mechanics with a narrow prototype and a rollback path. Further parser-only, JSON-only, epoll-wait-only, or CPU-affinity work is unlikely to deliver the requested broad fair-handler win without new evidence.

Hardware cycle and instruction counters were unavailable on this kernel even with sudo (`perf_event_paranoid=4`), but `task-clock`, context switches, wrk latency, wrk errors, and syscall counts were recorded.
