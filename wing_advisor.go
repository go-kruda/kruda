package kruda

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Blocking advisor: Wing observes inline-dispatched handlers and warns —
// once per route per process — when one repeatedly blocks the event loop.
// It never switches dispatch modes; evidence shows misclassification costs
// -63% to -97% throughput or stalls the loop, so the human decides.
//
// The advisor judges a route by the *share* of its requests that run long, not
// by how many have. Inline time is wall time, so it also counts any delay the
// OS scheduler adds between entering and leaving the handler. When the machine
// is CPU-saturated — a load test, a noisy neighbour, a pod at its CPU limit —
// even a handler that only writes a constant string is occasionally descheduled
// for longer than advisorBlockNanos. Warning on an absolute count made that a
// false positive: at a few hundred thousand requests a second, a handful of
// slow samples out of millions reached the threshold within seconds and told
// the user to annotate a route that never blocks on anything.
//
// A route that genuinely blocks — a synchronous DB call, a file read — blocks
// on nearly every request, while scheduler noise touches a very small fraction.
// Requiring advisorBlockPercent of at least advisorMinSamples requests to run
// long separates the two by orders of magnitude.
const (
	advisorBlockNanos   = 100_000 // inline wall time that counts as a block (100µs)
	advisorWarnAfter    = 10      // minimum long requests on a route before warning
	advisorMinSamples   = 200     // minimum requests observed before a share is meaningful
	advisorBlockPercent = 20      // minimum share of long requests, in percent, to warn
	advisorMaxRoutes    = 1024    // stop tracking new routes beyond this (param-route flood guard)
)

type advisorEntry struct {
	total   atomic.Int64
	blocked atomic.Int64
	warned  atomic.Bool
}

var (
	advisorRoutes sync.Map // "METHOD path" → *advisorEntry
	advisorSize   atomic.Int64
)

func advisorResetForTest() {
	advisorRoutes = sync.Map{}
	advisorSize.Store(0)
}

// advisorLookup returns the entry for a route, allocating one on first sight and
// respecting the route cap. It returns nil once the cap is reached and the route
// is not already tracked.
func advisorLookup(key string) *advisorEntry {
	if v, ok := advisorRoutes.Load(key); ok {
		return v.(*advisorEntry)
	}
	if advisorSize.Load() >= advisorMaxRoutes {
		return nil
	}
	v, loaded := advisorRoutes.LoadOrStore(key, &advisorEntry{})
	if !loaded {
		advisorSize.Add(1)
	}
	return v.(*advisorEntry)
}

// advisorObserve records one inline request and its wall time. It is called for
// every inline request, not only slow ones, because the share of slow requests
// is what distinguishes a blocking handler from scheduler noise.
//
// The caller passes an entry resolved through the worker's route cache, so the
// common case of consecutive requests to one route costs a string comparison
// and two atomic adds rather than a map lookup.
func advisorObserve(e *advisorEntry, key string, elapsedNanos int64, explicitPreset bool) {
	if e == nil {
		return
	}
	total := e.total.Add(1)
	if elapsedNanos < advisorBlockNanos {
		return
	}
	blocked := e.blocked.Add(1)

	if blocked < advisorWarnAfter || total < advisorMinSamples {
		return
	}
	if blocked*100 < total*advisorBlockPercent {
		return
	}
	if !e.warned.CompareAndSwap(false, true) {
		return
	}

	share := blocked * 100 / total
	blockedFor := time.Duration(elapsedNanos).Round(10 * time.Microsecond)
	if explicitPreset {
		slog.Warn("kruda: route is annotated for inline dispatch but blocked the event loop — verify the preset, or use kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
			"route", key, "blocked", blockedFor.String(), "count", blocked, "share_percent", share)
		return
	}
	slog.Warn("kruda: route blocked the event loop — add kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
		"route", key, "blocked", blockedFor.String(), "count", blocked, "share_percent", share)
}
