//go:build linux || darwin

package wing

// Feather is the per-route tuning system for Wing.
// Only axes that are actually read by the transport are kept.
type Feather struct {
	Dispatch       DispatchMode
	StaticResponse []byte // pre-built full HTTP response; bypasses handler entirely
}

// FeatherOption modifies a Feather.
type FeatherOption func(*Feather)

// With returns a copy of f with the given options applied.
func (f Feather) With(opts ...FeatherOption) Feather {
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// defaults fills zero-valued axes with Inline defaults.
func (f *Feather) defaults() {
	if f.Dispatch == 0 {
		f.Dispatch = Inline
	}
}

// --------------- Dispatch (how the handler is scheduled) ---------------

// DispatchMode controls how the handler goroutine is scheduled.
type DispatchMode uint8

const (
	// Inline runs the handler directly in the ioLoop goroutine.
	// Zero overhead, but blocks the event loop during execution.
	// Use for handlers with no I/O wait (plaintext, JSON, cached).
	Inline DispatchMode = iota + 1

	// Pool dispatches the handler to a bounded goroutine pool.
	// ~1μs overhead. Use for short I/O (DB query, Redis).
	Pool

	// Spawn creates a new goroutine per request.
	// ~2-3μs overhead. Use for heavy compute or variable latency.
	Spawn
)

// --------------- Option constructors ---------------

// Dispatch returns a FeatherOption that sets the dispatch mode.
func Dispatch(m DispatchMode) FeatherOption { return func(f *Feather) { f.Dispatch = m } }

// Static returns a FeatherOption that sets a pre-built static response.
func Static(resp []byte) FeatherOption { return func(f *Feather) { f.StaticResponse = resp } }

// --------------- Named Presets ---------------

var (
	// Bolt — maximum throughput for in-memory responses.
	// Inline dispatch, HTTP pipelining.
	Bolt = Feather{Dispatch: Inline}

	// Arrow — the default preset for most routes.
	// Pool dispatch for DB/Redis/external I/O.
	Arrow = Feather{Dispatch: Pool}

	// ---- Aliases ----
	Plaintext = Bolt
	JSON      = Bolt
	ParamJSON = Bolt
	PostJSON  = Bolt
	Query     = Arrow
	Render    = Arrow
)

// --------------- Stringer ---------------

func (m DispatchMode) String() string {
	switch m {
	case Inline:
		return "Inline"
	case Pool:
		return "Pool"
	case Spawn:
		return "Spawn"
	default:
		return "Unknown"
	}
}
