//go:build linux || darwin

package wing

import "syscall"

// setTCPNodelay enables TCP_NODELAY on the given socket fd.
func setTCPNodelay(fd int32) {
	syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_NODELAY, 1)
}

// closeFd closes a file descriptor (socket or pipe).
func closeFd(fd int) {
	syscall.Close(fd)
}
