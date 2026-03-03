package wing

import "syscall"

// sendfile wraps the Linux sendfile(2) syscall.
// Sends count bytes from inFd to outFd starting at offset.
// Returns bytes sent and any error.
func sendfile(outFd, inFd int32, offset *int64, count int) (int, error) {
	n, err := syscall.Sendfile(int(outFd), int(inFd), offset, count)
	return n, err
}
