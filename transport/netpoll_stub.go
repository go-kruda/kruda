//go:build windows

package transport

import (
	"errors"
	"time"
)

// NetpollTransport is a stub for Windows where netpoll is not supported.
// On Linux and macOS, the real implementation in netpoll.go is used instead.
type NetpollTransport struct{}

// NetpollConfig holds configuration for the netpoll transport.
// This is a stub matching the real config so code compiles on Windows.
type NetpollConfig struct {
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxBodySize    int
	MaxHeaderBytes int
	TrustProxy     bool
}

// NewNetpoll returns an error on Windows because netpoll relies on
// epoll/kqueue which are not available on this platform.
func NewNetpoll(cfg NetpollConfig) (*NetpollTransport, error) {
	return nil, errors.New("netpoll: not supported on Windows")
}
