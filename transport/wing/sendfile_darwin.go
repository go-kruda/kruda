//go:build darwin

package wing

import "syscall"

// sendfile wraps the Darwin sendfile(2) syscall.
func sendfile(outFd, inFd int32, offset *int64, count int) (int, error) {
	n, err := syscall.Sendfile(int(outFd), int(inFd), offset, count)
	return n, err
}
