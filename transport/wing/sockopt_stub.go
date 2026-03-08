//go:build !linux && !darwin

package wing

// Socket option stubs for unsupported platforms.
// These functions are only called from transport.go (linux || darwin),
// so no function stubs are needed here.
