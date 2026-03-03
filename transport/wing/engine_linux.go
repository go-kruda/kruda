//go:build linux

package wing

import (
	"runtime"
	"sync/atomic"
	"syscall"
	"unsafe"
)

const (
	epollin    = 0x1
	epollout   = 0x4
	epollet    = 0x80000000
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
	pipeW    int
	pipeR    int
	pipeBuf  [8]byte
	rawMode  bool  // true = RawSyscall (LockOSThread), false = Syscall (async)
	wakeup   int32 // CAS gate for PostWake
	idle     int32 // consecutive zero-event polls for adaptive wait
	connPtrs map[int32]unsafe.Pointer // fd → *conn for epollMod data
	listenFd int
}

func newEngine() engine {
	return &epollEngine{
		connPtrs: make(map[int32]unsafe.Pointer, 1024),
	}
}

func (e *epollEngine) Init(cfg engineConfig) error {
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return err
	}
	e.epfd = epfd
	e.pipeW = cfg.PipeW
	e.rawMode = cfg.RawMode

	// find pipeR from pipeW — transport passes pipeW; we need pipeR for wake.
	// Instead, use the pipe pair created by transport: pipeW is in cfg,
	// but we need pipeR to register with epoll.
	// Solution: transport registers pipeR via SubmitPipeRecv on first call.
	e.pipeR = -1
	return nil
}

func (e *epollEngine) PostWake() {
	if atomic.CompareAndSwapInt32(&e.wakeup, 0, 1) {
		syscall.Write(e.pipeW, []byte{1})
	}
}

func (e *epollEngine) SubmitAccept(listenFd int) {
	e.listenFd = listenFd
	e.epollAdd(int32(listenFd), epollin|epollet)
}

func (e *epollEngine) SubmitPipeRecv(pipeFd int, _ []byte) {
	if e.pipeR == pipeFd {
		return
	}
	e.pipeR = pipeFd
	e.epollAdd(int32(pipeFd), epollin|epollet)
}

func (e *epollEngine) RegisterConn(fd int32, ptr unsafe.Pointer) {
	e.connPtrs[fd] = ptr
	// Edge-triggered read — fires when new data arrives. No re-arm needed.
	ev := epollEvent{events: epollin | epollet}
	*(*unsafe.Pointer)(unsafe.Pointer(&ev.data[0])) = ptr
	syscall.EpollCtl(e.epfd, epollCtlMod, int(fd), (*syscall.EpollEvent)(unsafe.Pointer(&ev)))
}

func (e *epollEngine) SubmitRecv(fd int32, _ []byte, _ int) {
	// Re-arm EPOLLIN only (remove EPOLLOUT if it was added by SubmitSend).
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

func (e *epollEngine) Wait(events []event) (int, error) {
	// Adaptive: non-blocking first when busy, block when idle.
	msec := 0 // non-blocking
	if e.idle > 64 {
		msec = -1 // block
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
	var epevs [128]epollEvent
	n, err := epollWait(e.epfd, epevs[:], msec, e.rawMode)
	if err != nil {
		if err == syscall.EINTR {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for i := 0; i < n && count < len(events); i++ {
		ev := &epevs[i]
		// Listen fd and pipe fd store int32 in data[0:4].
		// Conn fds store unsafe.Pointer in data[0:8].
		fd := *(*int32)(unsafe.Pointer(&ev.data[0]))

		if int(fd) == e.listenFd {
			count += e.drainAccept(events[count:])
			continue
		}

		if int(fd) == e.pipeR {
			syscall.Read(e.pipeR, e.pipeBuf[:])
			atomic.StoreInt32(&e.wakeup, 0)
			events[count] = event{Op: opWake, Fd: int32(e.pipeR)}
			count++
			continue
		}

		// Conn fd — decode pointer from epoll data.
		ptr := *(*unsafe.Pointer)(unsafe.Pointer(&ev.data[0]))

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
	ev := epollEvent{events: events}
	ptr := e.connPtrs[fd]
	*(*unsafe.Pointer)(unsafe.Pointer(&ev.data[0])) = ptr
	syscall.EpollCtl(e.epfd, epollCtlMod, int(fd), (*syscall.EpollEvent)(unsafe.Pointer(&ev)))
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
