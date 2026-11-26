//go:build linux || darwin

package kruda

import (
	"net"
	"strconv"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// rawTestListener is a listening TCP socket on a free loopback port.
func rawTestListener(t *testing.T) (int, string) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Listen(fd, 16); err != nil {
		t.Fatal(err)
	}
	sa, err := syscall.Getsockname(fd)
	if err != nil {
		t.Fatal(err)
	}
	syscall.SetNonblock(fd, true)
	return fd, net.JoinHostPort("127.0.0.1", strconv.Itoa(sa.(*syscall.SockaddrInet4).Port))
}

// TestWingStopAcceptForgetsAReusedNumber closes a worker's listener and gives
// its number to another listening socket, as a handler opening its own
// listener during Shutdown could. The engine must not accept on it: that
// connection belongs to someone else.
func TestWingStopAcceptForgetsAReusedNumber(t *testing.T) {
	wakeR, wakeW, err := createWakeFds()
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(wakeR)
	if wakeW != wakeR {
		defer syscall.Close(wakeW)
	}
	e := newEngine()
	if err := e.Init(engineConfig{EventFd: wakeR, PipeW: wakeW}); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	old, _ := rawTestListener(t)
	e.SubmitAccept(old) // the running loop's arm, still queued on kqueue
	e.StopAccept()
	syscall.Close(old)
	other, addr := rawTestListener(t)
	if other != old { // the kernel usually hands the freed number straight back
		if err := unix.Dup2(other, old); err != nil {
			t.Fatal(err)
		}
		syscall.Close(other)
	}
	defer syscall.Close(old)
	client, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	time.Sleep(20 * time.Millisecond)
	var events [16]event
	n, _ := e.WaitFor(events[:], 100)
	for _, ev := range events[:n] {
		if ev.Op == opAccept {
			if ev.Res > 0 {
				syscall.Close(int(ev.Res))
			}
			t.Fatalf("accepted on the reused number %d", old)
		}
	}
	if nfd, _, err := syscall.Accept(old); err != nil {
		t.Fatalf("the new owner lost its connection: %v", err)
	} else {
		syscall.Close(nfd)
	}
}
