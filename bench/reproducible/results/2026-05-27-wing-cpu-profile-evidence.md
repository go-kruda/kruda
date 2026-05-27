# Wing CPU Profile Evidence

Date: 2026-05-27
Host: tiger dev server
Source commit: `origin/main` at `f9dced8`
Evidence directory: `profile-20260527T121730Z/`

## Scope

This is Kruda-only diagnostic profiling evidence for selecting the next Wing CPU-bound candidate. It is not cross-runtime benchmark claim evidence because Kruda was built with `bench_pprof` and sampled through Go pprof while load was running.

Routes:

- `/plaintext-handler`
- `/json-static`
- `/json-serialize`

Run shape:

- `GOMAXPROCS=8`
- `KRUDA_WORKERS=4`
- `KRUDA_READ_BUF_SIZE=4096`
- `wrk --latency -t4 -c256`
- 3s warmup per route
- 15s CPU profile per route

## Top Findings

The profile remains syscall/kernel dominated.

| Route | Top flat cost | Notable user-space costs |
|-------|---------------|--------------------------|
| `/plaintext-handler` | `internal/runtime/syscall/linux.Syscall6` at 83.27% flat | `parseHTTPRequestInternal` 3.02% cum, `appendPlaintextTo` 0.86% cum, `serveWingSingleHandler` 1.22% cum |
| `/json-static` | `internal/runtime/syscall/linux.Syscall6` at 86.96% flat | `parseHTTPRequestInternal` 3.08% cum, `appendJSONTo` 0.70% cum, `serveWingSingleHandler` 1.30% cum |
| `/json-serialize` | `internal/runtime/syscall/linux.Syscall6` at 82.51% flat | `Ctx.JSON` 4.14% cum, `encoding/json.Marshal` 3.81% cum, `parseHTTPRequestInternal` 2.92% cum |

## Decision

Do not start another blind runtime patch from this profile. The previous rejected candidates already targeted parser, response building, context reuse, route executor metadata, write policy, and write scheduling without clearing route evidence.

The only visible non-syscall pocket above roughly 3% is JSON serialization on `/json-serialize`, but improving that route cannot move `/plaintext-handler` or `/json-static` enough for a broad fair-handler win. A future candidate should be selected only if it changes the I/O architecture or if the goal narrows to JSON serialization evidence explicitly.

## Files

- `profile-20260527T121730Z/environment.txt`
- `profile-20260527T121730Z/raw/*.txt`
- `profile-20260527T121730Z/reports/*-top.txt`
- `profile-20260527T121730Z/profiles/*.pb.gz`
