//go:build !linux && !darwin

package wing

// Socket stubs for unsupported platforms.
// createListenFd is only called from transport.go (linux || darwin).
