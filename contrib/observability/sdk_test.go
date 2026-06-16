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

// TestBuildSDK_PerAppRegistryNoCollision verifies two metrics-enabled SDKs each
// get an independent Prometheus registry (no global DefaultRegisterer collision),
// so both can be gathered without "collected before" errors.
func TestBuildSDK_PerAppRegistryNoCollision(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none") // keep autoexport off; otelprom reader still added
	r := Config{SetGlobal: ptrBool(false)}.resolve()

	s1, err := buildSDK(context.Background(), r)
	if err != nil {
		t.Fatalf("buildSDK 1: %v", err)
	}
	t.Cleanup(func() { _ = s1.shutdown(context.Background()) })
	s2, err := buildSDK(context.Background(), r)
	if err != nil {
		t.Fatalf("buildSDK 2: %v", err)
	}
	t.Cleanup(func() { _ = s2.shutdown(context.Background()) })

	if s1.promReg == nil || s2.promReg == nil {
		t.Fatal("metrics-on SDK must have a dedicated promReg")
	}
	if s1.promReg == s2.promReg {
		t.Fatal("each app must get its OWN registry, not a shared one")
	}
	// Both registries must Gather without error (would fail if they shared the global).
	if _, err := s1.promReg.Gather(); err != nil {
		t.Fatalf("registry 1 Gather: %v", err)
	}
	if _, err := s2.promReg.Gather(); err != nil {
		t.Fatalf("registry 2 Gather: %v", err)
	}
}
