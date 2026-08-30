//go:build linux || darwin

package kruda

import (
	"os"
	"syscall"
)

func duplicateStaticFile(file *os.File) (int32, bool) {
	fd, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), uintptr(syscall.F_DUPFD_CLOEXEC), 3)
	if errno != 0 {
		return 0, false
	}
	return int32(fd), true
}
