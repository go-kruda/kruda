//go:build !linux

package kruda

import "net"

// optimizedListener on non-Linux platforms falls back to standard net.Listen.
// TCP_DEFER_ACCEPT and TCP_FASTOPEN are Linux-specific.
func optimizedListener(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
