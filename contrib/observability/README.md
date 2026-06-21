# contrib/observability

Turnkey OpenTelemetry for Kruda: distributed tracing, RED metrics, log
correlation, and Kubernetes probes from a single call. It wires OTel SDK
providers (driven by the standard `OTEL_*` env vars via `autoexport`), installs
a panic-safe span middleware plus an always-fires RED-metrics hook, injects
`trace_id`/`span_id` into `c.Log()`, and mounts `/livez`, `/readyz`, `/health`,
and `/metrics`.

```go
app := kruda.New()

prov, err := observability.Enable(app, observability.Config{ServiceName: "demo"})
if err != nil {
    panic(err)
}
defer prov.Flush(context.Background()) // bounded shutdown flush

app.Get("/hello", func(c *kruda.Ctx) error {
    c.Log().Info("handling hello") // auto-correlated: trace_id, span_id
    return c.Status(200).JSON(kruda.Map{"msg": "hi"})
})

app.Listen(":8080")
```

## Enable before routes (hard rule)

**Call `observability.Enable` BEFORE registering any route.** The span
middleware is installed via `app.Use`, and Kruda bakes middleware into each
route's handler chain at registration time (`addRoute`). Routes registered
*before* `Enable` will run, but they will **not** be instrumented — no span, no
log correlation. The RED-metrics hook (an `OnResponse` hook) still fires for
those routes, but you lose tracing on them. Register middleware first, routes
second.

## Before you import: version floors

This module pins the OTel v1.44.0 train and imposes minimum versions on the
importing application. Make sure your `go.mod` can satisfy:

| Dependency | Minimum |
|---|---|
| Go | 1.25 |
| `go.opentelemetry.io/otel` (+ `sdk`, `metric`, `trace`) | v1.43.0 (this module pins v1.44.0) |
| `go.opentelemetry.io/contrib/*` (otelhttp, otelgrpc, autoexport, exporters) | matching contrib train (v0.69.x for the v1.44.0 core) |
| `google.golang.org/grpc` | v1.79.1 |
| `google.golang.org/protobuf` | v1.36.11 |
| `github.com/prometheus/client_golang` | v1.22.0 |

### The gRPC floor is a package contract

Importing this package pulls in `autoexport` — the turnkey, env-driven exporter
mechanism that lets `OTEL_EXPORTER_OTLP_PROTOCOL=grpc` work out of the box.
`autoexport` links the OTLP/gRPC exporter, so **importing `contrib/observability`
imposes the grpc-go (≥ v1.79.1) and protobuf (≥ v1.36.11) floors on your app
even if you never send a gRPC request.** There is no runtime cost when gRPC is
unused — it is purely a compile-time/version constraint.

If your application cannot move to those floors (e.g. it is pinned to an older
grpc-go), do not use this turnkey package. Use the à-la-carte
[`contrib/otel`](../otel) + [`contrib/prometheus`](../prometheus) packages
instead and wire the exporters you actually need.

## Configuration via `OTEL_*` env

`autoexport` reads the standard OpenTelemetry environment variables, so most
deployment config lives outside your code:

| Variable | Purpose |
|---|---|
| `OTEL_SERVICE_NAME` | Service name (overridden by `Config.ServiceName` when set) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Collector endpoint, e.g. `http://collector:4317` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` (port 4317) or `http/protobuf` (port 4318) |
| `OTEL_TRACES_EXPORTER` | `otlp` (default), `console`, or `none` |
| `OTEL_METRICS_EXPORTER` | `otlp`, `prometheus`, `console`, or `none` |
| `OTEL_TRACES_SAMPLER` / `OTEL_TRACES_SAMPLER_ARG` | Sampler selection (see Sampling) |

> **Local dev:** the default trace exporter is OTLP. With no
> `OTEL_EXPORTER_OTLP_ENDPOINT` set, the SDK logs one warning and **drops**
> telemetry. For local development without a Collector, set
> `OTEL_TRACES_EXPORTER=console` to print spans to stdout.

## `/metrics` exposure

The Prometheus `/metrics` endpoint is **internal by default**: it is served on a
separate loopback listener, not on your public app port. This keeps cardinality
and internals off the public surface. To change that:

- `Config.MetricsPublic: true` — mount `/metrics` on the app/public port.
- `Config.MetricsBind: "127.0.0.1:9090"` — serve on a specific separate address.
- `Config.MetricsAuth: "<token>"` — require `Authorization: Bearer <token>`;
  the compare is constant-time over the SHA-256 of the token. Use this whenever
  `/metrics` is reachable beyond loopback.
- `Config.MetricsPath` — change the path (default `/metrics`).

## Kubernetes probes

`Enable` mounts three probe endpoints (gated by `Config.HealthEnabled`, default
on):

- `/livez` — cheap, always-200 liveness check. No dependency calls.
- `/readyz` — readiness: runs DI-registered `HealthChecker`s; degrades to "ok"
  gracefully. The readiness health-check timeout defaults **below 1s** (900ms)
  so a slow dependency does not make the probe itself flap on its own timeout.
- `/health` — alias of `/readyz` (set `Config.HealthPath: ""` to disable).

Paths are configurable via `LivenessPath` / `ReadinessPath` / `HealthPath`.

```yaml
livenessProbe:
  httpGet: { path: /livez, port: 8080 }
  periodSeconds: 10
  timeoutSeconds: 1
  failureThreshold: 3
