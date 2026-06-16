package observability

import (
	"context"
	"log/slog"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/trace"
)

// traceAttrsFromContext returns trace_id/span_id slog attrs for a valid span
// context, or nil. The hex IDs are inherently well-formed (no injection risk).
func traceAttrsFromContext(ctx context.Context) []slog.Attr {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return nil
	}
	return []slog.Attr{
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	}
}

func logEnricher(c *kruda.Ctx) []slog.Attr {
	return traceAttrsFromContext(c.Context())
}
