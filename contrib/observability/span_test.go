package observability

import (
	"context"
	"testing"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// newRecordingProviders builds Providers backed by an in-memory span recorder.
func newRecordingProviders(t *testing.T) (*Providers, *tracetest.SpanRecorder) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	return &Providers{Tracer: tp}, sr
}

// TestSpanMiddleware_NamedByRoutePattern verifies the span is named by the route
// pattern (not the concrete path) and ends even when the handler runs normally.
func TestSpanMiddleware_NamedByRoutePattern(t *testing.T) {
	prov, sr := newRecordingProviders(t)
	app := kruda.New(kruda.NetHTTP())
	app.Use(spanMiddleware(prov, nil, Config{}.resolve())) // m=nil: metrics wired in Task 5
	app.Get("/users/:id", func(c *kruda.Ctx) error { return c.Status(200).Text("ok") })
	app.Compile()

	doGET(t, app, "/users/42")

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("want 1 span, got %d", len(spans))
	}
	if spans[0].Name() != "GET /users/:id" {
		t.Fatalf("span name = %q, want GET /users/:id", spans[0].Name())
	}
}

// TestSpanMiddleware_NoResponseSideTraceparent guards the deliberate decision not
// to echo trace context back in the response. Echoing traceparent is non-standard,
// and injecting before c.Next() — the only point at which it could reach the wire —
// would populate respHeaders and disable the string/JSON fast lane on every traced
// route. The header must never appear on the response.
func TestSpanMiddleware_NoResponseSideTraceparent(t *testing.T) {
	prov, _ := newRecordingProviders(t)
	app := kruda.New(kruda.NetHTTP())
	app.Use(spanMiddleware(prov, nil, Config{}.resolve()))
	app.Get("/x", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Compile()

	rec := doGET(t, app, "/x")
	if got := rec.Header().Get("Traceparent"); got != "" {
		t.Fatalf("response carried Traceparent=%q; Kruda must not echo trace context in responses", got)
	}
}

// TestSpanMiddleware_TracesDisabledNoSpan verifies metrics-only mode records NO span
// but still runs (in-flight/reqState path) so RED metrics keep working.
func TestSpanMiddleware_TracesDisabledNoSpan(t *testing.T) {
	prov, sr := newRecordingProviders(t)
	r := Config{TracesEnabled: ptrBool(false)}.resolve()
	app := kruda.New(kruda.NetHTTP())
	app.Use(spanMiddleware(prov, nil, r))
	app.Get("/x", func(c *kruda.Ctx) error { return c.Status(200).Text("ok") })
	app.Compile()
	doGET(t, app, "/x")
	if n := len(sr.Ended()); n != 0 {
		t.Fatalf("traces disabled must record 0 spans, got %d", n)
	}
}
