// Example: Observability — turnkey OpenTelemetry via contrib/observability
//
// Demonstrates one-call tracing, RED metrics, log correlation, and k8s probes:
//   - observability.Enable(app, cfg) before any route is registered
//   - trace_id/span_id auto-injected into c.Log()
//   - /livez, /readyz, /health probes and /metrics mounted automatically
//   - defer prov.Flush(...) for a bounded shutdown flush
//
// Run (local dev, spans printed to stdout):
//
//	OTEL_TRACES_EXPORTER=console go run ./examples/observability/
//
// Test:
//
//	curl http://localhost:8080/hello   → JSON, and a span is exported
//	curl http://localhost:8080/livez   → 200 liveness
//	curl http://localhost:8080/readyz  → readiness
package main

import (
	"context"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/contrib/observability"
)

func main() {
	app := kruda.New()

	// Enable BEFORE registering routes — the span middleware bakes into route
	// chains at addRoute time, so routes registered before Enable are NOT instrumented.
	prov, err := observability.Enable(app, observability.Config{ServiceName: "demo"})
	if err != nil {
		panic(err)
	}
	defer prov.Flush(context.Background())

	app.Get("/hello", func(c *kruda.Ctx) error {
		c.Log().Info("handling hello") // auto-correlated with trace_id
		return c.Status(200).JSON(kruda.Map{"msg": "hi"})
	})

	_ = app.Listen(":8080")
}
