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

// advisorObserve folds one request straight into a shared entry, bypassing the
// per-worker tally, so the threshold logic in advisorFlush can be driven a
// sample at a time.
func advisorObserve(e *advisorEntry, key string, elapsedNanos int64, explicitPreset bool) {
	if elapsedNanos < advisorBlockNanos {
		advisorFlush(e, key, 1, 0, 0, explicitPreset)
		return
	}
	advisorFlush(e, key, 1, 1, elapsedNanos, explicitPreset)
}

// observe feeds n requests for one route, each taking elapsed nanoseconds.
func observe(key string, n int, elapsed int64, explicit bool) {
	e := advisorLookup(key)
	for i := 0; i < n; i++ {
		advisorObserve(e, key, elapsed, explicit)
	}
}

const (
	fastNanos = advisorBlockNanos - 1 // a request that does not count as blocked
	slowNanos = 1_200_000             // 1.2ms — comfortably blocked
)

func TestAdvisorWarnsOnceWhenMostRequestsBlock(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	// Below the sample floor there is no meaningful share yet, however many
	// requests blocked.
	observe("GET /db", advisorMinSamples-1, slowNanos, false)
	if buf.Len() != 0 {
		t.Fatalf("warned before the sample floor: %s", buf.String())
	}

	observe("GET /db", 1, slowNanos, false)
	out := buf.String()
	if !strings.Contains(out, "GET /db") || !strings.Contains(out, "kruda.DB") {
		t.Fatalf("missing route/suggestion in warning: %s", out)
	}

	observe("GET /db", 1000, slowNanos, false)
	if n := strings.Count(buf.String(), "blocked the event loop"); n != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %s", n, buf.String())
	}
}

// TestAdvisorIgnoresRareSlowRequests is the regression test for the false
// positive this advisor produced under CPU saturation. Inline time is wall
// time, so a handler that never blocks still records occasional long requests
// when the OS deschedules the thread. Warning on an absolute count meant a
// plain c.Text handler was told to add kruda.DB after ten such samples, which a
// load test reached within seconds. A rare slow request must not warn no matter
// how many requests the route has served.
func TestAdvisorIgnoresRareSlowRequests(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	e := advisorLookup("GET /plaintext")
	// 1 in 1000 requests runs long: far more slow samples than the old
	// absolute threshold of 10, but nowhere near a blocking handler's share.
	for i := 0; i < 200_000; i++ {
		elapsed := int64(fastNanos)
		if i%1000 == 0 {
			elapsed = slowNanos
		}
		advisorObserve(e, "GET /plaintext", elapsed, false)
	}

	if buf.Len() != 0 {
		t.Fatalf("warned on scheduler noise (%d slow of 200000): %s",
			e.blocked.Load(), buf.String())
	}
	if got := e.blocked.Load(); got < advisorWarnAfter {
		t.Fatalf("test is not exercising the count threshold: only %d slow samples", got)
	}
}

// TestAdvisorShareThreshold checks both sides of the share threshold with an
// evenly spaced pattern. The advisor evaluates the share incrementally, as each
// request arrives, so a route whose slow requests arrive in bursts can cross the
// threshold on a running total even when its overall share sits below it —
// intended, since bursty blocking is still blocking. These cases therefore sit
// clear of the boundary rather than exactly on it.
func TestAdvisorShareThreshold(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	// 10% of requests slow, evenly spaced: below the threshold, no warning.
	e := advisorLookup("GET /under")
	for i := 0; i < 2000; i++ {
		elapsed := int64(fastNanos)
		if i%10 == 0 {
			elapsed = slowNanos
		}
		advisorObserve(e, "GET /under", elapsed, false)
	}
	if buf.Len() != 0 {
		t.Fatalf("warned at a 10%% share, below the %d%% threshold: %s",
			advisorBlockPercent, buf.String())
	}

	// 25% of requests slow, evenly spaced: above the threshold, warns.
	e2 := advisorLookup("GET /over")
	for i := 0; i < 2000; i++ {
		elapsed := int64(fastNanos)
		if i%4 == 0 {
			elapsed = slowNanos
		}
		advisorObserve(e2, "GET /over", elapsed, false)
	}
	if !strings.Contains(buf.String(), "GET /over") {
		t.Fatalf("expected a warning at a 25%% share, got: %s", buf.String())
	}
}

func TestAdvisorReportsShareInWarning(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)
	observe("GET /db", 500, slowNanos, false)
	if out := buf.String(); !strings.Contains(out, "share_percent=100") {
		t.Fatalf("expected share_percent in warning, got: %s", out)
	}
}

func TestAdvisorExplicitPresetVariant(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)
	observe("GET /annotated", advisorMinSamples, slowNanos, true)
	if !strings.Contains(buf.String(), "annotated for inline dispatch") {
		t.Fatalf("expected explicit-preset variant, got: %s", buf.String())
	}
}

func TestAdvisorRouteCap(t *testing.T) {
	advisorResetForTest()
	for i := 0; i < advisorMaxRoutes; i++ {
		if e := advisorLookup("GET /r" + strconv.Itoa(i)); e == nil {
			t.Fatalf("route %d rejected before the cap", i)
		}
	}
	// Past the cap: new routes are dropped — no panic, no warning explosion.
	buf := captureWarnings(t)
	if e := advisorLookup("GET /overflow"); e != nil {
		t.Fatal("route past the cap should not be tracked")
	}
	observe("GET /overflow", advisorMinSamples*2, slowNanos, false)
	if strings.Contains(buf.String(), "/overflow") {
		t.Fatalf("route past cap should not warn: %s", buf.String())
	}
}

// TestAdvisorObserveNilEntryIsSafe covers the capped-route path, where the
// worker caches a nil entry and keeps serving requests.
func TestAdvisorObserveNilEntryIsSafe(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)
	advisorObserve(nil, "GET /nil", slowNanos, false)
	if buf.Len() != 0 {
		t.Fatalf("nil entry produced output: %s", buf.String())
	}
}
