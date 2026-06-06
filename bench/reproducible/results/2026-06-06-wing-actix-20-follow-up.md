# Wing Actix 20 Follow-up Evidence

This note records a follow-up investigation after merging the Linux accept
re-arm fix on `main` at `c3a17ef` (`perf: skip duplicate Linux accept rearm`).
The goal was to check whether the current Wing fair handler path could support a
broad 20% Actix throughput claim on tiger without unsafe runtime changes.

This is evidence-only. No runtime candidate from this pass is accepted.

## Environment

- Host: tiger development server
- CPU: 13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs
- OS: Linux `6.8.0-117-generic` x86_64
- Go: `go1.25.10 linux/amd64`
- Rust: `rustc 1.93.1`
- Cargo: `cargo 1.93.1`
- wrk: Debian `4.1.0-4build2`
- Runtime: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`
- Routes: `plaintext-handler`, `json-static`, `json-serialize`
- Profiles:
  - latency: `wrk --latency -t4 -c128 -d8s`
  - throughput: `wrk --latency -t4 -c256 -d8s`
- Rounds: 3 measured rounds plus warmup
- Error gate: every recorded row had zero socket errors and zero non-2xx responses.

## Current Main Smoke

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/current-main-c3a17ef-actix20-smoke-20260606T112852Z/`

Median throughput-profile results:

| Route | Kruda median RPS | Actix median RPS | Kruda delta |
|---|---:|---:|---:|
| `plaintext-handler` | 818,891.92 | 726,613.10 | +12.70% |
| `json-static` | 816,751.72 | 713,123.66 | +14.53% |
| `json-serialize` | 808,543.31 | 710,940.22 | +13.73% |

Median latency-profile results:

| Route | Kruda median RPS | Actix median RPS | Kruda delta |
|---|---:|---:|---:|
| `plaintext-handler` | 819,596.15 | 711,992.34 | +15.11% |
| `json-static` | 817,918.00 | 697,201.84 | +17.31% |
| `json-serialize` | 814,870.08 | 676,518.24 | +20.45% |

Only `json-serialize` in the latency profile crossed +20%. The throughput
profile did not.

## Fresh Syscall and CPU Profile

Temporary result directories:

- Syscall profile:
  `/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/current-main-c3a17ef-syscall-actix20-20260606T114857Z/`
- Kruda pprof:
  `/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/current-main-c3a17ef-profile-actix20-20260606T115150Z/`

The syscall diagnostic still showed approximately one read/recv and one
write/send syscall per request for both Kruda and Actix. Kruda's `epoll_ctl`
count was not the current limiter after the accept re-arm fix.

Kruda pprof remained syscall-dominated:

| Route | `internal/runtime/syscall.Syscall6` flat CPU |
|---|---:|
| `plaintext-handler` | 83.54% |
| `json-static` | 84.25% |
| `json-serialize` | 84.51% |

The largest non-syscall pockets were too small to explain the missing broad
5-8%:

- `runtime.futex`: about 3.60% on plaintext/static and 1.33% on serialized JSON.
- `time.runtimeNow`: about 1.43-1.62%.
- JSON serialization path: visible only on `json-serialize`, not a broad
  plaintext/static lever.

## Candidate Checks

### Cached event timestamp candidate

A small runtime candidate reused the event-batch timestamp instead of calling
`time.Now()` again after a request parsed successfully. It passed remote Linux
tests on rerun, but benchmark evidence rejected it.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/eventnow-candidate-actix20-20260606T115751Z/`

Throughput-profile Kruda change versus current main:

| Route | Kruda change |
|---|---:|
| `plaintext-handler` | -0.49% |
| `json-static` | -0.05% |
| `json-serialize` | +0.90% |

Decision: reject. It does not move the broad Actix gap.

### Combined PGO candidate

A temporary merged PGO profile was built from the three Kruda pprof captures
and passed to the Kruda benchmark binary with `GOFLAGS=-pgo=<profile>`.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/pgo-combined-candidate-actix20-20260606T120540Z/`

Throughput-profile Kruda change versus current main:

| Route | Kruda change |
|---|---:|
| `plaintext-handler` | +0.10% |
| `json-static` | +0.21% |
| `json-serialize` | +2.53% |

Throughput-profile Kruda deltas versus Actix in the PGO run:

| Route | Kruda delta |
|---|---:|
| `plaintext-handler` | +13.22% |
| `json-static` | +12.07% |
| `json-serialize` | +14.87% |

