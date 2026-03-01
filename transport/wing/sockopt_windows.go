//go:build windows

package wing

import "syscall"

// setTCPNodelay enables TCP_NODELAY on the given socket handle.
// On Windows, SetsockoptInt takes syscall.Handle, not int.
func setTCPNodelay(fd int32) {
	syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}

// closeFd closes a socket handle.
// On Windows, sockets must be closed with Closesocket, not Close.
// Pipe fds are -1 (dummy) on Windows and should be skipped by the caller.
func closeFd(fd int) {
	syscall.Closesocket(syscall.Handle(fd))
}
