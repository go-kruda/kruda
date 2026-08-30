//go:build darwin

package kruda

import (
	"io"
	"syscall"
)

// sendfile wraps the Darwin sendfile(2) syscall.
func sendfile(outFd, inFd int32, offset *int64, count int) (int, error) {
	if offset == nil {
		current, err := syscall.Seek(int(inFd), 0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		n, sendErr := syscall.Sendfile(int(outFd), int(inFd), &current, count)
		if n > 0 {
			_, seekErr := syscall.Seek(int(inFd), current+int64(n), io.SeekStart)
			if seekErr != nil {
				return n, seekErr
			}
		}
		return n, sendErr
	}
	n, err := syscall.Sendfile(int(outFd), int(inFd), offset, count)
	return n, err
}
