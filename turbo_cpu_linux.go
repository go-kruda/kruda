//go:build linux

package kruda

import (
	"os"
	"runtime"
	"strconv"
)

// availableCPUs returns CPUs available to this process.
// Reads cgroup v2 cpu.max quota first (containers/Kubernetes),
// falls back to runtime.NumCPU() if unlimited or unreadable.
func availableCPUs() int {
	if data, err := os.ReadFile("/sys/fs/cgroup/cpu.max"); err == nil {
		fields := make([][]byte, 0, 2)
		start := 0
		for i, b := range data {
			if b == ' ' || b == '\n' {
				if i > start {
					fields = append(fields, data[start:i])
				}
				start = i + 1
			}
		}
		if len(fields) >= 2 && string(fields[0]) != "max" {
			quota, err1 := strconv.ParseInt(string(fields[0]), 10, 64)
			period, err2 := strconv.ParseInt(string(fields[1]), 10, 64)
			if err1 == nil && err2 == nil && period > 0 {
				if n := int((quota + period - 1) / period); n >= 1 {
					return n
				}
			}
		}
	}
	return runtime.NumCPU()
}
