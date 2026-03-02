//go:build !linux && !darwin

package wing

import "fmt"

func createListenFd(_ string) (int, error) {
	return 0, fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}
