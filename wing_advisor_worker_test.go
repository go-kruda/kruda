//go:build linux || darwin

// The worker-level advisor tests live here because worker is defined only for
// the platforms Wing supports; wing_advisor.go itself is platform-neutral.

package kruda

import (
	"strings"
	"testing"

	"github.com/go-kruda/kruda/internal/bytesconv"
)

// wing_advisor_test.go drives advisorFlush directly. These drive the worker
// method that feeds it, which is where the route key and the tally cache are
// decided — an earlier version keyed on Preset.path and so silently ignored
// every route registered without an explicit preset. These drive the worker method
// that feeds it, which is where the route key and the entry cache are decided —
// an earlier version of this code keyed on Preset.path and so silently ignored
// every route registered without an explicit preset.

func TestObserveAdvisorTracksRoutesWithoutAPreset(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	// Preset zero value: what a plain app.Get route gets from the table default.
	w := &worker{}
	for i := 0; i < advisorMinSamples; i++ {
		w.observeAdvisor(Preset{}, "GET", "/blocking", slowNanos)
	}

	if !strings.Contains(buf.String(), "GET /blocking") {
		t.Fatalf("route with an empty Preset.path was not tracked: %q", buf.String())
	}
}

func TestObserveAdvisorSwitchesBetweenRoutes(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	w := &worker{}
	// Alternate two routes so every request is a cache miss, and make only one
	// of them slow. The fast route must not inherit the slow one's samples.
	for i := 0; i < advisorMinSamples*2; i++ {
		w.observeAdvisor(Preset{}, "GET", "/slow", slowNanos)
		w.observeAdvisor(Preset{}, "GET", "/fast", fastNanos)
	}

	out := buf.String()
	if !strings.Contains(out, "GET /slow") {
		t.Fatalf("slow route did not warn: %q", out)
	}
	if strings.Contains(out, "GET /fast") {
		t.Fatalf("fast route warned: %q", out)
	}

	// Fast requests are tallied worker-locally and folded in every
	// advisorFlushEvery, so the shared totals only settle after a flush.
	w.advisor.flush()

	slow, _ := advisorRoutes.Load("GET /slow")
	fast, _ := advisorRoutes.Load("GET /fast")
	if slow == nil || fast == nil {
		t.Fatal("expected both routes tracked separately")
	}
	if n := fast.(*advisorEntry).blocked.Load(); n != 0 {
		t.Errorf("fast route recorded %d blocked requests, want 0", n)
	}
	if n := fast.(*advisorEntry).total.Load(); n != int64(advisorMinSamples*2) {
		t.Errorf("fast route recorded %d requests, want %d", n, advisorMinSamples*2)
	}
}

// TestObserveAdvisorDoesNotRetainRequestPath guards the read-buffer aliasing
// rule: the request path points into the connection's read buffer, which is
// overwritten by the next request. If the worker cached that string rather than
// a copy, a later request would compare against mutated bytes and could either
// miss the cache forever or attribute requests to the wrong route.
func TestObserveAdvisorDoesNotRetainRequestPath(t *testing.T) {
	advisorResetForTest()
	_ = captureWarnings(t)

	pathBuf := []byte("/original")
	w := &worker{}
	w.observeAdvisor(Preset{}, "GET", bytesconv.UnsafeString(pathBuf), fastNanos)

	slot := w.advisor.last
	if slot == nil || slot.path != "/original" {
		t.Fatalf("cached path = %q, want %q", slot.path, "/original")
	}

	// Simulate the buffer being reused for the next request.
	copy(pathBuf, []byte("/OVERWRIT"))

	if slot.path != "/original" {
		t.Fatalf("cached path aliased the request buffer: became %q after the buffer was reused", slot.path)
	}
	if slot.key != "GET /original" {
		t.Fatalf("cached key aliased the request buffer: became %q", slot.key)
	}
	// The map key must own its strings too, or the lookup would go stale.
	if _, ok := w.advisor.slots[advisorRouteKey{"GET", "/original"}]; !ok {
		t.Fatal("slot map key aliased the request buffer")
	}
}

// TestObserveAdvisorZeroAllocInterleavedRoutes guards the inline hot path.
// Wing observes every inline request to measure the share that run long, so the
// observation itself must be free: an earlier version resolved the route through
// a one-slot cache backed by a sync.Map, which allocated a fresh key string on
// every request as soon as traffic alternated between routes — 12 B and ~35 ns
// per request on any app serving more than one path.
//
// TestStringLaneZeroAlloc covers the response builder, not dispatch, so it does
// not catch this.
func TestObserveAdvisorZeroAllocInterleavedRoutes(t *testing.T) {
	advisorResetForTest()
	w := &worker{}
	paths := []string{"/plaintext", "/json", "/db", "/fortunes"}

	// Prime: the first sight of a route allocates its slot, once per worker.
	for _, p := range paths {
		w.observeAdvisor(Preset{}, "GET", p, fastNanos)
	}

	i := 0
	allocs := testing.AllocsPerRun(1000, func() {
		w.observeAdvisor(Preset{}, "GET", paths[i&3], fastNanos)
		i++
	})
	if allocs != 0 {
		t.Fatalf("observing inline requests across %d routes must be zero-alloc, got %.2f allocs/op", len(paths), allocs)
	}
}
