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
const (
	advisorBlockNanos = 100_000 // inline wall time that counts as a block (100µs)
	advisorWarnAfter  = 10      // warn on the Nth blocked request per route
	advisorMaxRoutes  = 1024    // stop tracking new routes beyond this (param-route flood guard)
)

type advisorEntry struct {
	count  atomic.Int64
	warned atomic.Bool
}

var (
	advisorRoutes sync.Map // "METHOD path" → *advisorEntry
	advisorSize   atomic.Int64
)

func advisorResetForTest() {
	advisorRoutes = sync.Map{}
	advisorSize.Store(0)
}

// advisorObserve records one blocked inline request. Callers only invoke it
// when elapsed >= advisorBlockNanos, so the hot path pays nothing beyond two
// clock reads per request.
func advisorObserve(method, path string, elapsedNanos int64, explicitPreset bool) {
	key := method + " " + path
	v, ok := advisorRoutes.Load(key)
	if !ok {
		if advisorSize.Load() >= advisorMaxRoutes {
			return
		}
		var loaded bool
		v, loaded = advisorRoutes.LoadOrStore(key, &advisorEntry{})
		if !loaded {
			advisorSize.Add(1)
		}
	}
	e := v.(*advisorEntry)
	if e.count.Add(1) != advisorWarnAfter || !e.warned.CompareAndSwap(false, true) {
		return
	}
	blocked := time.Duration(elapsedNanos).Round(10 * time.Microsecond)
	if explicitPreset {
		slog.Warn("kruda: route is annotated for inline dispatch but blocked the event loop — verify the preset, or use kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
			"route", key, "blocked", blocked.String(), "count", advisorWarnAfter)
		return
	}
	slog.Warn("kruda: route blocked the event loop — add kruda.DB (short DB/Redis I/O) or kruda.Spear (blocking I/O)",
		"route", key, "blocked", blocked.String(), "count", advisorWarnAfter)
}
