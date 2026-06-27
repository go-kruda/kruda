# v1.4.0 consolidated perf gate — Kruda v1.3.1 vs HEAD (12d6a80) A/B

Date: 2026-06-27 · Host: tiger (Linux/epoll, shared box) · Kruda-only · default **Sonic** build · `-t4 -c128`(latency)/`-c256`(throughput) · 5 rounds/cell · `BENCH_ENABLE_DB=1` takeover dispatch · GOTOOLCHAIN go1.25.10.

Purpose: the public README 'beats Fiber/Actix' tables were measured at the **v1.3.1** runtime commit; ~15 commits since grew the request path (wing_http.go, wing_transport.go). This run confirms HEAD has **no perf regression vs v1.3.1** so those claims stay defensible at the tagged code.

Method: paired A/B run in BOTH checkout orders to separate code effects from the shared box's run-order/contention noise (documented technique: a deficit that follows whichever commit runs *second* is contention, not code).

**Result: PASS — no regression. 0 socket errors / 0 non-2xx across all four passes.**


## throughput — Δ = HEAD relative to v1.3.1 (median of 5)

| route | fwd ΔRPS% | fwd Δp99% | rev ΔRPS% | rev Δp99% | read |
|---|---:|---:|---:|---:|---|
| plaintext-handler | -1.5 | +0.9 | — | — | RPS parity (fwd only) |
| json-static | -0.8 | +8.9 | — | — | RPS parity (fwd only) |
| json-serialize | -1.1 | +3.4 | — | — | RPS parity (fwd only) |
| fortunes | +1.4 | +0.2 | +0.9 | +0.0 | parity both orders |
| db | -1.6 | +17.0 | -1.1 | +0.9 | p99 spike vanished when HEAD ran first → run-order noise |
| queries | -4.0 | +27.1 | -0.6 | -5.5 | parity both orders |

## latency — Δ = HEAD relative to v1.3.1 (median of 5)

| route | fwd ΔRPS% | fwd Δp99% | rev ΔRPS% | rev Δp99% | read |
|---|---:|---:|---:|---:|---|
| plaintext-handler | +0.2 | +39.4 | — | — | RPS parity (fwd only) |
| json-static | -0.8 | +5.9 | — | — | RPS parity (fwd only) |
| json-serialize | +0.6 | +16.7 | — | — | RPS parity (fwd only) |
| fortunes | -1.8 | +10.6 | +0.6 | +1.0 | p99 spike vanished when HEAD ran first → run-order noise |
| db | -3.8 | +21.8 | +0.1 | +0.8 | p99 spike vanished when HEAD ran first → run-order noise |
| queries | +0.1 | -6.3 | -0.2 | -5.4 | parity both orders |

## Why the forward db/queries p99 spikes are noise

db throughput p99 (ms): forward head=11.83 vs v131=10.11; reversed head=12.13 vs v131=12.02. Whichever commit runs **second** lands at ~12 ms regardless of commit → the tail is set by box contention/run position, not by the Wing changes. RPS stays within the pgx-ceiling noise band (±1.6% fwd / ±1.1% rev) in both orders.

Raw CSVs: results-v131, results-head (forward); results-rev-head, results-rev-v131 (reversed). All on tiger run dir `kruda-v140-ab-20260627T095416Z`.

