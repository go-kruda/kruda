//go:build linux || darwin

package kruda

import (
	"context"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

// newSocketpairFiles returns two connected *os.File endpoints (AF_UNIX stream).
func newSocketpairFiles(t *testing.T) (a, b *os.File) {
	t.Helper()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	return os.NewFile(uintptr(fds[0]), "a"), os.NewFile(uintptr(fds[1]), "b")
}

func TestWingHijackConn_SatisfiesNetConn(t *testing.T) {
	var _ net.Conn = (*wingHijackConn)(nil) // compile-time assertion
	af, bf := newSocketpairFiles(t)
	defer af.Close()
	defer bf.Close()
	c := newWingHijackConn(af, int32(af.Fd()), "203.0.113.7:5555", context.Background())
	if c.RemoteAddr().String() != "203.0.113.7:5555" {
		t.Errorf("RemoteAddr = %q", c.RemoteAddr().String())
	}
	// Write on the adapter is readable on the peer.
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make([]byte, 4)
	bf.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := bf.Read(got); err != nil || string(got) != "ping" {
		t.Fatalf("peer read = %q, %v", got, err)
	}
}

func TestWingHijackConn_CloseIsCoordinated(t *testing.T) {
	af, bf := newSocketpairFiles(t)
	defer bf.Close()
	fd := int32(af.Fd())
	c := newWingHijackConn(af, fd, "", context.Background())

	// Close half-closes the socket (peer sees EOF) but does NOT close the fd:
	// af must still be a valid fd afterward (hijackTakeover owns the real close).
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := c.Close(); err != nil { // idempotent
		t.Fatalf("second close: %v", err)
	}
	// fd still open: a syscall referencing it must not be EBADF.
	var st syscall.Stat_t
	if err := syscall.Fstat(int(fd), &st); err == syscall.EBADF {
		t.Fatal("adapter Close physically closed the fd; it must not")
	}
	af.Close() // real close for the test
}

func TestWingHijackConn_Done(t *testing.T) {
	af, bf := newSocketpairFiles(t)
	defer af.Close()
	defer bf.Close()
	ctx, cancel := context.WithCancel(context.Background())
	c := newWingHijackConn(af, int32(af.Fd()), "", ctx)
	select {
	case <-c.Done():
		t.Fatal("Done fired before cancel")
	default:
	}
	cancel()
	select {
	case <-c.Done():
	case <-time.After(time.Second):
		t.Fatal("Done did not fire after cancel")
	}
}
