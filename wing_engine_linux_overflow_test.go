//go:build linux

package kruda

import (
	"net"
	"strconv"
	"syscall"
	"testing"
	"unsafe"
)

func TestEpollWaitPreservesReadinessBeyondWorkerBatch(t *testing.T) {
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	e := &epollEngine{
		epfd:     epfd,
		evfd:     -1,
		listenFd: -1,
		connPtrs: make(map[int32]unsafe.Pointer),
		epevs:    make([]epollEvent, maxEventsPerWait*2),
	}
	defer e.Close()

	type pipePair [2]int
	pipes := make([]pipePair, maxEventsPerWait*2)
	for i := range pipes {
		if err := syscall.Pipe(pipes[i][:]); err != nil {
			t.Fatal(err)
		}
		defer syscall.Close(pipes[i][0])
		defer syscall.Close(pipes[i][1])
		if err := syscall.SetNonblock(pipes[i][0], true); err != nil {
			t.Fatal(err)
		}
		e.epollAdd(int32(pipes[i][0]), epollin|epollet)
		if _, err := syscall.Write(pipes[i][1], []byte{1}); err != nil {
			t.Fatal(err)
		}
	}

	var events [maxEventsPerWait]event
	total := 0
	for range 4 {
		n, err := e.WaitNonBlock(events[:])
		if err != nil {
			t.Fatal(err)
		}
		total += n
	}
	if total != len(pipes) {
		t.Fatalf("delivered %d readiness events, want %d", total, len(pipes))
	}
}

func TestEpollAcceptContinuesAfterWorkerBatchFills(t *testing.T) {
	listenFd, err := createListenFd("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFd(listenFd)
	sa, err := syscall.Getsockname(listenFd)
	if err != nil {
		t.Fatal(err)
	}
	port := sa.(*syscall.SockaddrInet4).Port

	eventFd, err := createEventfd()
	if err != nil {
		t.Fatal(err)
	}
	defer closeFd(eventFd)

	e := newEngine().(*epollEngine)
	if err := e.Init(engineConfig{EventFd: eventFd}); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.SubmitAccept(listenFd)

	clients := make([]net.Conn, 0, maxEventsPerWait*2)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for range maxEventsPerWait * 2 {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, conn)
	}
	defer func() {
		for _, conn := range clients {
			conn.Close()
		}
	}()

	var events [maxEventsPerWait]event
	accepted := 0
	for range 4 {
		n, err := e.WaitNonBlock(events[:])
		if err != nil {
			t.Fatal(err)
		}
		for _, ev := range events[:n] {
			if ev.Op != opAccept || ev.Res < 0 {
				continue
			}
			accepted++
			e.SubmitClose(ev.Res)
		}
	}
	if accepted != len(clients) {
		t.Fatalf("accepted %d connections, want %d", accepted, len(clients))
	}
}

func TestEpollAcceptFanoutPreservesReturnedConnectionEvent(t *testing.T) {
	listenFd, err := createListenFd("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFd(listenFd)
	sa, err := syscall.Getsockname(listenFd)
	if err != nil {
		t.Fatal(err)
	}
	port := sa.(*syscall.SockaddrInet4).Port

	eventFd, err := createEventfd()
	if err != nil {
		t.Fatal(err)
	}
	defer closeFd(eventFd)
	e := newEngine().(*epollEngine)
	if err := e.Init(engineConfig{EventFd: eventFd}); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.SubmitAccept(listenFd)

	clients := make([]net.Conn, 0, maxEventsPerWait*2)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	for range maxEventsPerWait * 2 {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, conn)
	}
	defer func() {
		for _, conn := range clients {
			conn.Close()
		}
	}()

	var pipeFds [2]int
	if err := syscall.Pipe(pipeFds[:]); err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(pipeFds[0])
	defer syscall.Close(pipeFds[1])
	marker := 1
	e.connPtrs[int32(pipeFds[0])] = unsafe.Pointer(&marker)
	if _, err := syscall.Write(pipeFds[1], []byte{1}); err != nil {
		t.Fatal(err)
	}
	e.epollAdd(int32(pipeFds[0]), epollin|epollet)

	var events [maxEventsPerWait]event
	n, err := e.WaitNonBlock(events[:])
	if err != nil {
		t.Fatal(err)
	}
	sawConnection := false
	for _, ev := range events[:n] {
		switch {
		case ev.Op == opAccept && ev.Res >= 0:
			e.SubmitClose(ev.Res)
		case ev.Op == opRecv && ev.ConnPtr == unsafe.Pointer(&marker):
			sawConnection = true
		}
	}
	if !sawConnection {
		t.Fatal("accept fanout discarded a connection event already returned by epoll")
	}
}
