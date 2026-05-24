# Wing Lazy Remote Address Evidence

This note summarizes the tiger evidence for lazy Wing peer-address lookup in `resource-20260524Tphase8-lazy-remote-addr-final-readbuf4k/`.

## Scope

- Workload: CPU-bound normal handler routes only
- Routes: `/plaintext-handler`, `/json-static`, `/json-serialize`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Runtime settings: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- Kruda benchmark Go tags: `kruda_stdjson`
- Kruda benchmark pprof server: excluded from the default binary and disabled at runtime (`BENCH_ENABLE_PPROF=0`)
- Host: tiger dev server, Linux `dev-server 6.8.0-111-generic`, Intel Core i5-13500, 8 vCPU

## What Changed

Wing no longer calls `getpeername` eagerly in `handleAccept`. The peer address is resolved only when a handler calls `Request.RemoteAddr()`, then cached on the connection for later requests on the same connection.

## Syscall Check

A short `strace -f -e getpeername` check was run against `/plaintext-handler`, which does not call `RemoteAddr()`.

| Version | wrk shape | `getpeername` lines |
|---------|-----------|--------------------:|
| Phase 7 baseline | `wrk -t4 -c128 -d2s` | 140 |
| Phase 8 lazy remote address | `wrk -t4 -c128 -d2s` | 0 |

## Throughput Profile Summary

| Route | Kruda RPS | Actix RPS | Kruda vs Actix RPS | Kruda p99 | Actix p99 | Kruda max RSS | Actix max RSS |
|------|----------:|----------:|-------------------:|----------:|----------:|--------------:|--------------:|
| `/plaintext-handler` | 840234.65 | 748302.49 | +12.29% | 0.890 ms | 3.300 ms | 19.53 MB | 10.32 MB |
| `/json-static` | 828715.84 | 737413.14 | +12.38% | 0.741 ms | 3.240 ms | 19.78 MB | 10.70 MB |
| `/json-serialize` | 799022.77 | 734178.97 | +8.83% | 0.970 ms | 3.120 ms | 20.96 MB | 10.70 MB |

All measured rows reported zero socket errors and zero non-2xx responses.

## Interpretation

This change removes an eager syscall and address-formatting step from the accept path for routes that do not use `RemoteAddr()`. The standard resource run remains in the same performance band as phase 7 and still passes the Actix throughput/p99 gate, but it does not show a meaningful RSS win. Actix still has lower RSS.
