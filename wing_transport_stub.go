//go:build !linux && !darwin

package kruda

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/go-kruda/kruda/transport"
)

// WingConfig, Preset, PresetOption, DispatchMode, RawRequest and their
// preset/helper definitions are declared in wing_types_shared.go (no build
// tag) so this stub never drifts out of sync with the linux/darwin impl.

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

// SetRoutePreset is a no-op on this platform; the stub Wing transport
// has no router to configure.
func (t *Transport) SetRoutePreset(_, _ string, _ any) {}

// Shutdown is a no-op on this platform because the stub Wing transport
// never starts any workers.
func (t *Transport) Shutdown(_ context.Context) error { return nil }

