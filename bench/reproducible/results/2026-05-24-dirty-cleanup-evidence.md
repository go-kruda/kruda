# Dirty Cleanup CPU Evidence

This evidence covers the generic `Ctx` dirty cleanup change for normal Wing handler-path routes. It is not Wing static bypass evidence.

## Environment

- Host: `tiger-linux` (`dev-server`, Linux 6.8.0-111-generic)
- CPU: Intel Core i5-13500, 8 logical CPUs exposed to the benchmark host
- Go: `go1.26.2 linux/amd64`
- Rust/Cargo: `cargo 1.93.1`
- wrk: `debian/4.1.0-4build2 [epoll]`
- Kruda build tag: `kruda_stdjson`
- Runtime: `GOMAXPROCS=8`, `KRUDA_WORKERS=4`, `KRUDA_READ_BUF_SIZE=4096`
- Profiles: `wrk --latency -t4 -c128 -d15s` and `wrk --latency -t4 -c256 -d15s`
- Rounds: one warmup and five measured rounds per framework/profile/route

## Throughput Profile Summary

| Route | Framework | Median RPS | Median p99 ms | Socket errors | Non-2xx |
|------|-----------|-----------:|--------------:|--------------:|--------:|
| `/plaintext-handler` | Kruda | 829459.01 | 0.950 | 0 | 0 |
| `/plaintext-handler` | Actix | 744041.04 | 3.270 | 0 | 0 |
| `/json-static` | Kruda | 831897.15 | 0.940 | 0 | 0 |
| `/json-static` | Actix | 742215.44 | 3.300 | 0 | 0 |

Kruda vs Actix under the throughput profile:

- `/plaintext-handler`: +11.48% median RPS, -70.95% p99.
- `/json-static`: +12.08% median RPS, -71.52% p99.

This satisfies the public "faster than Actix" gate for these CPU-bound handler-path routes. It is not a 20% throughput claim.

## Raw Evidence

- `results/20260524Tphase10-dirty-cleanup-plaintext-readbuf4k-p3630/`
- `results/20260524Tphase10-dirty-cleanup-json-static-readbuf4k-p3640/`
