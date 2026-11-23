//go:build linux

package kruda

import (
	"context"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// After a peek finds another request, read readiness alone cannot distinguish
// more input from a FIN. Poll only for a hangup and leave the bytes queued.
func watchQueuedStreamPeer(raw syscall.RawConn, ctx context.Context, stop <-chan struct{}) bool {
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()

	for {
		closed := false
		err := raw.Control(func(fd uintptr) {
			fds := [1]unix.PollFd{{Fd: int32(fd), Events: unix.POLLRDHUP}}
			_, err := unix.Poll(fds[:], 0)
			closed = err != nil && err != unix.EINTR ||
				fds[0].Revents&(unix.POLLRDHUP|unix.POLLHUP|unix.POLLERR|unix.POLLNVAL) != 0
		})
		if err != nil || closed {
			return false
		}
		select {
		case <-stop:
			return true
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
}
