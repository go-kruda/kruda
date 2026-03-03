package wing

// Feather (ขนนก) is the per-route tuning system for Wing.
// Each axis controls one aspect of request processing.
// Zero values are filled with Inline defaults (fast-by-default).
// Use named presets like Arrow for Pool dispatch.
type Feather struct {
	Dispatch       DispatchMode
	Engine         EngineMode
	Response       ResponseMode
	Buffer         BufferMode
	Conn           ConnMode
	StaticResponse []byte // pre-built full HTTP response; bypasses handler entirely
}

// FeatherOption modifies a single axis of a Feather.
type FeatherOption func(*Feather)

// With returns a copy of f with the given options applied.
// Use this to override specific axes from a preset:
//
//	wing.Arrow.With(wing.Buffer(wing.Grow))
func (f Feather) With(opts ...FeatherOption) Feather {
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// defaults fills zero-valued axes with Inline defaults (fast-by-default).
// Inline+Epoll+DirectWrite+Fixed+KeepAlive — no goroutine pool overhead.
func (f *Feather) defaults() {
	if f.Dispatch == 0 {
		f.Dispatch = Inline
	}
	if f.Engine == 0 {
		f.Engine = Epoll
	}
	if f.Response == 0 {
		f.Response = DirectWrite
	}
	if f.Buffer == 0 {
		f.Buffer = Fixed
	}
	if f.Conn == 0 {
		f.Conn = KeepAlive
	}
}

// --------------- Axis 1: Dispatch (วิธีรัน handler) ---------------

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

	// Persist keeps a goroutine alive for the connection lifetime.
	// Use for SSE, WebSocket, long-poll.
	Persist
)

// --------------- Axis 2: Engine (kernel I/O interface) ---------------

// EngineMode selects the kernel I/O mechanism.
type EngineMode uint8

const (
	// Epoll uses epoll (Linux) or kqueue (macOS) with direct syscalls.
	// Best for short HTTP request/response cycles.
	Epoll EngineMode = iota + 1

	// IOURing uses io_uring submission/completion queues.
	// Best for file I/O, sendfile, large bodies, 100K+ connections.
	IOURing

	// Splice uses splice(2) for zero-copy fd-to-fd transfer.
	// Best for reverse proxy and socket piping.
	Splice

	// Net uses Go's standard net/http (netpoller).
	// Required for HTTP/2, TLS termination, gRPC, Windows.
	Net
)

// --------------- Axis 3: Response (วิธีส่ง response) ---------------

// ResponseMode controls how the response is written back to the client.
type ResponseMode uint8

const (
	// Direct writes the response immediately from the same goroutine.
	// Use with Inline dispatch where handler runs in the ioLoop.
	Direct ResponseMode = iota + 1

	// Writeback sends the response through a channel back to the ioLoop.
	// Safe ordering for Pool/Spawn dispatch. Default async path.
	Writeback

	// DirectWrite lets the handler goroutine call write() directly,
	// bypassing the ioLoop pipe. Saves ~1.5μs per request.
	// Use with Pool dispatch for maximum throughput.
	DirectWrite

	// Batch coalesces multiple pipelined responses into a single writev().
	// Use for TFB plaintext with pipeline depth 16.
	Batch

	// Chunked uses Transfer-Encoding: chunked, flushing per chunk.
	// Use for SSE and streaming responses.
	Chunked

	// Sendfile uses sendfile(2) or io_uring splice for zero-copy file serving.
	Sendfile
)

// --------------- Axis 4: Buffer (วิธีจัดการ memory) ---------------

// BufferMode controls response buffer allocation strategy.
type BufferMode uint8

const (
	// Fixed uses a pre-allocated fixed-size buffer from sync.Pool.
	// Best for plaintext, JSON, and short responses.
	Fixed BufferMode = iota + 1

	// Grow starts small and grows as the response body increases.
	// Best for template rendering and variable-size responses.
	Grow

	// ZeroCopy detaches the buffer directly without copying.
	// Only safe with Inline dispatch (same goroutine, no race).
	ZeroCopy

	// Stream writes chunks incrementally without buffering the full response.
	// Best for large responses, SSE, and file streaming.
	Stream

	// Registered uses io_uring registered buffers (pinned memory).
	// Avoids page faults per I/O. Requires IOURing engine.
	Registered
)

// --------------- Axis 5: Connection (lifecycle) ---------------

// ConnMode controls connection lifecycle behavior.
type ConnMode uint8

const (
	// Pipeline allows parsing the next request before the current response
	// is fully sent. Enables HTTP/1.1 pipelining for maximum throughput.
	Pipeline ConnMode = iota + 1

	// KeepAlive processes one request at a time, then waits for the next.
	// Standard behavior for browsers and API clients.
	KeepAlive

	// OneShot closes the connection after a single response.
	// Use for webhooks, health probes, and one-off calls.
	OneShot

	// Upgrade switches protocols via 101 Switching Protocols.
	// Use for WebSocket and HTTP/2 upgrade.
	Upgrade
)

