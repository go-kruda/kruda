//go:build darwin

package kruda

import (
	"syscall"
	"testing"
)

// TestKqueueCloseDropsQueuedRegistration queues a read registration and closes
// the fd before the next kevent call. Submitted after the close, it would
// watch whatever socket reuses the number.
func TestKqueueCloseDropsQueuedRegistration(t *testing.T) {
	socks, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(socks[1])
	e := newEngine().(*kqueueEngine)
	e.SubmitRecv(int32(socks[0]), make([]byte, 16), 0)
	e.SubmitClose(int32(socks[0]))
	for _, ch := range e.changes {
		if ch.Ident == uint64(socks[0]) {
			t.Fatalf("queued change kept for closed fd %d: %+v", socks[0], ch)
		}
	}
}
