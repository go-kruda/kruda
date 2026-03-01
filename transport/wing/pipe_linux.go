//go:build linux

package wing

import "syscall"

func createPipe() (r, w int, err error) {
	var fds [2]int
	if err := syscall.Pipe2(fds[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return 0, 0, err
	}
	return fds[0], fds[1], nil
}
