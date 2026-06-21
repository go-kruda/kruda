package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// TestLogger_RecordsLatency guards against the Logger reporting latency=0 on
// every request. The middleware must call c.MarkStart() before c.Next() so
// c.Latency() measures real elapsed time — MarkStart is opt-in (the Logger is
// its only intended caller) precisely so non-logging apps skip the time.Now()
// cost. Before the fix, MarkStart() had no caller, so c.Latency() was always 0.
func TestLogger_RecordsLatency(t *testing.T) {
	var buf bytes.Buffer
	lg := slog.New(slog.NewJSONHandler(&buf, nil))

	app := kruda.New()
	app.Use(Logger(LoggerConfig{Logger: lg}))
	app.Get("/slow", func(c *kruda.Ctx) error {
		time.Sleep(2 * time.Millisecond)
		return c.JSON(kruda.Map{"ok": true})
	})

	resp := newMockResponse()
	app.ServeKruda(resp, getReq("/slow", nil))

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("log line not valid JSON: %v (%s)", err, buf.String())
	}
	latStr, ok := rec["latency"].(string)
	if !ok {
		t.Fatalf("log line has no string latency field: %s", buf.String())
	}
	d, err := time.ParseDuration(latStr)
	if err != nil {
		t.Fatalf("latency %q is not a duration: %v", latStr, err)
	}
	if d < time.Millisecond {
		t.Fatalf("Logger latency=%v — MarkStart() not called, so c.Latency() is always 0 (handler slept 2ms)", d)
	}
}
