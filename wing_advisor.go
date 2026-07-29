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
// Measuring a share means observing every inline request, not only the long
// ones. That count must therefore cost nothing on the hot path: workers tally
// their own requests in advisorCache, which is owned by one event-loop
// goroutine and uses neither atomics nor allocation, and fold those tallies
// into the shared per-route entry only on a long request or once every
// advisorFlushEvery requests.
const (
	advisorBlockNanos   = 100_000 // inline wall time that counts as a block (100µs)
	advisorWarnAfter    = 10      // minimum long requests on a route before warning
	advisorMinSamples   = 200     // minimum requests observed before a share is meaningful
	advisorBlockPercent = 20      // minimum share of long requests, in percent, to warn
	advisorMaxRoutes    = 1024    // stop tracking new routes beyond this (param-route flood guard)
	advisorFlushEvery   = 256     // requests a worker tallies locally before folding them in
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

// advisorFlush folds one worker's tallies for a route into the shared entry and
// warns if the route now looks like it blocks the loop. lastBlockedNanos is the
// wall time of the most recent long request and is only read when blockedDelta
// is non-zero.
func advisorFlush(e *advisorEntry, key string, totalDelta, blockedDelta, lastBlockedNanos int64, explicitPreset bool) {
	if e == nil || (totalDelta == 0 && blockedDelta == 0) {
		return
	}
	// Add(0) is a no-op that still reads the running total, so a flush carrying
	// only blocked samples is not silently dropped.
	total := e.total.Add(totalDelta)
	if blockedDelta == 0 {
		return
	}
	blocked := e.blocked.Add(blockedDelta)

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
	blockedFor := time.Duration(lastBlockedNanos).Round(10 * time.Microsecond)
	if explicitPreset {
		slog.Warn("kruda: route is annotated for inline dispatch but blocked the event loop — verify the preset, or use kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
			"route", key, "blocked", blockedFor.String(), "count", blocked, "share_percent", share)
		return
	}
	slog.Warn("kruda: route blocked the event loop — add kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
		"route", key, "blocked", blockedFor.String(), "count", blocked, "share_percent", share)
}

type advisorRouteKey struct{ method, path string }

// advisorSlot is a resolved route: its shared entry plus the strings needed to
// report it. key, method and path are owned copies, because the request path
// handed to observe aliases the connection read buffer and goes stale.
type advisorSlot struct {
	key      string
	method   string
	path     string
	entry    *advisorEntry // nil once the process-wide route cap is reached
	explicit bool
}

// advisorCache holds one worker's advisor tallies. It is owned by that worker's
// event-loop goroutine: no atomics, no locking, and no allocation after a route
// has been seen once.
//
// The tallies live here rather than in advisorSlot so that observing a request
// touches only fields inline in the worker, with no pointer to chase. They
// belong to last, and are folded into its shared entry whenever the route
// changes, a request runs long, or advisorFlushEvery requests accumulate.
type advisorCache struct {
	lastMethod  string
	lastPath    string
	total       int64
	blocked     int64
	lastBlocked int64
	last        *advisorSlot
	slots       map[advisorRouteKey]*advisorSlot
}

// observe records one inline request and its wall time. Consecutive requests to
// the same route — the common case, since Wing serves a connection's pipeline
// in order — cost two string comparisons and an increment.
func (a *advisorCache) observe(method, path string, elapsedNanos int64, explicitPreset bool) {
	if a.lastPath != path || a.lastMethod != method {
		a.switchRoute(method, path, explicitPreset)
	}
	if a.last == nil {
		return
	}
	a.total++
	if elapsedNanos >= advisorBlockNanos {
		a.blocked++
		a.lastBlocked = elapsedNanos
		a.flush()
		return
	}
	if a.total >= advisorFlushEvery {
		a.flush()
	}
}

// switchRoute settles the outgoing route's tallies before pointing the cache at
// the incoming one, so a worker alternating between routes loses no counts.
func (a *advisorCache) switchRoute(method, path string, explicitPreset bool) {
	a.flush()
	a.last = a.slotFor(method, path, explicitPreset)
	if a.last == nil {
		// No owned copy of path to cache, and retaining the caller's would alias
		// the read buffer. Leave the cache empty; past the route cap every
		// request is a fresh path anyway.
		a.lastMethod, a.lastPath = "", ""
		return
	}
	a.lastMethod, a.lastPath = a.last.method, a.last.path
}

func (a *advisorCache) slotFor(method, path string, explicitPreset bool) *advisorSlot {
	if s, ok := a.slots[advisorRouteKey{method, path}]; ok {
		return s
	}
	if len(a.slots) >= advisorMaxRoutes {
		return nil
	}
	// One allocation per route per worker, never per request. method and path
	// are re-sliced out of key so the slot owns every string it stores.
	key := method + " " + path
	s := &advisorSlot{
		key:      key,
		method:   key[:len(method)],
		path:     key[len(method)+1:],
		entry:    advisorLookup(key),
		explicit: explicitPreset,
	}
	if a.slots == nil {
		a.slots = make(map[advisorRouteKey]*advisorSlot)
	}
	a.slots[advisorRouteKey{s.method, s.path}] = s
	return s
}

// flush folds the pending tallies into the current route's shared entry. Every
// other route was already flushed when the cache switched away from it, so
// calling this as a worker stops settles everything it observed.
//
// The guard tests both counters rather than relying on them being reset
// together, so it stays correct if a future caller flushes them separately.
func (a *advisorCache) flush() {
	if a.last == nil || (a.total == 0 && a.blocked == 0) {
		return
	}
	advisorFlush(a.last.entry, a.last.key, a.total, a.blocked, a.lastBlocked, a.last.explicit)
	a.total, a.blocked = 0, 0
}
