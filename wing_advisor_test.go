package kruda

import (
	"bytes"
	"log/slog"
	"strconv"
	"strings"
	"testing"
)

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestAdvisorWarnsOnceAfterThreshold(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	for i := 0; i < advisorWarnAfter-1; i++ {
		advisorObserve("GET", "/db", 1_200_000, false)
	}
	if buf.Len() != 0 {
		t.Fatalf("warned before threshold: %s", buf.String())
	}
	advisorObserve("GET", "/db", 1_200_000, false)
	out := buf.String()
	if !strings.Contains(out, "GET /db") || !strings.Contains(out, "kruda.DB") {
		t.Fatalf("missing route/suggestion in warning: %s", out)
	}
	for i := 0; i < 100; i++ {
		advisorObserve("GET", "/db", 1_200_000, false)
	}
	if n := strings.Count(buf.String(), "blocked the event loop"); n != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %s", n, buf.String())
	}
}

func TestAdvisorExplicitPresetVariant(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)
	for i := 0; i < advisorWarnAfter; i++ {
		advisorObserve("GET", "/annotated", 500_000, true)
	}
	if !strings.Contains(buf.String(), "annotated for inline dispatch") {
		t.Fatalf("expected explicit-preset variant, got: %s", buf.String())
	}
}

func TestAdvisorRouteCap(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)
	for i := 0; i < advisorMaxRoutes; i++ {
		advisorObserve("GET", "/r"+strconv.Itoa(i), 200_000, false)
	}
	// past the cap: new routes are dropped — no panic, no warning explosion
	for i := 0; i < advisorWarnAfter; i++ {
		advisorObserve("GET", "/overflow", 200_000, false)
	}
	if strings.Contains(buf.String(), "/overflow") {
		t.Fatalf("route past cap should not be tracked: %s", buf.String())
	}
}
