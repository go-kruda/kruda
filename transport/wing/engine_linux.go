//go:build linux

package wing

import (
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	epollin      = 0x1
	epollout     = 0x4
	epollet      = 0x80000000
	epolloneshot = 0x40000000

	epollCtlAdd = 1
	epollCtlMod = 3
	epollCtlDel = 2
)

type epollEvent struct {
	events uint32
	data   [8]byte // fd packed as int32 in first 4 bytes
}

type epollEngine struct {
	epfd     int
	evfd     int // eventfd for wake (replaces pipe pair)
	rawMode  bool
	wakeup   int32
	idle     int32
	connPtrs map[int32]unsafe.Pointer
	listenFd int
	epevs    []epollEvent // elastic event list
}

func newEngine() engine {
	return &epollEngine{
		connPtrs: make(map[int32]unsafe.Pointer, 1024),
		epevs:    make([]epollEvent, 128),
	}
}

func (e *epollEngine) Init(cfg engineConfig) error {
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return err
	}
	e.epfd = epfd
	e.evfd = cfg.EventFd
	e.rawMode = cfg.RawMode
	return nil
}

func (e *epollEngine) PostWake() {
	if atomic.CompareAndSwapInt32(&e.wakeup, 0, 1) {
		eventfdWrite(e.evfd)
	}
}

func (e *epollEngine) SubmitAccept(listenFd int) {
	e.listenFd = listenFd
	e.epollAdd(int32(listenFd), epollin|epollet)
	// Register eventfd for wake signaling
	e.epollAdd(int32(e.evfd), epollin|epollet)
}

func (e *epollEngine) SubmitPipeRecv(_ int, _ []byte) {
	// eventfd registered in SubmitAccept — no-op for compat
}

func (e *epollEngine) RegisterConn(fd int32, ptr unsafe.Pointer) {
	e.connPtrs[fd] = ptr
	// Edge-triggered read — fires when new data arrives. No re-arm needed.
	e.epollMod(fd, epollin|epollet)
}

func (e *epollEngine) SubmitRecv(fd int32, _ []byte, _ int) {
	// Edge-triggered EPOLLIN persists — only need epollMod to remove EPOLLOUT
	// after a partial write fallback. Track with hasOut flag on conn.
	e.epollModPtr(fd, epollin|epollet)
}

func (e *epollEngine) SubmitSend(fd int32, _ []byte) {
	// Add EPOLLOUT for partial write fallback.
	e.epollModPtr(fd, epollin|epollout|epollet)
}

func (e *epollEngine) SubmitClose(fd int32) {
	delete(e.connPtrs, fd)
	syscall.EpollCtl(e.epfd, epollCtlDel, int(fd), nil)
	syscall.Close(int(fd))
}

func (e *epollEngine) Detach(fd int32) {
	delete(e.connPtrs, fd)
	syscall.EpollCtl(e.epfd, epollCtlDel, int(fd), nil)
}

func (e *epollEngine) Wait(events []event) (int, error) {
	// Adaptive: non-blocking first when busy, block when idle.
	msec := 0
	if e.idle > 64 {
		msec = -1
	}
	n, err := e.waitWithTimeout(events, msec)
	if n > 0 {
		e.idle = 0
	} else {
		e.idle++
		if msec == 0 && e.idle <= 64 {
			runtime.Gosched()
		}
	}
	return n, err
}

func (e *epollEngine) WaitNonBlock(events []event) (int, error) {
	return e.waitWithTimeout(events, 0)
}

func (e *epollEngine) waitWithTimeout(events []event, msec int) (int, error) {
	n, err := epollWait(e.epfd, e.epevs, msec, e.rawMode)
	if err != nil {
		if err == syscall.EINTR {
			return 0, nil
		}
		return 0, err
	}

	// Elastic resize
	if n == len(e.epevs) && len(e.epevs) < 1024 {
		e.epevs = make([]epollEvent, len(e.epevs)*2)
	} else if n < len(e.epevs)/4 && len(e.epevs) > 128 {
		e.epevs = make([]epollEvent, len(e.epevs)/2)
	}

	count := 0
	for i := 0; i < n && count < len(events); i++ {
		ev := &e.epevs[i]
		// All epoll_data stores fd as int32 in data[0:4].
		fd := *(*int32)(unsafe.Pointer(&ev.data[0]))

		if int(fd) == e.listenFd {
			count += e.drainAccept(events[count:])
			continue
		}

		if int(fd) == e.evfd {
			eventfdRead(e.evfd)
			atomic.StoreInt32(&e.wakeup, 0)
			events[count] = event{Op: opWake, Fd: int32(e.evfd)}
			count++
			continue
		}

		// Conn fd — look up pointer from connPtrs map.
		ptr := e.connPtrs[fd]

		if ev.events&epollout != 0 {
			events[count] = event{Op: opSend, ConnPtr: ptr}
			count++
			continue
		}

		if ev.events&epollin != 0 {
			events[count] = event{Op: opRecv, ConnPtr: ptr}
			count++
		}
	}
	return count, nil
}

func (e *epollEngine) drainAccept(events []event) int {
	// Re-arm listen fd (ET, persistent — no need to re-add)
	count := 0
	for count < len(events) {
		nfd, _, err := syscall.Accept4(e.listenFd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				break
			}
			events[count] = event{Op: opAccept, Res: -1}
			count++
			break
		}
		e.epollAdd(int32(nfd), 0)
		events[count] = event{Op: opAccept, Res: int32(nfd)}
		count++
	}
	return count
}

func (e *epollEngine) Flush() error { return nil }

func (e *epollEngine) Close() {
	if e.epfd > 0 {
		syscall.Close(e.epfd)
	}
}

// epoll helpers

func (e *epollEngine) epollAdd(fd int32, events uint32) {
	ev := epollEvent{events: events}
	*(*int32)(unsafe.Pointer(&ev.data[0])) = fd
	syscall.EpollCtl(e.epfd, epollCtlAdd, int(fd), (*syscall.EpollEvent)(unsafe.Pointer(&ev)))
}

func (e *epollEngine) epollMod(fd int32, events uint32) {
	ev := epollEvent{events: events}
	*(*int32)(unsafe.Pointer(&ev.data[0])) = fd
	syscall.EpollCtl(e.epfd, epollCtlMod, int(fd), (*syscall.EpollEvent)(unsafe.Pointer(&ev)))
}

func (e *epollEngine) epollModPtr(fd int32, events uint32) {
	// All epoll_data now stores fd — pointer lookup via connPtrs map.
	e.epollMod(fd, events)
}

func epollWait(epfd int, events []epollEvent, msec int, raw bool) (int, error) {
	var n uintptr
	var errno syscall.Errno
	if raw {
		n, _, errno = syscall.RawSyscall6(
			syscall.SYS_EPOLL_PWAIT,
			uintptr(epfd),
			uintptr(unsafe.Pointer(&events[0])),
			uintptr(len(events)),
			uintptr(msec),
			0, 0,
		)
	} else {
		n, _, errno = syscall.Syscall6(
			syscall.SYS_EPOLL_PWAIT,
			uintptr(epfd),
			uintptr(unsafe.Pointer(&events[0])),
			uintptr(len(events)),
			uintptr(msec),
			0, 0,
		)
	}
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}
