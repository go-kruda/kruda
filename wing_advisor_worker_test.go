//go:build linux || darwin

// The worker-level advisor tests live here because worker is defined only for
// the platforms Wing supports; wing_advisor.go itself is platform-neutral.

package kruda

import (
	"strconv"
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

	e := w.advisor.last
	if e == nil || e.path != "/original" {
		t.Fatalf("cached path = %q, want %q", e.path, "/original")
	}

	// Simulate the buffer being reused for the next request.
	copy(pathBuf, []byte("/OVERWRIT"))

	if e.path != "/original" {
		t.Fatalf("cached path aliased the request buffer: became %q after the buffer was reused", e.path)
	}
	if e.key != "GET /original" {
		t.Fatalf("cached key aliased the request buffer: became %q", e.key)
	}
	// The map key must own its strings too, or the lookup would go stale.
	if _, ok := w.advisor.routes[advisorRouteKey{"GET", "/original"}]; !ok {
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

// TestObserveAdvisorNoWarnFromEarlyScatteredBlocks guards the batching rule.
// Long requests must reach the shared entry alongside the totals they came
// from. An earlier version folded each long request in on its own while the
// totals stayed in per-worker tallies, so a handful of workers a few requests
// into their first batch could publish enough blocked samples against an almost
// empty shared total to cross the share threshold — a false positive worst at
// process start, which is exactly when scheduler noise peaks and the advisor is
// supposed to stay quiet.
func TestObserveAdvisorNoWarnFromEarlyScatteredBlocks(t *testing.T) {
	advisorResetForTest()
	buf := captureWarnings(t)

	// 64 workers, each a few requests into a route it will serve heavily, each
	// having caught a single descheduled request early on.
	workers := make([]*worker, 64)
	for i := range workers {
		workers[i] = &worker{}
		workers[i].observeAdvisor(Preset{}, "GET", "/plaintext", slowNanos)
		for j := 0; j < 4; j++ {
			workers[i].observeAdvisor(Preset{}, "GET", "/plaintext", fastNanos)
		}
	}
	if out := buf.String(); out != "" {
		t.Fatalf("warned on 64 scattered early samples (true share 20%% of 320 requests, well under advisorMinSamples): %s", out)
	}

	// Let them run: 1% descheduled, the rate the advisor must tolerate.
	for _, w := range workers {
		for i := 0; i < 2000; i++ {
			elapsed := int64(fastNanos)
			if i%100 == 0 {
				elapsed = slowNanos
			}
			w.observeAdvisor(Preset{}, "GET", "/plaintext", elapsed)
		}
		w.advisor.flush()
	}
	if out := buf.String(); out != "" {
		t.Fatalf("warned at a ~1%% block rate across 64 workers: %s", out)
	}

	e, _ := advisorRoutes.Load("GET /plaintext")
	total := e.(*advisorEntry).total.Load()
	blocked := e.(*advisorEntry).blocked.Load()
	if want := int64(64 * 2005); total != want {
		t.Errorf("shared total = %d, want %d — every observed request must be folded in", total, want)
	}
	if share := blocked * 100 / total; share >= advisorBlockPercent {
		t.Errorf("shared share = %d%%, want well under %d%%", share, advisorBlockPercent)
	}
}

// TestObserveAdvisorRouteFloodStaysBounded guards the param-route flood cap.
// The cap is process-wide, so a worker must never track a route the shared map
// already refused — an earlier version capped each worker's map independently
// and stored an entry-less slot when the shared lookup returned nothing, giving
// every worker its own 1024 routes and a fresh key allocation for every flooded
// path. One worker cannot show this: its own map fills at the same moment the
// process-wide one does, so the second worker is the one that matters.
func TestObserveAdvisorRouteFloodStaysBounded(t *testing.T) {
	advisorResetForTest()
	_ = captureWarnings(t)

	first := &worker{}
	for i := 0; i < advisorMaxRoutes*2; i++ {
		first.observeAdvisor(Preset{}, "GET", "/users/"+strconv.Itoa(i), fastNanos)
	}
	if n := advisorSize.Load(); n != advisorMaxRoutes {
		t.Fatalf("process tracked %d routes, want the cap %d", n, advisorMaxRoutes)
	}

	// A second worker meeting the flood with an empty map of its own.
	second := &worker{}
	for i := advisorMaxRoutes * 10; i < advisorMaxRoutes*10+500; i++ {
		second.observeAdvisor(Preset{}, "GET", "/users/"+strconv.Itoa(i), fastNanos)
	}
	if n := len(second.advisor.routes); n != 0 {
		t.Errorf("second worker tracked %d routes the process-wide cap had already refused, want 0", n)
	}
	for _, w := range []*worker{first, second} {
		if n := len(w.advisor.routes); n > advisorMaxRoutes {
			t.Errorf("worker tracked %d routes, want at most %d", n, advisorMaxRoutes)
		}
		for k := range w.advisor.routes {
			if _, ok := advisorRoutes.Load(k.method + " " + k.path); !ok {
				t.Fatalf("worker tracks %q %q, which the process-wide cap refused", k.method, k.path)
			}
		}
	}

	// Past the cap a flooded path must not allocate: a map miss, an atomic load,
	// and a return. The test's own strconv.Itoa and concatenation set the floor.
	i := advisorMaxRoutes * 20
	allocs := testing.AllocsPerRun(200, func() {
		second.observeAdvisor(Preset{}, "GET", "/users/"+strconv.Itoa(i), fastNanos)
		i++
	})
	base := testing.AllocsPerRun(200, func() {
		sink = "/users/" + strconv.Itoa(i)
		i++
	})
	if allocs > base {
		t.Errorf("observing a flooded path allocated %.2f/op over a %.2f/op baseline", allocs, base)
	}
}

var sink string
