package otel

import (
	"fmt"

	"github.com/go-kruda/kruda"
	otelglobal "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/go-kruda/kruda/contrib/otel"

// New creates an OpenTelemetry tracing middleware.
// It extracts trace context from incoming headers, creates a server span,
// sets standard HTTP attributes, and injects trace context into response headers.
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}

	// Apply defaults from global OTel providers.
	tp := cfg.TracerProvider
	if tp == nil {
		tp = otelglobal.GetTracerProvider()
	}

	propagators := cfg.Propagators
	if propagators == nil {
		propagators = otelglobal.GetTextMapPropagator()
	}

	tracer := tp.Tracer(tracerName)

	// Pre-compute extra attributes slice once at registration time.
	extraAttrs := cfg.Attributes

	spanNameFn := cfg.SpanNameFunc
	if spanNameFn == nil {
		spanNameFn = defaultSpanName
	}

	return func(c *kruda.Ctx) error {
		// Skip check — zero alloc on skip path.
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		// Extract trace context from incoming request headers.
		ctx := propagators.Extract(c.Context(), &headerCarrier{c: c})

		// Build span name.
		spanName := spanNameFn(c)

		// Start server span.
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// Set initial span attributes.
		attrs := make([]attribute.KeyValue, 0, 4+len(extraAttrs))
		attrs = append(attrs,
			semconv.HTTPRequestMethodKey.String(c.Method()),
			semconv.URLPath(c.Path()),
			semconv.HTTPRoute(c.Route()),
		)
		if host := c.Header("Host"); host != "" {
			attrs = append(attrs, semconv.ServerAddress(host))
		}
		if cfg.ServerName != "" {
			attrs = append(attrs, semconv.ServerAddress(cfg.ServerName))
		}
		attrs = append(attrs, extraAttrs...)
		span.SetAttributes(attrs...)

		// Propagate trace context to downstream handlers.
		c.SetContext(ctx)

		// Execute downstream handlers.
		err := c.Next()

		// Set response attributes.
		statusCode := c.StatusCode()
		span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))

		// Mark span as error for 5xx responses.
		if statusCode >= 500 {
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}

		// Record handler error on span.
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}

		// Inject trace context into response headers.
		propagators.Inject(ctx, &responseHeaderCarrier{c: c})

		return err
	}
}

// defaultSpanName returns "METHOD route" (e.g. "GET /users/:id").
func defaultSpanName(c *kruda.Ctx) string {
	return c.Method() + " " + c.Route()
}

// headerCarrier adapts Kruda's Ctx to propagation.TextMapCarrier
// for extracting trace context from incoming request headers.
type headerCarrier struct {
	c *kruda.Ctx
}

func (h *headerCarrier) Get(key string) string {
	return h.c.Header(key)
}

func (h *headerCarrier) Set(key, value string) {
	h.c.SetHeader(key, value)
}

func (h *headerCarrier) Keys() []string {
	return []string{"traceparent", "tracestate"}
}

// responseHeaderCarrier adapts Kruda's Ctx to propagation.TextMapCarrier
// for injecting trace context into response headers.
type responseHeaderCarrier struct {
	c *kruda.Ctx
}

func (h *responseHeaderCarrier) Get(key string) string {
	return ""
}

func (h *responseHeaderCarrier) Set(key, value string) {
	h.c.SetHeader(key, value)
}

func (h *responseHeaderCarrier) Keys() []string {
	return nil
}
