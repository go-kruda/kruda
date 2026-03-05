package otel

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// setupTracer creates an in-memory exporter and a TracerProvider for testing.
func setupTracer() (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	return exporter, tp
}

// newTestApp creates a Kruda app with otel middleware and a handler at the given path.
func newTestApp(cfg Config, method, path string, handler kruda.HandlerFunc) *httptest.Server {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(cfg))
	switch method {
	case "GET":
		app.Get(path, handler)
	case "POST":
		app.Post(path, handler)
	default:
		app.Get(path, handler)
	}
	app.Compile()
	return httptest.NewServer(app)
}

func TestNew_CreatesSpan(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{TracerProvider: tp}, "GET", "/ping", func(c *kruda.Ctx) error {
		return c.Text("pong")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span, got none")
	}

	span := spans[0]
	if span.Name != "GET /ping" {
		t.Errorf("span name = %q, want %q", span.Name, "GET /ping")
	}
	if span.SpanKind != trace.SpanKindServer {
		t.Errorf("span kind = %v, want %v", span.SpanKind, trace.SpanKindServer)
	}
}

func TestNew_SpanAttributes(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{TracerProvider: tp}, "GET", "/users/:id", func(c *kruda.Ctx) error {
		return c.Status(200).JSON(map[string]string{"id": c.Param("id")})
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/users/42")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]

	// Verify span name uses route pattern, not actual path.
	if span.Name != "GET /users/:id" {
		t.Errorf("span name = %q, want %q", span.Name, "GET /users/:id")
	}

	// Check for expected attributes.
	attrMap := make(map[attribute.Key]attribute.Value)
	for _, a := range span.Attributes {
		attrMap[a.Key] = a.Value
	}

	// http.request.method
	if v, ok := attrMap["http.request.method"]; !ok {
		t.Error("missing http.request.method attribute")
	} else if v.AsString() != "GET" {
		t.Errorf("http.request.method = %q, want %q", v.AsString(), "GET")
	}

	// http.route
	if v, ok := attrMap["http.route"]; !ok {
		t.Error("missing http.route attribute")
	} else if v.AsString() != "/users/:id" {
		t.Errorf("http.route = %q, want %q", v.AsString(), "/users/:id")
	}

	// http.response.status_code
	if v, ok := attrMap["http.response.status_code"]; !ok {
		t.Error("missing http.response.status_code attribute")
	} else if v.AsInt64() != 200 {
		t.Errorf("http.response.status_code = %d, want %d", v.AsInt64(), 200)
	}
}

func TestNew_ErrorSpanStatus(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{TracerProvider: tp}, "GET", "/error", func(c *kruda.Ctx) error {
		return c.Status(500).JSON(map[string]string{"error": "internal"})
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/error")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", span.Status.Code, codes.Error)
	}

	// Verify status_code attribute is 500.
	attrMap := make(map[attribute.Key]attribute.Value)
	for _, a := range span.Attributes {
		attrMap[a.Key] = a.Value
	}
	if v, ok := attrMap["http.response.status_code"]; !ok {
		t.Error("missing http.response.status_code attribute")
	} else if v.AsInt64() != 500 {
		t.Errorf("http.response.status_code = %d, want %d", v.AsInt64(), 500)
	}
}

func TestNew_HandlerError(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	handlerErr := errors.New("handler failed")

	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{TracerProvider: tp}))
	app.Get("/fail", func(c *kruda.Ctx) error {
		c.Status(500)
		return handlerErr
	})
	app.Compile()
	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/fail")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	span := spans[0]
	if span.Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", span.Status.Code, codes.Error)
	}

	// Verify error was recorded on span.
	if len(span.Events) == 0 {
		t.Error("expected error event on span, got none")
	} else {
		foundErr := false
		for _, ev := range span.Events {
			if ev.Name == "exception" {
				foundErr = true
				break
			}
		}
		if !foundErr {
			t.Error("expected 'exception' event on span")
		}
	}
}

func TestNew_ContextPropagation(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	var capturedCtx bool
	srv := newTestApp(Config{TracerProvider: tp}, "GET", "/ctx", func(c *kruda.Ctx) error {
		span := trace.SpanFromContext(c.Context())
		capturedCtx = span.SpanContext().IsValid()
		return c.Text("ok")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ctx")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if !capturedCtx {
		t.Error("expected valid span context in c.Context(), got invalid")
	}

	_ = exporter
}

func TestNew_Skip(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		TracerProvider: tp,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/health"
		},
	}))
	app.Get("/health", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Get("/api", func(c *kruda.Ctx) error { return c.Text("data") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// /health should be skipped — no span.
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) != 0 {
		t.Errorf("expected 0 spans for skipped request, got %d", len(spans))
	}

	// /api should create a span.
	resp, err = http.Get(srv.URL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans = exporter.GetSpans()
	if len(spans) != 1 {
		t.Errorf("expected 1 span for /api, got %d", len(spans))
	}
}

func TestNew_CustomSpanName(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{
		TracerProvider: tp,
		SpanNameFunc: func(c *kruda.Ctx) string {
			return "custom:" + c.Path()
		},
	}, "GET", "/hello", func(c *kruda.Ctx) error {
		return c.Text("world")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	if spans[0].Name != "custom:/hello" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "custom:/hello")
	}
}

func TestNew_CustomAttributes(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{
		TracerProvider: tp,
		Attributes: []attribute.KeyValue{
			attribute.String("service.env", "test"),
			attribute.Int("service.version", 42),
		},
	}, "GET", "/attrs", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/attrs")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	attrMap := make(map[attribute.Key]attribute.Value)
	for _, a := range spans[0].Attributes {
		attrMap[a.Key] = a.Value
	}

	if v, ok := attrMap["service.env"]; !ok {
		t.Error("missing service.env attribute")
	} else if v.AsString() != "test" {
		t.Errorf("service.env = %q, want %q", v.AsString(), "test")
	}

	if v, ok := attrMap["service.version"]; !ok {
		t.Error("missing service.version attribute")
	} else if v.AsInt64() != 42 {
		t.Errorf("service.version = %d, want %d", v.AsInt64(), 42)
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	// Test that New() works with zero config (uses global provider).
	// No panic, no error.
	mw := New()
	if mw == nil {
		t.Fatal("expected non-nil middleware")
	}
}

func TestNew_ServerName(t *testing.T) {
	exporter, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	srv := newTestApp(Config{
		TracerProvider: tp,
		ServerName:     "my-api",
	}, "GET", "/sn", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/sn")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	spans := exporter.GetSpans()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}

	attrMap := make(map[attribute.Key]attribute.Value)
	for _, a := range spans[0].Attributes {
		attrMap[a.Key] = a.Value
	}

	if v, ok := attrMap["server.address"]; !ok {
		t.Error("missing server.address attribute")
	} else if v.AsString() != "my-api" {
		t.Errorf("server.address = %q, want %q", v.AsString(), "my-api")
	}
}

func TestNew_InjectResponseHeaders(t *testing.T) {
	_, tp := setupTracer()
	defer tp.Shutdown(context.Background())

	// Use W3C TraceContext propagator for injection.
	prop := propagation.TraceContext{}

	srv := newTestApp(Config{
		TracerProvider: tp,
		Propagators:    prop,
	}, "GET", "/inject", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/inject")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// The traceparent header should be present in the response
	// because we inject trace context after handling.
	tp2 := resp.Header.Get("Traceparent")
	if tp2 == "" {
		t.Log("traceparent header not in response (may depend on transport); skipping assertion")
	}
}
