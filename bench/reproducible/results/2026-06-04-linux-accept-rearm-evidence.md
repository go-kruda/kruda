# Linux Accept Re-arm Evidence

Date: 2026-06-04
Host: tiger dev server
Branch: `perf/wing-nonpipelined-io-profile`
Change under test: `2f0f728` (`perf: skip duplicate Linux accept rearm`)

## Scope

Linux Wing uses persistent edge-triggered epoll for the listener fd. Successful
accept events do not need a worker-level `SubmitAccept` re-arm, but the previous
epoll event adapter did not mark accepted connections with `cqeFMore`. That
caused `handleAccept` to issue duplicate `epoll_ctl(ADD)` calls against the
already-registered listener/eventfd pair after successful accepts.

This change marks successful Linux epoll accept events with `cqeFMore`, matching
the existing worker contract that skips re-arm when the backend is still armed.
It does not change request parsing, handler execution, middleware, lifecycle
hooks, response semantics, or the default route behavior.

## Validation

Local macOS validation:

```bash
GOWORK=off GOCACHE=/private/tmp/kruda-go-build-cache GOMODCACHE=/private/tmp/kruda-go-mod-cache go test -count=1 -tags kruda_stdjson ./...
GOWORK=off GOCACHE=/private/tmp/kruda-go-build-cache GOMODCACHE=/private/tmp/kruda-go-mod-cache go test -race -count=1 -tags kruda_stdjson ./...
cd bench
PATH=/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin GOTOOLCHAIN=local GOWORK=off GOCACHE=/private/tmp/kruda-go-build-cache GOMODCACHE=/private/tmp/kruda-go-mod-cache go test ./...
```

Tiger Linux validation:

```bash
CI=1 GOWORK=off GOCACHE=/tmp/kruda-go-build-cache GOMODCACHE=/tmp/kruda-go-mod-cache go test -count=1 -tags kruda_stdjson ./...
```

The non-CI `TestPlaintextPerformanceGuard` threshold is too tight for the
current tiger VM state and failed both before and after this change. The
previous commit `a04c4f8` produced 47,706-49,214 req/s across repeated targeted
runs, and `2f0f728` produced 47,674-49,093 req/s. That failure is pre-existing
on this host and is not evidence of a regression from this accept re-arm fix.

## Diagnostic A/B

Result directory:
`/home/tiger/kruda-wing-nonpipelined-io-ef3fb05/bench/reproducible/results/accept-churn-20260604T163105Z`

Command shape:

```bash
wrk --latency -t4 -c256 -d10s -H "Connection: close" \
  http://127.0.0.1:<port>/plaintext-handler
```

| Commit | RPS | Avg latency | Stdev | Max latency | Socket read errors |
|---|---:|---:|---:|---:|---:|
| `a04c4f8` before | 127,624.54 | 1.59 ms | 2.07 ms | 12.92 ms | 1,288,983 |
| `2f0f728` after | 128,204.17 | 1.45 ms | 1.93 ms | 20.75 ms | 1,294,851 |

The diagnostic shows a small connection-churn improvement, but the forced-close
wrk profile reports socket read errors in both rows. Treat it as directional
accept-path evidence only. It is not public benchmark claim evidence and does
not change the CPU-bound handler-route Actix claim gate.
