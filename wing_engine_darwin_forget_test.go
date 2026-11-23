//go:build darwin

package kruda

import (
	"syscall"
	"testing"
)

// TestKqueueCloseDropsQueuedRegistration queues a read registration and closes
// the fd before the next kevent call. Submitted after the close, it would
// watch whatever socket reuses the number. A live socket's change is kept.
func TestKqueueCloseDropsQueuedRegistration(t *testing.T) {
	socks, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(socks[1])
	live, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(live[0])
	defer syscall.Close(live[1])
	e := newEngine().(*kqueueEngine)
	e.SubmitRecv(int32(socks[0]), make([]byte, 16), 0)
	e.SubmitRecv(int32(live[0]), make([]byte, 16), 0)
	e.SubmitClose(int32(socks[0]))
	e.liveChanges() // what Wait does before kevent
	if len(e.changes) != 1 || e.changes[0].Ident != uint64(live[0]) {
		t.Fatalf("queued changes %+v, want only fd %d", e.changes, live[0])
	}
}