Decision: reject as a broad 20% path. PGO may still be useful for application
builds, but it does not support the fair handler benchmark claim here.

### Read buffer 4096 candidate

The default benchmark run used the framework default read buffer (`8192`). A
fresh `KRUDA_READ_BUF_SIZE=4096` run checked the documented throughput/p99
profile on current main.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/current-main-c3a17ef-readbuf4096-actix20-20260606T121309Z/`

Throughput-profile Kruda change versus current main:

| Route | Kruda change |
|---|---:|
| `plaintext-handler` | -0.79% |
| `json-static` | +1.18% |
| `json-serialize` | +0.54% |

Throughput-profile Kruda deltas versus Actix in the 4096 run:

| Route | Kruda delta |
|---|---:|
| `plaintext-handler` | +11.28% |
| `json-static` | +12.84% |
| `json-serialize` | +12.12% |

Decision: reject as a broad 20% path on current main. Keep `4096` as a
workload/resource profile candidate, not a blanket throughput claim.

### Static response bypass candidate

A benchmark-only candidate wired the existing `WingStaticText` and
`WingStaticJSON` route options into the Kruda benchmark app behind
`BENCH_KRUDA_STATIC=1` for `/plaintext-handler` and `/json-static`.

This candidate is explicitly not a fair-handler path: the static Wing response
options bypass the handler, middleware, lifecycle hooks, cookies, CORS, and
secure-header injection on Wing transports. It was tested only as a
workload-specific static hot-path profile.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/static-profile-candidate-actix20-20260606T123704Z/`

Throughput-profile Kruda change versus current main:

| Route | Kruda change |
|---|---:|
| `plaintext-handler` | +1.84% |
| `json-static` | +1.49% |

Throughput-profile Kruda deltas versus Actix in the static-profile run:

| Route | Kruda delta |
|---|---:|
| `plaintext-handler` | +11.83% |
| `json-static` | +11.18% |

Decision: reject as a 20% path. Even the scoped static bypass profile did not
clear +20% on tiger, so it should not be added to the benchmark harness for this
claim.

### Actix worker-count methodology check

A temporary benchmark-harness candidate added `BENCH_ACTIX_WORKERS=4` so Actix
used the same worker count as the Kruda harness default (`KRUDA_WORKERS=4`) and
the wrk client thread count (`-t4`). The check was methodology-only: it tested
whether the current default comparison was hiding a +20% Kruda win.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/actix-workers4-methodology-actix20-20260606T125903Z/`

Throughput-profile comparison:

| Route | Main Kruda delta vs default Actix | Kruda delta vs Actix workers=4 | Actix workers=4 change vs default |
|---|---:|---:|---:|
| `plaintext-handler` | +12.70% | +2.99% | +10.39% |
| `json-static` | +14.53% | +4.20% | +11.81% |
| `json-serialize` | +13.73% | +8.20% | +6.74% |

Latency-profile comparison:

| Route | Main Kruda delta vs default Actix | Kruda delta vs Actix workers=4 | Actix workers=4 change vs default |
|---|---:|---:|---:|
| `plaintext-handler` | +15.11% | +4.54% | +11.79% |
| `json-static` | +17.31% | +1.94% | +13.00% |
| `json-serialize` | +20.45% | +9.38% | +13.44% |

Decision: reject as a Kruda win path. On tiger, pinning Actix to 4 workers made
Actix faster and shrank the Kruda delta. If the harness keeps this knob later,
it should be documented as a fairness/methodology control, not as evidence for
a broad +20% claim.

### Kruda workers=8 scaling check

A no-code scaling check reran the same three CPU-bound routes with
`KRUDA_WORKERS=8` on the 8-online-CPU tiger host, leaving Actix at its default
worker configuration. An earlier workers=8 run at
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/20260606T131105Z/`
was discarded for this comparison because it used Go `1.26.3`; the accepted
check below forced `GOTOOLCHAIN=go1.25.10` to match the current-main smoke.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/20260606T131911Z/`

Throughput-profile comparison:

| Route | Kruda workers=4 median RPS | Kruda workers=8 median RPS | Kruda change | Kruda delta vs Actix with workers=8 |
|---|---:|---:|---:|---:|
| `plaintext-handler` | 818,891.92 | 808,161.34 | -1.31% | +8.33% |
| `json-static` | 816,751.72 | 809,487.05 | -0.89% | +9.69% |
| `json-serialize` | 808,543.31 | 803,442.08 | -0.63% | +11.33% |

Latency-profile comparison:

| Route | Kruda workers=4 median RPS | Kruda workers=8 median RPS | Kruda change | Kruda delta vs Actix with workers=8 |
|---|---:|---:|---:|---:|
| `plaintext-handler` | 819,596.15 | 799,862.49 | -2.41% | +11.10% |
| `json-static` | 817,918.00 | 799,577.46 | -2.24% | +12.73% |
| `json-serialize` | 814,870.08 | 783,996.36 | -3.79% | +10.68% |

Decision: reject as a 20% path. On this host and workload, increasing Kruda
workers from 4 to 8 reduced Kruda median RPS and reduced the Actix delta.

## Workload-Specific DB Probe

After the CPU-bound fair-handler checks failed to support a broad +20% claim, a
read-only DB workload probe checked routes that use Wing's DB-oriented feathers
instead of the syscall-dominated plaintext/static path.

The original DB attempt failed before Actix measurement because the benchmark
harness default DSN included pgx-specific query options (`pool_max_conns`),
which Actix's `tokio-postgres` parser rejects. The harness was then updated to
use framework-specific default DB DSNs: Kruda/Fiber keep the pgx pool query
parameters, while Actix receives the same base PostgreSQL DSN without pgx-only
query options. A 1-round smoke run at
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/20260606T133834Z/`
verified that Actix starts with default framework DSNs and zero DB-row errors.

