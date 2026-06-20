// This file holds Wing transport types and helpers that are platform-neutral
// (pure data + pure logic). Keeping them in one un-tagged file prevents the
// previous "duplicate-and-keep-in-sync" pattern between wing_*.go (linux/
// darwin) and wing_transport_stub.go (!linux && !darwin), where adding a
// field to one definition silently broke cross-platform compilation.

package kruda

import (
	"runtime"
	"time"
)

// WingConfig configures the Wing transport. Values are honored on Linux and
// macOS (where Wing has a real implementation); on other platforms the stub
// transport accepts the same struct but does not act on it.
type WingConfig struct {
	Workers           int
	RingSize          uint32
	ReadBufSize       int
	MaxHeaderCount    int
	MaxHeaderSize     int
	MaxConnsPerWorker int
	HandlerPoolSize   int               // goroutine pool size per worker (Pool dispatch routes)
	Presets           map[string]Preset // per-route preset config ("METHOD /path" → Preset)
	DefaultPreset     Preset            // fallback preset for routes not in Presets
	ReadTimeout              time.Duration // max time to receive a complete request (0 = disabled)
	WriteTimeout             time.Duration // max time to send a response (0 = disabled)
	IdleTimeout              time.Duration // max time a keep-alive conn can be idle (0 = disabled)
	BodyLimit                int           // max request body bytes (0 = disabled). Maps to a 413.
	HeaderLimit              int           // max header bytes (0 = disabled). Maps to a 431.
	TrustProxy               bool          // honor X-Forwarded-For / X-Real-IP
	MaxInflightBodyBytes     int           // per-worker cap on concurrently-accumulating body bytes (0 = derived)
	MaxConns                 int           // resolved absolute total cap (0 = unlimited)
	MaxConnsPerIP            int           // concurrent connections per source IP (0 = off)
	AcceptRatePerSec         int           // new-connection rate limit per second (0 = off)
	AcceptRateBurst          int           // burst allowance for AcceptRatePerSec
}

func (c *WingConfig) defaults() {
	if c.Workers <= 0 {
		c.Workers = runtime.NumCPU()
	}
	if c.RingSize == 0 {
		c.RingSize = 4096
	}
	if c.HeaderLimit < 0 {
		c.HeaderLimit = 0
	}
	if c.ReadBufSize <= 0 {
		c.ReadBufSize = 8192
	}
	// Headers must fit the read buffer to be parseable; grow the buffer to honor HeaderLimit.
	if c.HeaderLimit > c.ReadBufSize {
		c.ReadBufSize = c.HeaderLimit
	}
	if c.MaxHeaderSize == 0 && c.HeaderLimit > 0 {
		c.MaxHeaderSize = c.HeaderLimit
	}
	if c.HandlerPoolSize <= 0 {
		c.HandlerPoolSize = c.Workers
	}
	if c.MaxInflightBodyBytes <= 0 && c.BodyLimit > 0 {
		c.MaxInflightBodyBytes = 64 * c.BodyLimit
	}
}

// DispatchMode controls how Wing schedules a handler when a request arrives.
type DispatchMode uint8

// DispatchMode values.
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

	// Takeover spawns a goroutine that owns the connection.
	// The goroutine loops read→handle→write directly, bypassing ioLoop
	// until the connection goes idle (EAGAIN). Best for DB/Redis I/O
	// where handler latency dominates — eliminates doneCh/wake/re-arm
	// overhead between requests on the same connection.
	Takeover
)

// String renders the DispatchMode for logs/diagnostics.
func (m DispatchMode) String() string {
	switch m {
	case Inline:
		return "Inline"
	case Pool:
		return "Pool"
	case Spawn:
		return "Spawn"
	case Takeover:
		return "Takeover"
	default:
		return "Unknown"
	}
}

type responseMode uint8

const (
	responseGeneric responseMode = iota
	responseJSON
	responseRender // intent/diagnostics tag for the Render preset; does not gate the lane
)

// Preset is the per-route tuning hint passed to Wing. Construct via the
// preset vars (Bolt, Arrow, Spear, …) or directly with options.
type Preset struct {
	Dispatch       DispatchMode
	StaticResponse []byte // pre-built full HTTP response; bypasses handler entirely
	ResponseMode   responseMode
	handlers       []HandlerFunc
	path           string
	pathClean      bool
	explicit       bool // set for preset-table entries; selects the advisor message variant
}

// PresetOption modifies a Preset in-place.
type PresetOption func(*Preset)

// With returns a copy of f with the given options applied.
func (f Preset) With(opts ...PresetOption) Preset {
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// applyRoute implements RouteOption: passing a Preset at route registration
// attaches the per-route composition to the route.
//
//	app.Get("/db", handler, kruda.DB)
func (p Preset) applyRoute(rc *routeConfig) {
	cp := p
	rc.preset = &cp
}

func (f *Preset) defaults() {
	if f.Dispatch == 0 {
		f.Dispatch = Inline
	}
}

// Dispatch returns a PresetOption that sets the dispatch mode.
func Dispatch(m DispatchMode) PresetOption { return func(f *Preset) { f.Dispatch = m } }

// Static returns a PresetOption that sets a pre-built static response,
// allowing Wing to skip the handler entirely on supported platforms.
func Static(resp []byte) PresetOption { return func(f *Preset) { f.StaticResponse = resp } }

// Presets — pick by what the route does. Structural presets (Bolt, Arrow,
// Spear) name the dispatch; semantic presets name the workload.
var (
	// Bolt — inline in ioLoop. Maximum throughput, zero dispatch overhead.
	Bolt = Preset{Dispatch: Inline}

	// Arrow — goroutine pool dispatch.
	Arrow = Preset{Dispatch: Pool}

	// Spear — goroutine owns connection with blocking I/O.
	// Go runtime auto-creates OS threads, avoiding ioLoop starvation.
	Spear = Preset{Dispatch: Takeover}

	// Plaintext — Bolt-aliased preset for static text and health-check routes.
	Plaintext = Bolt
	// JSON — Bolt-aliased preset for JSON-only handlers (no I/O).
	JSON = Bolt
	// DB — Spear-aliased preset for short DB/Redis lookups (named Query in v1.2.x).
	DB = Spear
	// Render — Spear dispatch tagged for DB + template/HTML responses.
	Render = Preset{Dispatch: Takeover, ResponseMode: responseRender}
)

// RawRequest provides low-level access to Wing's request data.
// Obtain via transport.Request.RawRequest():
//
//	if raw, ok := req.RawRequest().(kruda.RawRequest); ok {
//	    fd := raw.Fd()
//	}
//
// On platforms without Wing support the interface is still declared so that
// user code referencing it compiles; the underlying request type only
// implements it on Linux and macOS.
type RawRequest interface {
	RawMethod() string
	RawPath() []byte
	RawHeader(name string) []byte
	RawBody() []byte
	Fd() int32
	KeepAlive() bool
}
