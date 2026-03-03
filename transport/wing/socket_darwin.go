//go:build darwin

package wing

import (
	"fmt"
	"net"
	"syscall"
)
// SO_REUSEPORT on macOS/BSD.
const soReusePort = syscall.SO_REUSEPORT

func createListenFd(addr string) (int, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return 0, err
	}

	family := syscall.AF_INET
	if tcpAddr.IP != nil && tcpAddr.IP.To4() == nil {
		family = syscall.AF_INET6
	}

	fd, err := syscall.Socket(family, syscall.SOCK_STREAM, 0)
	if err != nil {
		return 0, fmt.Errorf("socket: %w", err)
	}

	// macOS doesn't have SOCK_NONBLOCK/SOCK_CLOEXEC in socket(), set them separately.
	syscall.SetNonblock(fd, true)
	syscall.CloseOnExec(fd)

	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, soReusePort, 1)
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
		syscall.Close(fd)
		return 0, fmt.Errorf("bind %s: %w", addr, err)
	}
	if err := syscall.Listen(fd, 4096); err != nil {
		syscall.Close(fd)
		return 0, fmt.Errorf("listen: %w", err)
	}
	return fd, nil
}

// setCPUAffinity is a no-op on macOS (no sched_setaffinity).
func setCPUAffinity(_ int) {}

func setTCPQuickACK(_ int32) {}

// getPeerAddr returns "ip:port" of the remote end of fd, or "" on error.
func getPeerAddr(fd int32) string {
	sa, err := syscall.Getpeername(int(fd))
	if err != nil {
		return ""
	}
	switch v := sa.(type) {
	case *syscall.SockaddrInet4:
		return fmt.Sprintf("%d.%d.%d.%d:%d", v.Addr[0], v.Addr[1], v.Addr[2], v.Addr[3], v.Port)
	case *syscall.SockaddrInet6:
		return fmt.Sprintf("[%x]:%d", v.Addr, v.Port)
	}
	return ""
}
