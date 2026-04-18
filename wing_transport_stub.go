//go:build !linux && !darwin

package kruda

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// WingConfig for Wing transport (unsupported on this platform).
// Fields mirror the real WingConfig so code compiles cross-platform.
type WingConfig struct {
	Workers           int
	RingSize          uint32
	ReadBufSize       int
	MaxHeaderCount    int
	MaxHeaderSize     int
	MaxConnsPerWorker int
	HandlerPoolSize   int
	Feathers          map[string]Feather
	DefaultFeather    Feather
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Transport is a stub on unsupported platforms.
type Transport struct {
	ready    chan struct{}
	shutdown chan struct{}
	wg       sync.WaitGroup
	config   WingConfig
}

// NewWingTransport returns a stub Transport that errors on Listen.
func NewWingTransport(cfg WingConfig) *Transport {
	return &Transport{ready: make(chan struct{}), config: cfg}
}

// ListenAndServe always returns an error on this platform — Wing requires
// epoll (Linux) or kqueue (Darwin). Use FastHTTP or NetHTTP instead.
func (t *Transport) ListenAndServe(_ string, _ transport.Handler) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}

// Serve always returns an error on this platform — Wing requires epoll
// (Linux) or kqueue (Darwin). Use FastHTTP or NetHTTP instead.
func (t *Transport) Serve(_ net.Listener, _ transport.Handler) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}

// SetRouteFeather is a no-op on this platform; the stub Wing transport
// has no router to configure.
func (t *Transport) SetRouteFeather(_, _ string, _ any) {}

// Shutdown is a no-op on this platform because the stub Wing transport
// never starts any workers.
func (t *Transport) Shutdown(_ context.Context) error { return nil }

// DispatchMode controls how Wing dispatches requests.
type DispatchMode uint8

// DispatchMode constants mirror the real Wing definitions so cross-platform
// code referencing them compiles. The stub transport never reads them.
const (
	// Inline runs the handler on the ioLoop goroutine (zero overhead).
	Inline DispatchMode = iota + 1
	// Pool dispatches to a bounded goroutine pool.
	Pool
	// Spawn creates a new goroutine per request.
	Spawn
	// Takeover hands the connection to a goroutine using blocking I/O.
	Takeover
)

// Feather is a per-route optimization hint.
type Feather struct {
	Dispatch       DispatchMode
	StaticResponse []byte
}

// Feather presets mirror the real Wing definitions so user code compiles
// cross-platform. They have no effect when the stub transport is used.
var (
	// Bolt — Inline dispatch preset.
	Bolt = Feather{Dispatch: Inline}
	// Arrow — Pool dispatch preset.
	Arrow = Feather{Dispatch: Pool}
	// Spear — Takeover dispatch preset (goroutine-owned connection).
	Spear = Feather{Dispatch: Takeover}
	// Plaintext is a Bolt-aliased preset for static text/health-check routes.
	Plaintext = Bolt
	// JSON is a Bolt-aliased preset for JSON-only handlers (no I/O).
	JSON = Bolt
	// Query is a Spear-aliased preset for short DB/Redis lookups.
	Query = Spear
	// Render is a Spear-aliased preset for DB + template/HTML responses.
	Render = Spear
)

// FeatherOption configures a Feather.
type FeatherOption func(*Feather)

// With returns a copy of f with the given options applied. Mirrors the real
// Wing implementation so cross-platform code compiles.
func (f Feather) With(opts ...FeatherOption) Feather {
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

func (f *Feather) defaults() {
	if f.Dispatch == 0 {
		f.Dispatch = Inline
	}
}

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

// Dispatch returns a FeatherOption that sets the dispatch mode on a Feather.
func Dispatch(m DispatchMode) FeatherOption { return func(f *Feather) { f.Dispatch = m } }

// Static returns a FeatherOption that attaches a pre-built HTTP response,
// allowing Wing to skip the handler entirely on supported platforms.
func Static(resp []byte) FeatherOption { return func(f *Feather) { f.StaticResponse = resp } }

// NOTE: Feather types above are kept in sync with feather.go — update both together.

// RawRequest provides low-level access to Wing's request data.
// Mirrors the real interface in raw.go so user code compiles cross-platform.
type RawRequest interface {
	RawMethod() string
	RawPath() []byte
	RawHeader(name string) []byte
	RawBody() []byte
	Fd() int32
	KeepAlive() bool
}
