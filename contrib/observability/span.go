package observability

import (
	"fmt"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/go-kruda/kruda/contrib/observability"

// spanMiddleware arms the in-flight counter Inc/Dec when m is non-nil and, when
// traces are enabled, starts a server span (panic-safe defer span.End). It runs
// only for matched routes, so it marks reqState.matched=true — the metric hook
// uses that to decide the route label (matched => c.Route(); unmatched 404/405
// => sentinelRoute). reqState + in-flight run regardless of tracesOn because the
// RED metrics path depends on them even in metrics-only mode.
func spanMiddleware(prov *Providers, m *metrics, r resolved) kruda.HandlerFunc {
	tracer := prov.Tracer.Tracer(tracerName)
	prop := prov.Propagator
	if prop == nil {
		prop = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	}
	return func(c *kruda.Ctx) error {
		st := &reqState{startNanos: nowNanos(), matched: true}
		c.Provide(reqStateKey, st)

		if m != nil && m.inflight != nil {
			set := attribute.NewSet(attribute.String("http.route", c.Route()))
			m.inflight.Add(c.Context(), 1, metric.WithAttributeSet(set))
			defer m.inflight.Add(c.Context(), -1, metric.WithAttributeSet(set))
		}

		// Metrics-only mode (traces disabled): no span, just timing + in-flight.
		if !r.tracesOn {
			return c.Next()
		}

		ctx := prop.Extract(c.Context(), &headerCarrier{c: c})
		ctx, span := tracer.Start(ctx, c.Method()+" "+c.Route(),
			trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			semconv.HTTPRequestMethodKey.String(c.Method()),
			attribute.String("http.route", c.Route()),
			semconv.URLScheme(schemeOf(c)),
		)
		c.SetContext(ctx)

		err := c.Next()

		status := c.StatusCode()
		span.SetAttributes(semconv.HTTPResponseStatusCode(status))
		if status >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
		}
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		// Deliberately no response-side traceparent inject: echoing trace context
		// back in the response is non-standard, and it cannot work in Kruda anyway —
		// c.Next() has already committed the response, and injecting before c.Next()
		// would populate respHeaders and disable the string/JSON fast lane on every
		// traced route.
		return err
	}
}

func schemeOf(c *kruda.Ctx) string {
	if c.Header("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}

type headerCarrier struct{ c *kruda.Ctx }

func (h *headerCarrier) Get(k string) string { return h.c.Header(k) }
func (h *headerCarrier) Set(k, v string)     { h.c.SetHeader(k, v) }
func (h *headerCarrier) Keys() []string      { return []string{"traceparent", "tracestate", "baggage"} }
