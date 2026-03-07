//go:build linux

package wing

import (
	"syscall"
	"unsafe"
)

func createEventfd() (int, error) {
	fd, _, errno := syscall.RawSyscall(syscall.SYS_EVENTFD2, 0, syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(fd), nil
}

// createWakeFds creates an eventfd for wake signaling.
// For eventfd, read and write use the same fd.
func createWakeFds() (r, w int, err error) {
	fd, err := createEventfd()
	return fd, fd, err
}

func eventfdWrite(fd int) {
	val := uint64(1)
	syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(unsafe.Pointer(&val)), 8)
}

func eventfdRead(fd int) {
	var val uint64
	syscall.RawSyscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(&val)), 8)
}
