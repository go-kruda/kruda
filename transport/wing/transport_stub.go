//go:build !linux && !darwin

package wing

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-kruda/kruda/transport"
)

// Config for Wing transport (unsupported on this platform).
type Config struct {
	Workers      int
	RingSize     uint32
	ReadTimeout  int
	WriteTimeout int
	IdleTimeout  int
	MaxBodySize  int
	Feathers     map[string]Feather
}

func (c *Config) defaults()    {}
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
	return fmt.Errorf("wing: unsupported platform")
}

func (t *Transport) Serve(_ net.Listener, _ transport.Handler) error {
	return fmt.Errorf("wing: unsupported platform")
}

func (t *Transport) SetRouteFeather(_, _ string, _ any) {}

func (t *Transport) Shutdown(_ context.Context) error { return nil }

// DispatchMode controls how Wing dispatches requests.
type DispatchMode uint8

const (
	Inline DispatchMode = iota + 1
	Pool
	Spawn
)

// Feather is a per-route optimization hint.
type Feather struct {
	Dispatch       DispatchMode
	StaticResponse []byte
}

var (
	Bolt      = Feather{Dispatch: Inline}
	Arrow     = Feather{Dispatch: Pool}
	Plaintext = Bolt
	JSON      = Bolt
	ParamJSON = Bolt
	PostJSON  = Bolt
	Query     = Arrow
	Render    = Arrow
)

// FeatherOption configures a Feather.
type FeatherOption func(*Feather)

func (f Feather) With(_ ...FeatherOption) Feather { return f }
func (m DispatchMode) String() string             { return "stub" }
func Dispatch(m DispatchMode) FeatherOption       { return func(f *Feather) { f.Dispatch = m } }
func Static(resp []byte) FeatherOption            { return func(f *Feather) { f.StaticResponse = resp } }

// FeatherTable maps routes to Feathers.
type FeatherTable struct{}

// NewFeatherTable creates a stub FeatherTable.
func NewFeatherTable(_ map[string]Feather, _ Feather) FeatherTable { return FeatherTable{} }

// RawRequest is the interface for accessing raw Wing request data.
type RawRequest interface {
	RawBytes() []byte
}
