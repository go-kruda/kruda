//go:build !linux && !darwin && !windows

package wing

import "fmt"

func createPipe() (r, w int, err error) {
	return 0, 0, fmt.Errorf("wing: pipes not supported on this platform")
}
