package kruda

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

// TestUse_AfterRoutesWarnsOnce documents the middleware-ordering contract:
// Use() only affects routes registered after it. Calling Use() once routes
// exist is almost always a mistake, so Kruda warns — once per app.
func TestUse_AfterRoutesWarnsOnce(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	app := New()
	app.Get("/a", func(c *Ctx) error { return c.Text("a") })
	app.Use(func(c *Ctx) error { return c.Next() })
	app.Use(func(c *Ctx) error { return c.Next() })

	out := buf.String()
	if !strings.Contains(out, "Use() called after routes") {
		t.Errorf("expected a late-Use warning, got: %q", out)
	}
	if n := strings.Count(out, "Use() called after routes"); n != 1 {
		t.Errorf("warning should fire exactly once per app, fired %d times", n)
	}
}

// TestUse_BeforeRoutesDoesNotWarn guards against false positives.
func TestUse_BeforeRoutesDoesNotWarn(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)

	app := New()
	app.Use(func(c *Ctx) error { return c.Next() })
	app.Get("/a", func(c *Ctx) error { return c.Text("a") })

	if out := buf.String(); strings.Contains(out, "Use() called after routes") {
		t.Errorf("Use() before routes must not warn, got: %q", out)
	}
}
