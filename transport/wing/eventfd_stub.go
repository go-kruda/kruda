//go:build darwin

package wing

// createWakeFds creates a pipe pair for wake signaling on macOS.
// Linux uses eventfd instead (eventfd_linux.go).
func createWakeFds() (r, w int, err error) {
	return createPipe()
}
