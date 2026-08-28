//go:build linux || darwin

package kruda

import (
	"os"
	"syscall"
)

func duplicateStaticFile(file *os.File) (int32, bool) {
	fd, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		return 0, false
	}
	if fd == 0 {
		next, dupErr := syscall.Dup(int(file.Fd()))
		_ = syscall.Close(fd)
		if dupErr != nil {
			return 0, false
		}
		fd = next
	}
	syscall.CloseOnExec(fd)
	return int32(fd), true
}
