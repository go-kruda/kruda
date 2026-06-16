package kruda

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// captureApp builds an App whose logger writes JSON to buf, with the given enricher.
func captureApp(t *testing.T, buf *bytes.Buffer, enricher func(*Ctx) []slog.Attr) *App {
	t.Helper()
	h := slog.NewJSONHandler(buf, nil)
	opts := []Option{WithLogger(slog.New(h))}
	if enricher != nil {
		opts = append(opts, WithLogEnricher(enricher))
	}
	return New(opts...)
}

// TestWithLogEnricher_AttachesAttrAtEmitTime verifies the enricher runs per-record:
// a logger cached BEFORE the enricher's data is available still gets the attr.
func TestWithLogEnricher_AttachesAttrAtEmitTime(t *testing.T) {
	var buf bytes.Buffer
	app := captureApp(t, &buf, func(c *Ctx) []slog.Attr {
		if v, ok := c.locals["trace_id"].(string); ok && v != "" {
			return []slog.Attr{slog.String("trace_id", v)}
		}
		return nil
	})
	c := app.ctxPool.Get().(*Ctx)
	t.Cleanup(func() { c.cleanup(); app.ctxPool.Put(c) })

	// Cache the logger BEFORE trace_id exists.
	lg := c.Log()
	// Now a later hook sets trace_id.
	c.Provide("trace_id", "abc123")
	lg.Info("hello")

	if !strings.Contains(buf.String(), `"trace_id":"abc123"`) {
		t.Fatalf("enricher did not attach trace_id at emit time: %s", buf.String())
	}
}

// TestWithLogEnricher_NilSafeNoSpan verifies an enricher returning nil is a no-op.
func TestWithLogEnricher_NilSafeNoSpan(t *testing.T) {
	var buf bytes.Buffer
	app := captureApp(t, &buf, func(c *Ctx) []slog.Attr { return nil })
	c := app.ctxPool.Get().(*Ctx)
	t.Cleanup(func() { c.cleanup(); app.ctxPool.Put(c) })
	c.Log().Info("plain")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("log line not valid JSON: %v (%s)", err, buf.String())
	}
	if _, present := rec["trace_id"]; present {
		t.Fatalf("nil enricher must not add attrs: %s", buf.String())
	}
}

// TestWithLogEnricher_Disabled verifies zero behavior change when no enricher is set.
func TestWithLogEnricher_Disabled(t *testing.T) {
	var buf bytes.Buffer
	app := captureApp(t, &buf, nil)
	if app.logEnricher != nil {
		t.Fatal("logEnricher must be nil when WithLogEnricher not passed")
	}
	c := app.ctxPool.Get().(*Ctx)
	t.Cleanup(func() { c.cleanup(); app.ctxPool.Put(c) })
	c.Provide("trace_id", "should-not-appear")
	c.Log().Info("x")
	if strings.Contains(buf.String(), "trace_id") {
		t.Fatalf("no enricher should mean no trace_id: %s", buf.String())
	}
}

// TestSetLogEnricher_PostConstruction verifies the setter half of the seam.
func TestSetLogEnricher_PostConstruction(t *testing.T) {
	var buf bytes.Buffer
	app := captureApp(t, &buf, nil) // no enricher at New()
	app.SetLogEnricher(func(c *Ctx) []slog.Attr {
		return []slog.Attr{slog.String("k", "v")}
	})
	c := app.ctxPool.Get().(*Ctx)
	t.Cleanup(func() { c.cleanup(); app.ctxPool.Put(c) })
	c.Log().Info("x")
	if !strings.Contains(buf.String(), `"k":"v"`) {
		t.Fatalf("SetLogEnricher did not install enricher: %s", buf.String())
	}
}
