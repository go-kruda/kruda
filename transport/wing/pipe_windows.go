//go:build windows

package wing

// createPipe returns dummy pipe fds on Windows.
// IOCP uses PostQueuedCompletionStatus for wake instead of pipes.
// The -1 sentinel values cause transport.go to skip pipe cleanup.
func createPipe() (r, w int, err error) {
	return -1, -1, nil
}
