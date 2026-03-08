// Package otel provides OpenTelemetry tracing middleware for Kruda.
//
// Automatically creates server spans for incoming requests with standard
// HTTP semantic conventions. Supports context propagation, custom span
// naming, skip functions, and extra attributes.
//
// Usage:
//
//	app := kruda.New()
//	app.Use(otel.New(otel.Config{
//	    ServerName: "my-service",
//	}))
package otel

import (
	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Config holds the configuration for the OpenTelemetry tracing middleware.
type Config struct {
	// TracerProvider is the OTel TracerProvider to use.
	// Default: otel.GetTracerProvider() (global provider).
	TracerProvider trace.TracerProvider

	// Propagators is the TextMapPropagator used for context extraction/injection.
	// Default: otel.GetTextMapPropagator() (global propagator).
	Propagators propagation.TextMapPropagator

	// ServerName is the server name to include in span attributes.
	// Default: "" (omitted).
	ServerName string

	// Skip is an optional function that returns true for requests that should
	// bypass tracing (e.g. health checks, metrics endpoints).
	Skip func(*kruda.Ctx) bool

	// SpanNameFunc is an optional function to customize span names.
	// Default: "METHOD route" (e.g. "GET /users/:id").
	SpanNameFunc func(*kruda.Ctx) string

	// Attributes is an optional list of extra attributes to add to every span.
	Attributes []attribute.KeyValue
}