// --------------- Option constructors ---------------

// Dispatch returns a FeatherOption that sets the dispatch mode.
func Dispatch(m DispatchMode) FeatherOption { return func(f *Feather) { f.Dispatch = m } }

// Engine returns a FeatherOption that sets the engine mode.
func Engine(m EngineMode) FeatherOption { return func(f *Feather) { f.Engine = m } }

// Response returns a FeatherOption that sets the response mode.
func Response(m ResponseMode) FeatherOption { return func(f *Feather) { f.Response = m } }

// Buffer returns a FeatherOption that sets the buffer mode.
func Buffer(m BufferMode) FeatherOption { return func(f *Feather) { f.Buffer = m } }

// Conn returns a FeatherOption that sets the connection mode.
func Conn(m ConnMode) FeatherOption { return func(f *Feather) { f.Conn = m } }

// Static returns a FeatherOption that sets a pre-built static response.
// When set, the handler is bypassed entirely — the response bytes are written directly.
func Static(resp []byte) FeatherOption { return func(f *Feather) { f.StaticResponse = resp } }

// --------------- Named Presets (ชุดสำเร็จรูป) ---------------

var (
	// Bolt ⚡ — maximum throughput for in-memory responses.
	// Inline dispatch, zero-copy buffer, HTTP pipelining.
	// Use for: TFB plaintext/JSON, health checks, cached responses.
	Bolt = Feather{
		Dispatch: Inline,
		Engine:   Epoll,
		Response: Direct,
		Buffer:   ZeroCopy,
		Conn:     Pipeline,
	}

	// Flash — fast in-memory responses with standard keep-alive.
	// Inline dispatch, fixed buffer, no pipelining.
	// Use for: JSON API without I/O, cached lookups.
	Flash = Feather{
		Dispatch: Inline,
		Engine:   Epoll,
		Response: Direct,
		Buffer:   Fixed,
		Conn:     KeepAlive,
	}

	// Arrow — the default preset for most routes.
	// Pool dispatch with direct write for minimal overhead.
	// Use for: DB queries, Redis, short external API calls.
	Arrow = Feather{
		Dispatch: Pool,
		Engine:   Epoll,
		Response: DirectWrite,
		Buffer:   Fixed,
		Conn:     KeepAlive,
	}

	// Hawk — pool dispatch with growable buffer for dynamic content.
	// Use for: template rendering, Fortunes, variable-size responses.
	Hawk = Feather{
		Dispatch: Pool,
		Engine:   Epoll,
		Response: Writeback,
		Buffer:   Grow,
		Conn:     KeepAlive,
	}

	// Glide — persistent connection for streaming protocols.
	// Use for: SSE, WebSocket, long-poll.
	Glide = Feather{
		Dispatch: Persist,
		Engine:   Epoll,
		Response: Chunked,
		Buffer:   Stream,
		Conn:     Upgrade,
	}

	// Talon — optimized file serving with io_uring.
	// Use for: static files, downloads, large uploads.
	Talon = Feather{
		Dispatch: Pool,
		Engine:   IOURing,
		Response: Sendfile,
		Buffer:   Registered,
		Conn:     OneShot,
	}

	// Soar — standard Go net/http for protocol features.
	// Use for: HTTP/2, gRPC, TLS termination.
	Soar = Feather{
		Dispatch: Spawn,
		Engine:   Net,
		Response: Direct,
		Buffer:   Grow,
		Conn:     KeepAlive,
	}
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
	case Persist:
		return "Persist"
	default:
		return "Unknown"
	}
}

func (m EngineMode) String() string {
	switch m {
	case Epoll:
		return "Epoll"
	case IOURing:
		return "IOURing"
	case Splice:
		return "Splice"
	case Net:
		return "Net"
	default:
		return "Unknown"
	}
}

func (m ResponseMode) String() string {
	switch m {
	case Direct:
		return "Direct"
	case Writeback:
		return "Writeback"
	case DirectWrite:
		return "DirectWrite"
	case Batch:
		return "Batch"
	case Chunked:
		return "Chunked"
	case Sendfile:
		return "Sendfile"
	default:
		return "Unknown"
	}
}

func (m BufferMode) String() string {
	switch m {
	case Fixed:
		return "Fixed"
	case Grow:
		return "Grow"
	case ZeroCopy:
		return "ZeroCopy"
	case Stream:
		return "Stream"
	case Registered:
		return "Registered"
	default:
		return "Unknown"
	}
}

func (m ConnMode) String() string {
	switch m {
	case Pipeline:
		return "Pipeline"
	case KeepAlive:
		return "KeepAlive"
	case OneShot:
		return "OneShot"
	case Upgrade:
		return "Upgrade"
	default:
		return "Unknown"
	}
}
