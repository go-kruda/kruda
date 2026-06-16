# Observability

Turnkey OpenTelemetry with `contrib/observability`: one `Enable` call wires
distributed tracing, RED metrics, `trace_id`/`span_id` log correlation, and the
`/livez`, `/readyz`, `/health`, `/metrics` endpoints.

`observability.Enable` is called **before** any route is registered — the span
middleware bakes into route chains at registration time, so routes added before
`Enable` are not instrumented.

## Run

```bash
cd examples/observability
# Local dev: print spans to stdout instead of shipping to a Collector.
OTEL_TRACES_EXPORTER=console go run .
```

Without `OTEL_TRACES_EXPORTER=console` (or an `OTEL_EXPORTER_OTLP_ENDPOINT`
pointing at a Collector), the default OTLP exporter logs one warning and drops
telemetry.

## Test

```bash
# Handler response (emits a span; check stdout for the console exporter output)
curl http://localhost:8080/hello

# Probes
curl http://localhost:8080/livez    # cheap 200 liveness
curl http://localhost:8080/readyz   # readiness (runs DI health checkers)
curl http://localhost:8080/health   # alias of /readyz
```

## Metrics

`/metrics` (Prometheus) is **internal by default** — served on a separate
loopback listener, not the app port. To scrape it from the app port for this
demo, set `MetricsPublic: true` in the `observability.Config` and scrape
`http://localhost:8080/metrics`. See `contrib/observability/README.md` for the
full exposure and auth options.
