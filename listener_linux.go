//go:build linux

package kruda

import (
	"context"
	"net"
	"syscall"
)

// optimizedListener creates a TCP listener with performance socket options:
//   - TCP_DEFER_ACCEPT: kernel holds connection until client sends data
//   - TCP_FASTOPEN: data in SYN packet, saves 1 RTT for returning clients
//
// On Linux this is automatic; other platforms fall back to net.Listen.
func optimizedListener(addr string) (net.Listener, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			err := c.Control(func(fd uintptr) {
				f := int(fd)
				_ = syscall.SetsockoptInt(f, syscall.IPPROTO_TCP, tcpDeferAccept, 1)
				_ = syscall.SetsockoptInt(f, syscall.SOL_TCP, tcpFastOpen, fastOpenQueueLen)
			})
			if err != nil {
				return err
			}
			return opErr
		},
	}
	return lc.Listen(context.Background(), "tcp", addr)
}
