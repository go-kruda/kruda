//go:build !linux && !darwin

package wing

// Pipe stubs for unsupported platforms.
// createPipe is only called from eventfd_stub.go (darwin) and transport.go (linux || darwin).
