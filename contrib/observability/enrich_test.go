package observability

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/trace"
)

// TestEnricher_EmitsValidTraceID verifies the enricher returns trace_id/span_id
// when a valid span is in the context, and nothing when there is no span.
func TestEnricher_EmitsValidTraceID(t *testing.T) {
	tid, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	sid, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: tid, SpanID: sid})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	attrs := traceAttrsFromContext(ctx)
	got := map[string]string{}
	for _, a := range attrs {
		got[a.Key] = a.Value.String()
	}
	if got["trace_id"] != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("trace_id = %q", got["trace_id"])
	}
	if got["span_id"] != "0123456789abcdef" {
		t.Fatalf("span_id = %q", got["span_id"])
	}

	if a := traceAttrsFromContext(context.Background()); len(a) != 0 {
		t.Fatalf("no span => no attrs, got %v", a)
	}
}

// TestEnricher_ReachesLogOutput drives a request through an Enable'd app and
// asserts the handler's c.Log() line carries a real (non-zero) trace_id.
func TestEnricher_ReachesLogOutput(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	var buf bytes.Buffer
	app := kruda.New(kruda.NetHTTP(), kruda.WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))))
	_, err := Enable(app, Config{ServiceName: "t", SetGlobal: ptrBool(false)})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	app.Get("/x", func(c *kruda.Ctx) error {
		c.Log().Info("hi")
		return c.Status(200).Text("ok")
	})
	app.Compile()
	doGET(t, app, "/x")
	if !strings.Contains(buf.String(), `"trace_id":`) {
		t.Fatalf("c.Log() output missing trace_id: %s", buf.String())
	}
	// trace_id must be a real (non-zero) id.
	if strings.Contains(buf.String(), `"trace_id":"00000000000000000000000000000000"`) {
		t.Fatalf("trace_id is all-zero (span not recording): %s", buf.String())
	}
}
