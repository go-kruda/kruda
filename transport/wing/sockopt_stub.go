//go:build !linux && !darwin

package wing

// setTCPNodelay is a no-op on unsupported platforms.
func setTCPNodelay(_ int32) {}

// closeFd is a no-op on unsupported platforms.
func closeFd(_ int) {}
