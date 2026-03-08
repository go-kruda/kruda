//go:build !linux

package wing

// createWakeFds creates a pipe pair for wake signaling on non-Linux platforms.
// Returns (readFd, writeFd, err). Both fds are needed for kqueue.
func createWakeFds() (r, w int, err error) {
	return createPipe()
}