readinessProbe:
  httpGet: { path: /readyz, port: 8080 }
  periodSeconds: 10
  timeoutSeconds: 2
  failureThreshold: 3
# For slow DI/dependency boots, reuse /livez as a startupProbe so the kubelet
# waits for the process to come up before liveness/readiness start counting.
startupProbe:
  httpGet: { path: /livez, port: 8080 }
  periodSeconds: 5
  timeoutSeconds: 1
  failureThreshold: 30   # ~150s of grace before liveness takes over
```

## Sampling honesty

By default the sampler honors `OTEL_TRACES_SAMPLER` (a `ParentBased` sampler).
`Config.SampleRatio` overrides it **only when set** (> 0); leave it zero to defer
to the env.

This is **head sampling** — the keep/drop decision is made at span *start*,
before the response status is known. That means **head sampling cannot
force-sample 5xx responses**: by the time a handler returns an error, the
sampling decision has already been made. If you need a guarantee that every
error trace is kept, do **tail sampling in the Collector** (e.g. the OTel
Collector `tail_sampling` processor with a status-code policy).

## Performance note

Enabling observability installs an `OnResponse` hook. On Wing, the presence of a
response hook **drops the single-handler fast lane** for instrumented routes
(the fast lane only applies when no hook needs the response), and per-request
tracing adds span work on top.

Measured on a paired loopback A/B (i5-13500, `wrk -t4 -c128`, fast-lane routes
`/`, `/json`, `/json-static` serving ~760K RPS uninstrumented — see
`bench/reproducible/results/2026-06-20-d3-observability-ab-evidence.md`):

| config                              | RPS vs off | p99       |
|-------------------------------------|------------|-----------|
| RED metrics only (`TracesEnabled:false`) | ~−22%  | ~+27%     |
| full bundle (traces + metrics)      | ~−47%      | ~+55%     |

This is the worst case: handler-bound routes that do real I/O (DB, cache,
upstream calls) spend that fixed overhead against a much larger budget, so the
relative hit is far smaller. To trim the cost on hot routes, sample traces
(`Config.SampleRatio`) — tracing is the larger half — or run metrics-only. Either
way, **re-baseline your HPA / autoscaling targets against an instrumented build**
rather than reusing pre-observability numbers.

## gRPC and outbound HTTP propagation

`Enable` sets the OTel globals (propagator + tracer/meter providers), gated by
`Config.SetGlobal` (default true). With the globals set:

- **gRPC:** `otelgrpc` stats handlers read the globals, so inbound/outbound gRPC
  tracing Just Works with no wrapper from this package — provided you build your
  gRPC handlers/clients *after* `Enable` set the globals.
- **Outbound HTTP:** wrap your client so the trace context propagates:

  ```go
  client := observability.HTTPClient()                 // ready-to-use *http.Client
  // or
  client := &http.Client{Transport: observability.RoundTripper(http.DefaultTransport)}
  ```

- **Kafka:** there is no maintained first-party OTel wrapper for `segmentio/kafka-go`.
  If you need Kafka trace propagation, use `IBM/sarama` with `dnwe/otelsarama`,
  or hand-roll a carrier that copies the trace context into Kafka headers.

## Flush footgun

`prov.Flush(ctx)` is the **shutdown flush** — it is wired to `OnShutdown` and is
once-guarded. Calling it doubles as a provider shutdown: it flushes **and shuts
the providers down**. Do **not** call `prov.Flush` mid-run to "force a flush" —
that will tear down your exporters for the rest of the process. Rely on the
automatic `OnShutdown` flush (or a single `defer prov.Flush(context.Background())`
at exit) and nothing else.
