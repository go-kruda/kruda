package kruda

import "math"

// resolveCPUs returns the number of CPUs/processes to use.
// Priority: Processes > CPUPercent > availableCPUs()
func resolveCPUs(processes int, pct float64) int {
	if processes > 0 {
		return processes
	}
	n := availableCPUs()
	if pct > 0 && pct < 100 {
		n = int(math.Ceil(float64(n) * pct / 100))
		if n < 1 {
			n = 1
		}
	}
	return n
}