This is workload-specific evidence. It should not be used as a broad CPU-bound
handler-path claim.

Temporary result directory:
`/tmp/kruda-actix20-main.sN7hiW/bench/reproducible/results/20260606T134022Z/`

Environment notes:

- `BENCH_ENABLE_DB=1`
- `BENCH_KRUDA_DB_DISPATCH=takeover`
- `KRUDA_WORKERS=4`
- `GOTOOLCHAIN=go1.25.10`
- `database_url_common_override=0`
- `kruda_database_url_override=0`
- `actix_database_url_override=0`
- Routes: `db`, `fortunes`
- Every recorded row had zero socket errors and zero non-2xx responses.

Throughput-profile median results:

| Route | Kruda median RPS | Actix median RPS | Kruda delta | Kruda p99 ms | Actix p99 ms |
|---|---:|---:|---:|---:|---:|
| `db` | 96,919.04 | 37,246.32 | +160.21% | 9.32 | 32.42 |
| `fortunes` | 83,192.39 | 42,651.34 | +95.05% | 5.88 | 19.60 |

Latency-profile median results:

| Route | Kruda median RPS | Actix median RPS | Kruda delta | Kruda p99 ms | Actix p99 ms |
|---|---:|---:|---:|---:|---:|
| `db` | 102,426.10 | 37,082.48 | +176.23% | 6.26 | 29.89 |
| `fortunes` | 83,376.94 | 43,539.69 | +91.50% | 4.53 | 16.63 |

Decision: accept as a clean workload-specific +20% path. The benchmark harness
now reproduces the DB comparison without a manual DB URL override, but the claim
must stay scoped to read-only DB workloads.

## Decision

Do not claim a broad 20% Actix win for the current default fair handler path.

The current evidence supports:

- Kruda remains ahead of Actix on the tested CPU-bound routes.
- Kruda p99 remains materially lower than Actix in these runs.
- The missing broad 20% is not recoverable from small user-space cleanup,
  read-buffer tuning, temporary PGO, static response bypass, or Actix worker
  count alignment on this host.
- Raising Kruda workers from 4 to 8 does not recover the gap on this host.
- The remaining fair-handler ceiling is syscall-dominated at roughly 84% flat
  CPU.
- A separate read-only DB workload probe does show +20% or better versus Actix
  on `db` and `fortunes`, with much lower p99, using patched framework-specific
  DB DSN defaults. It must still be labeled as workload-specific.

The next credible performance track is not another small fair-handler patch.
It should be one of:

1. A clearly labeled workload-specific profile that changes the I/O model, with
   benchmark wording scoped to that workload.
2. JSON-specific work, explicitly scoped to serialized JSON instead of
   plaintext/static routes.
3. Product/API work where Kruda wins on typed handlers, route hints, resource
   ergonomics, and lower p99 rather than a broad +20% plaintext-style claim.
