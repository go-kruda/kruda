# Non-Pipelined Syscall Ratio Evidence

Date: 2026-06-07
Host: tiger Linux dev server (`tiger-linux`)
Commit: `7d30d7b` (`bench: add non-pipelined syscall ratios`)
Scope: Diagnostic harness output only. This change does not alter Kruda
runtime behavior.

## Local Verification

- `bash -n bench/reproducible/syscall-profile.sh`
- `git diff --check`
- Extracted the embedded `summarize` Python block and ran it against
  `bench/reproducible/results/phase5-default-syscall-sudo-20260528T025554Z`.
  The generated CSV and Markdown included request counts, requests-per-syscall
  ratios, and per-1k-request syscall ratios.

## Command

```bash
cd /home/tiger/kruda-nonpipelined-syscall-ratios-20260607T133743Z/bench/reproducible
export PATH="$HOME/.cargo/bin:/usr/local/go/bin:$PATH"
GOTOOLCHAIN=go1.25.10 \
RESULT_DIR=results/nonpipelined-syscall-ratio-json-serialize-20260607T133830Z \
PROFILE_DURATION=8s \
WARMUP_DURATION=2s \
WRK_CONNECTIONS=256 \
PROFILE_SUDO=1 \
PERF_EVENTS=task-clock,context-switches,cpu-migrations,page-faults \
KRUDA_PORT=3261 \
ACTIX_PORT=3263 \
./syscall-profile.sh json-serialize
```

## Environment

- Result directory:
  `/home/tiger/kruda-nonpipelined-syscall-ratios-20260607T133743Z/bench/reproducible/results/nonpipelined-syscall-ratio-json-serialize-20260607T133830Z`
- Git commit: `7d30d7b`
- Git tracked dirty: `0`
- CPU: `13th Gen Intel(R) Core(TM) i5-13500`, 8 logical CPUs
- OS/kernel: Linux `6.8.0-117-generic`
- Go: `go1.25.10 linux/amd64`
- Rust: `rustc 1.93.1`
- Cargo: `cargo 1.93.1`
- wrk: `debian/4.1.0-4build2 [epoll]`
- perf: `6.8.12`
- strace: `6.8`
- `profile_duration=8s`
- `warmup_duration=2s`
- `wrk_threads=4`
- `wrk_connections=256`
- `profile_sudo=1`
- `perf_event_paranoid=4`
- `ptrace_scope=1`
- `gomaxprocs=8`
- `kruda_workers=4`
- `actix_workers=default`
- `kruda_read_buf_size=4096`
- `kruda_go_tags=kruda_stdjson`
- `routes=json-serialize`

## Summary

Perf rows are the comparable wrk pass. Strace rows are intrusive syscall-count
diagnostics; use their syscall counts and ratios, not their RPS or latency, for
I/O architecture conclusions.

| Framework | Route | Kind | Requests | RPS | p99 | Socket errors | Non-2xx | read/recv | write/send | req/read | req/write | epoll wait | epoll wait/1k req | epoll_ctl | epoll_ctl/1k req | futex | futex/1k req | context switches |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| actix | json-serialize | perf | 6009861 | 741883.78 | 3.13ms | 0 | 0 |  |  |  |  |  |  |  |  |  |  | 673640 |
| actix | json-serialize | strace | 722415 | 90106.61 | 3.98ms | 0 | 0 | 722776 | 722948 | 1.00 | 1.00 | 22877 | 31.67 | 362 | 0.50 |  |  |  |
| kruda | json-serialize | perf | 7016971 | 866299.88 | 698.00us | 0 | 0 |  |  |  |  |  |  |  |  |  |  | 329440 |
| kruda | json-serialize | strace | 548190 | 68404.26 | 1.29s | 13 | 0 | 552165 | 548441 | 0.99 | 1.00 | 683 | 1.25 | 771 | 1.41 | 12833 | 23.41 |  |

Status files:

- `actix-json-serialize-perf-status.txt`: `wrk_status=0`, `perf_status=0`
- `actix-json-serialize-strace-status.txt`: `wrk_status=0`, `strace_status=130`
- `kruda-json-serialize-perf-status.txt`: `wrk_status=0`, `perf_status=0`
- `kruda-json-serialize-strace-status.txt`: `wrk_status=0`, `strace_status=130`

`strace_status=130` is expected because the harness stops strace with SIGINT
after the measured wrk run.

## Interpretation

The updated `syscall-profile.sh` emits request counts and syscall-shape ratios
for the non-pipelined diagnostic harness. This makes future I/O profile
candidates comparable without manual ratio math.

The comparable perf pass stayed healthy on this smoke route: Kruda completed
`json-serialize` at 866,299.88 RPS with 698.00us p99 and zero socket errors or
non-2xx responses; Actix completed the same pass at 741,883.78 RPS with 3.13ms
p99 and zero socket errors or non-2xx responses.

The intrusive strace pass shows both frameworks near one request per
write/send syscall on the non-pipelined `json-serialize` route. Kruda showed
far fewer epoll waits per 1k requests in this diagnostic row, while the strace
pass itself introduced 13 Kruda socket errors; that straced row should not be
used for throughput or tail-latency claims.
