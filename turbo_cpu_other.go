//go:build !linux

package kruda

import "runtime"

func availableCPUs() int { return runtime.NumCPU() }
