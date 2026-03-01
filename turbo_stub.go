//go:build !linux

package kruda

import (
	"errors"
	"net"
	"runtime"
)

var errTurboUnsupported = errors.New("kruda: turbo mode requires Linux (SO_REUSEPORT)")

// ReuseportListener is not supported on non-Linux platforms.
func ReuseportListener(addr string) (net.Listener, error) {
	return nil, errTurboUnsupported
}

// IsSupervisor always returns false on non-Linux — no fork model.
func IsSupervisor() bool { return false }

// IsChild always returns false on non-Linux.
func IsChild() bool { return false }

// ChildID always returns -1 on non-Linux.
func ChildID() int { return -1 }

// Supervisor on non-Linux falls back to tuning GOMAXPROCS for single-process mode.
type Supervisor struct {
	Addr       string
	Processes  int
	CPUPercent float64
	GoMaxProcs int
}

// Run on non-Linux tunes GOMAXPROCS and returns nil so the caller continues to serve normally.
func (s *Supervisor) Run() error {
	n := resolveCPUs(s.Processes, s.CPUPercent)
	runtime.GOMAXPROCS(n)
	return nil
}

// SetupChild is a no-op on non-Linux.
func SetupChild() {}
