# PGO Candidate Rejection Evidence

Date: 2026-06-07 UTC
Host: tiger Linux dev server (`tiger-linux`)
Base commit: `eaf46fc` (`bench: reject epoll conn pointer table candidate`)
Scope: Go PGO/toolchain candidate discovery for the default fair CPU-bound
handler routes. No runtime or benchmark default change is accepted from this
pass.

## Candidate

The candidate tested whether a merged CPU profile from the three default
fair-handler routes could make the Kruda benchmark binary clear the broad CPU
route gate without changing framework behavior.

The candidate did not change source code. It built the Kruda benchmark app with
Go PGO through:

```bash
GOFLAGS="-pgo=/home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible/results/pgo-merged-20260607T161842Z/kruda-cpu-merged.pb"
```

## Profile Generation

Temporary checkout:
`/home/tiger/kruda-pgo-candidate-20260607T161715Z`

The checkout was pinned to current `origin/main` at `eaf46fc` before running
the experiment.

```bash
cd /home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/pgo-profile-20260607T161842Z \
BENCH_DURATION=8 \
WARMUP_DURATION=2 \
THREADS=4 \
CONNECTIONS=256 \
PORT=3361 \
PPROF_PORT=6161 \
./profile-kruda.sh plaintext-handler json-static json-serialize
```

The route profiles were merged with:

```bash
go tool pprof -proto ./kruda/kruda-bench \
  results/pgo-profile-20260607T161842Z/profiles/kruda-plaintext-handler-cpu.pb.gz \
  results/pgo-profile-20260607T161842Z/profiles/kruda-json-static-cpu.pb.gz \
  results/pgo-profile-20260607T161842Z/profiles/kruda-json-serialize-cpu.pb.gz \
  > results/pgo-merged-20260607T161842Z/kruda-cpu-merged.pb
```

## Benchmark Commands

Baseline:

```bash
cd /home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/pgo-baseline-20260607T161842Z \
BENCH_FRAMEWORKS=kruda \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
KRUDA_PORT=3371 \
./bench.sh plaintext-handler json-static json-serialize
```

PGO candidate:

```bash
cd /home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible
GOFLAGS="-pgo=/home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible/results/pgo-merged-20260607T161842Z/kruda-cpu-merged.pb" \
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/pgo-candidate-20260607T161842Z \
BENCH_FRAMEWORKS=kruda \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
KRUDA_PORT=3381 \
./bench.sh plaintext-handler json-static json-serialize
```

The first baseline had noisy JSON throughput rows, so a second no-PGO baseline
control was run after the PGO candidate on the same checkout:

```bash
cd /home/tiger/kruda-pgo-candidate-20260607T161715Z/bench/reproducible
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/pgo-baseline-rerun-20260607T161842Z \
BENCH_FRAMEWORKS=kruda \
BENCH_ROUNDS=3 \
BENCH_DURATION=8s \
KRUDA_PORT=3391 \
./bench.sh plaintext-handler json-static json-serialize
```

## Environment

Concrete values from the result directories:

```text
git_commit=eaf46fc
git_tracked_dirty=0
bench_enable_db=0
bench_enable_pprof=0
bench_kruda_cpu_dispatch=inline
kruda_go_tags=kruda_stdjson
gomaxprocs=8
kruda_workers=4
kruda_read_buf_size=default
bench_rounds=3
bench_duration=8s
frameworks=kruda
routes=plaintext-handler json-static json-serialize
profiles=latency:-t4 -c128 -d8s throughput:-t4 -c256 -d8s
CPU=13th Gen Intel(R) Core(TM) i5-13500, 8 online CPUs, KVM
OS=Linux 6.8.0-117-generic x86_64
Go=go1.25.10 linux/amd64
wrk=debian/4.1.0-4build2
```

All measured rows in the baseline, PGO candidate, and baseline rerun had zero
socket errors and zero non-2xx responses.

## First Baseline Noise

The first baseline showed implausibly low JSON throughput compared with the
same host's current-main evidence and with the immediate baseline rerun:

| Profile | Route | First baseline RPS rows | First baseline median p99 ms |
|---|---|---:|---:|
| throughput | `json-static` | 781,327.93 / 719,697.23 / 555,490.57 | 4.040 |
| throughput | `json-serialize` | 593,879.73 / 627,836.90 / 675,868.27 | 4.830 |

Because this baseline was noisy, it is not used for the final candidate
decision.

## Decision Comparison

PGO candidate medians compared with the second no-PGO baseline control:

| Profile | Route | Baseline RPS | PGO RPS | PGO RPS delta | Baseline p99 ms | PGO p99 ms | PGO p99 delta |
|---|---|---:|---:|---:|---:|---:|---:|
| latency | `plaintext-handler` | 837,709.84 | 781,237.67 | -6.74% | 0.756 | 1.050 | +38.89% |
| latency | `json-static` | 798,651.51 | 797,021.89 | -0.20% | 1.060 | 1.360 | +28.30% |
| latency | `json-serialize` | 822,277.25 | 807,695.89 | -1.77% | 0.697 | 0.672 | -3.59% |
| throughput | `plaintext-handler` | 820,658.02 | 799,131.43 | -2.62% | 0.777 | 0.880 | +13.26% |
| throughput | `json-static` | 819,708.44 | 820,970.15 | +0.15% | 0.744 | 1.000 | +34.41% |
| throughput | `json-serialize` | 828,553.76 | 821,630.99 | -0.84% | 0.910 | 0.813 | -10.66% |

## Decision

Reject this PGO candidate as a default fair-handler CPU-bound performance path.

The candidate did not produce a balanced RPS improvement and regressed p99 in
four of six median rows. The clearest route, `plaintext-handler`, regressed in
both latency and throughput profiles. This does not support a broad +20% Actix
win path, and it does not justify changing the default benchmark build process.

Future PGO work should stay labeled as a build-profile experiment unless it
first demonstrates a balanced same-runner win across all three fair CPU-bound
routes and includes a reproducible shipping story for the generated profile.
