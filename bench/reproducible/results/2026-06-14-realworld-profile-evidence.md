# Realistic API Profile Evidence

Date: 2026-06-14 UTC
Host: tiger Linux dev server (`dev-server` / `tiger-linux`), Go 1.26.3,
wrk 4.1.0 (epoll), TechEmpower-style local PostgreSQL `hello_world`.
Source workdir: `/home/tiger/kruda-realworld-20260614` copied from the local
working tree, not a git checkout.
Framework versions: Fiber `github.com/gofiber/fiber/v2 v2.52.13`; Actix Web
`4.13.0`.

Command:

```bash
cd /home/tiger/kruda-realworld-20260614/bench/reproducible
PATH=$HOME/.cargo/bin:/snap/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
KRUDA_PORT=36100 FIBER_PORT=36102 ACTIX_PORT=36103 \
BENCH_ENABLE_DB=1 BENCH_ROUNDS=5 BENCH_DURATION=8s \
RESULT_DIR=results/realworld-profile-stable-fiber-20260614T101535Z \
./bench.sh 'realworld-profile/42?include=summary&limit=3&token=benchmark-token'
```

Route under test:

```text
GET /realworld-profile/:id?include=summary&limit=3&token=benchmark-token
```

The route intentionally combines common production API work while reusing the
existing TechEmpower `world` table: route params, query validation, an auth
gate, request-id header handling, one PostgreSQL read, and a nested JSON
response envelope. This is not a TechEmpower route and should be claimed
separately from the CPU-only and DB-only benchmark routes.

Every cell below had zero socket errors and zero non-2xx responses.

## Median results (5 rounds)

| Profile | Framework | RPS | p50 ms | p90 ms | p99 ms | max ms |
|---|---|---:|---:|---:|---:|---:|
| latency | Kruda | 105,992.60 | 1.080 | 1.920 | 6.010 | 20.680 |
| latency | Fiber | 106,189.11 | 1.070 | 1.990 | 7.950 | 23.590 |
| latency | Actix | 36,133.00 | 2.810 | 15.220 | 29.270 | 42.430 |
| throughput | Kruda | 103,773.08 | 2.290 | 3.400 | 5.460 | 19.660 |
| throughput | Fiber | 101,059.66 | 2.280 | 3.550 | 10.430 | 23.980 |
| throughput | Actix | 35,907.74 | 5.710 | 19.200 | 32.440 | 43.660 |

## Deltas

| Profile | Comparison | RPS delta | p99 delta |
|---|---|---:|---:|
| latency | Kruda vs Fiber | -0.19% | -24.40% |
| throughput | Kruda vs Fiber | +2.68% | -47.65% |
| latency | Kruda vs Actix | +193.34% | -79.47% |
| throughput | Kruda vs Actix | +189.00% | -83.17% |

## Reading

- Versus Fiber v2.52.13, the realworld route is RPS parity at the PostgreSQL
  ceiling: -0.19% on latency profile and +2.68% on throughput profile. This should be
  described as "same ballpark RPS", not a Kruda throughput win.
- Kruda's p99 is materially better than Fiber in both profiles (-24.40% and
  -47.65%). This is the useful real-world signal: after auth, validation,
  header work, one DB read, and JSON envelope serialization, Kruda keeps a
  shorter tail while matching Fiber's DB-bound throughput.
- Versus Actix Web 4.13.0, Kruda wins this route decisively on both RPS
  (+189% to +193%) and p99 (-79% to -83%) on this same-host local PostgreSQL
  workload.

## Claim wording

Good:

> On a realistic local PostgreSQL API route with auth/validation/request-id
> handling and nested JSON output, Kruda matches Fiber on DB-bound RPS and keeps
> materially lower p99 latency; it is much faster than Actix on this route.

Avoid:

> Kruda is faster than Fiber in real-world APIs.

This route is one reproducible local profile, not a broad production claim.
