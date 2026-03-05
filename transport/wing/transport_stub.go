//go:build !linux && !darwin

package wing

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// Config for Wing transport (unsupported on this platform).
// Fields mirror the real Config so code compiles cross-platform.
type Config struct {
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

func (c *Config) defaults()       {}
func (c *Config) needsPool() bool  { return false }
func (c *Config) needsAsync() bool { return false }

// Transport is a stub on unsupported platforms.
type Transport struct {
	ready    chan struct{}
	shutdown chan struct{}
	wg       sync.WaitGroup
	config   Config
}

// New returns a stub Transport that errors on Listen.
func New(cfg Config) *Transport {
	return &Transport{ready: make(chan struct{}), config: cfg}
}

func (t *Transport) ListenAndServe(_ string, _ transport.Handler) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}

func (t *Transport) Serve(_ net.Listener, _ transport.Handler) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}

func (t *Transport) SetRouteFeather(_, _ string, _ any) {}

func (t *Transport) Shutdown(_ context.Context) error { return nil }

// DispatchMode controls how Wing dispatches requests.
type DispatchMode uint8

const (
	Inline DispatchMode = iota + 1
	Pool
	Spawn
	Takeover
)

// Feather is a per-route optimization hint.
type Feather struct {
	Dispatch       DispatchMode
	StaticResponse []byte
}

var (
	Bolt      = Feather{Dispatch: Inline}
	Arrow     = Feather{Dispatch: Pool}
	Spear     = Feather{Dispatch: Takeover}
	Plaintext = Bolt
	JSON      = Bolt
	Query     = Spear
	Render    = Spear
)

// FeatherOption configures a Feather.
type FeatherOption func(*Feather)

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

func Dispatch(m DispatchMode) FeatherOption { return func(f *Feather) { f.Dispatch = m } }
func Static(resp []byte) FeatherOption      { return func(f *Feather) { f.StaticResponse = resp } }

// FeatherTable maps routes to Feathers.
type FeatherTable struct{}

// NewFeatherTable creates a stub FeatherTable.
func NewFeatherTable(_ map[string]Feather, _ Feather) FeatherTable { return FeatherTable{} }

// RawRequest is the interface for accessing raw Wing request data.
type RawRequest interface {
	RawBytes() []byte
}
