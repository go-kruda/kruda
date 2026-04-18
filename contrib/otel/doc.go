// Package otel provides OpenTelemetry tracing middleware for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/otel"
//
//	app := kruda.New()
//	app.Use(otel.New(otel.Config{
//	    ServerName: "my-service",
//	}))
//
// # What it does
//
// The middleware extracts the trace context from incoming request headers
// using the configured propagator, starts a server span named after the
// route pattern (e.g. "GET /users/:id"), and records standard HTTP
// semantic-convention attributes (http.method, http.route, http.status_code,
// etc.). The span and trace context are attached to the request context so
// downstream calls can continue the trace.
//
// Span errors are recorded via [trace.Span.RecordError] when the handler
// returns a non-nil error or sets a 5xx status. Use [Skip] to avoid tracing
// noisy endpoints (health checks, metrics endpoints).
//
// # Configuration
//
//   - TracerProvider: defaults to otel.GetTracerProvider() (global)
//   - Propagators:    defaults to otel.GetTextMapPropagator() (global)
//   - ServerName:     added as a span attribute when set
//   - SpanNameFunc:   override default "METHOD route" naming
//   - Attributes:     extra attributes added to every span
//   - Skip:           per-request bypass function
//
// # See also
//
//   - https://opentelemetry.io/docs/specs/semconv/http/http-spans/
package otel
