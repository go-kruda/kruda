//go:build darwin

package wing

import "syscall"

func createPipe() (r, w int, err error) {
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		return 0, 0, err
	}
	// macOS doesn't have Pipe2, set flags manually.
	syscall.SetNonblock(fds[0], true)
	syscall.CloseOnExec(fds[0])
	syscall.SetNonblock(fds[1], true)
	syscall.CloseOnExec(fds[1])
	return fds[0], fds[1], nil
}
