package observability

import (
	"context"
	"testing"

	otelglobal "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace/noop"
)

// serviceNameFromResource returns the service.name attribute value, or "" if absent.
func serviceNameFromResource(res *resource.Resource) string {
	if res == nil {
		return ""
	}
	for _, kv := range res.Attributes() {
		if kv.Key == semconv.ServiceNameKey {
			return kv.Value.AsString()
		}
	}
	return ""
}

// TestBuildSDK_ServiceNameChain verifies explicit ServiceName wins.
func TestBuildSDK_ServiceNameChain(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	s, err := buildSDK(context.Background(), Config{ServiceName: "explicit"}.resolve())
	if err != nil {
		t.Fatalf("buildSDK: %v", err)
	}
	t.Cleanup(func() { _ = s.shutdown(context.Background()) })
	if got := serviceNameFromResource(s.res); got != "explicit" {
		t.Fatalf("service.name = %q, want explicit", got)
	}
}

// TestBuildSDK_SetGlobalGate verifies SetGlobal:false leaves globals untouched.
func TestBuildSDK_SetGlobalGate(t *testing.T) {
	prevTP := otelglobal.GetTracerProvider()
	t.Cleanup(func() { otelglobal.SetTracerProvider(prevTP) })
	otelglobal.SetTracerProvider(noop.NewTracerProvider())

	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	s, err := buildSDK(context.Background(), Config{SetGlobal: ptrBool(false)}.resolve())
	if err != nil {
		t.Fatalf("buildSDK: %v", err)
	}
	t.Cleanup(func() { _ = s.shutdown(context.Background()) })

	// Globals untouched => a span from the global provider is non-recording.
	_, span := otelglobal.GetTracerProvider().Tracer("t").Start(context.Background(), "x")
	if span.SpanContext().IsValid() {
		t.Fatal("SetGlobal:false must not install a recording TracerProvider globally")
	}
	span.End()
}
