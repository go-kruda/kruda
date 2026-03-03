//go:build linux

package wing

import (
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

// SO_REUSEPORT on Linux.
const soReusePort = 0xf

func createListenFd(addr string) (int, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return 0, err
	}

	family := syscall.AF_INET
	if tcpAddr.IP != nil && tcpAddr.IP.To4() == nil {
		family = syscall.AF_INET6
	}

	fd, err := syscall.Socket(family, syscall.SOCK_STREAM|syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return 0, fmt.Errorf("socket: %w", err)
	}

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

// setCPUAffinity pins the current OS thread to a specific CPU core.
func setCPUAffinity(cpu int) {
	var mask [1024 / 64]uintptr
	mask[cpu/64] = 1 << (uint(cpu) % 64)
	syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(len(mask)*8), uintptr(unsafe.Pointer(&mask[0])))
}

// setTCPQuickACK disables delayed ACK on Linux.
func setTCPQuickACK(fd int32) {
	const tcpQuickACK = 12
	syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, tcpQuickACK, 1)
}
