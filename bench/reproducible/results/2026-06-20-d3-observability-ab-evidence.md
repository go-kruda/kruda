# D3 observability enabled-path A/B — 2026-06-20

Quantifies the per-request cost of `observability.Enable` on Kruda's Wing
fast-lane routes, to back the `contrib/observability` scaling note (HPA
re-baselining guidance). The disabled path is already proven zero-cost by the
package's alloc guards; this measures what a user pays when they turn the bundle
**on**.

## Method

- Harness: `bench/reproducible/obs-ab.sh` (branch `bench/d3-observability-ab`),
  one `kruda-bench` binary, `BENCH_ENABLE_OBS` toggle.
- Box: tiger, 13th Gen Intel i5-13500 (8 cores used), Linux 6.8, go1.25.10,
  `-tags kruda_stdjson`. Loopback `wrk -t4 -c128 -d10s`, 5 rounds per cell.
- Routes: `/` (plaintext), `/json` (serialize), `/json-static` — the Wing
  string/JSON fast-lane routes (no I/O), i.e. the worst case for per-request
  overhead.
- Arms:
  - **off** — `BENCH_ENABLE_OBS` unset (baseline).
  - **metrics** — `Enable` with `TracesEnabled=false` (RED metrics + health hook
    only). `OTEL_METRICS_EXPORTER=none`.
  - **full** — `Enable` default (traces + RED metrics). `OTEL_TRACES_EXPORTER=none`
    `OTEL_METRICS_EXPORTER=none` — spans are still created/recorded, only the
    network export is a no-op, so the number is the per-request hot-path cost a
    user pays regardless of the telemetry backend (a real collector adds more).

## Result — median RPS over 5 rounds (Δ vs off)

| route        | off (RPS) | metrics (RPS) | Δ      | full (RPS) | Δ      |
|--------------|-----------|---------------|--------|------------|--------|
| `/`          | 770,761   | 602,331       | −21.8% | 408,654    | −47.0% |
| `/json`      | 757,325   | 592,567       | −21.8% | 399,886    | −47.2% |
| `/json-static`| 764,666  | 597,242       | −21.9% | 403,270    | −47.3% |

### median p99 latency

| route        | off    | metrics | full   |
|--------------|--------|---------|--------|
| `/`          | 3.45ms | 4.53ms  | 5.77ms |
| `/json`      | 3.62ms | 4.46ms  | 5.50ms |
| `/json-static`| 3.54ms| 4.55ms  | 5.66ms |

## Conclusions

- **The OnResponse RED-metrics hook alone costs ~22% throughput** on fast-lane
  routes. This confirms the documented mechanism: a response hook drops Wing's
  zero-copy single-handler fast lane (the lane only applies when nothing needs
  the response), so the response takes the normal header-write path.
- **Per-request tracing roughly doubles that cost** — the full bundle is ~−47%
  RPS and +50–65% p99. Tracing (span start/end, attributes, context
  propagation) is the larger half of the full-bundle cost.
- **Framing:** these routes serve ~760K RPS uninstrumented over loopback — the
  fastest possible case, where a fixed per-request overhead is most visible.
  Real routes that do I/O (DB, cache, upstream calls) are handler-bound, so the
  same fixed overhead is a far smaller fraction of their budget.
- **Mitigations** for hot in-memory routes: sample traces (`Config.SampleRatio`)
  to cut the tracing half; run metrics-only (`TracesEnabled=false`) if traces are
  not needed; or leave observability off on the very hottest routes. Re-baseline
  HPA/autoscaling targets against an instrumented build, not pre-observability
  numbers.

Raw `wrk` output stayed on tiger (summary only copied back). Reproduce with
`bench/reproducible/obs-ab.sh`.
