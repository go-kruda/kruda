//go:build windows

package wing

import (
	"fmt"
	"net"
	"syscall"
)

func createListenFd(addr string) (int, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return 0, err
	}

	family := syscall.AF_INET
	if tcpAddr.IP != nil && tcpAddr.IP.To4() == nil {
		family = syscall.AF_INET6
	}

	// syscall.Socket on Windows creates overlapped-capable sockets
	// (WSA_FLAG_OVERLAPPED is set internally), which is required for IOCP.
	fd, err := syscall.Socket(family, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, fmt.Errorf("socket: %w", err)
	}

	// SO_REUSEADDR on Windows allows multiple sockets to bind to the same
	// address+port, enabling per-worker listen sockets (similar to
	// SO_REUSEPORT on Linux).
	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	syscall.SetsockoptInt(fd, syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)

	var sa syscall.Sockaddr
	if family == syscall.AF_INET {
		sa4 := &syscall.SockaddrInet4{Port: tcpAddr.Port}
		if ip4 := tcpAddr.IP.To4(); ip4 != nil {
			copy(sa4.Addr[:], ip4)
		}
		sa = sa4
	} else {
		sa6 := &syscall.SockaddrInet6{Port: tcpAddr.Port}
		if ip6 := tcpAddr.IP.To16(); ip6 != nil {
			copy(sa6.Addr[:], ip6)
		}
		sa = sa6
	}

	if err := syscall.Bind(fd, sa); err != nil {
		syscall.Closesocket(fd)
		return 0, fmt.Errorf("bind %s: %w", addr, err)
	}
	if err := syscall.Listen(fd, 4096); err != nil {
		syscall.Closesocket(fd)
		return 0, fmt.Errorf("listen: %w", err)
	}
	return int(fd), nil
}
