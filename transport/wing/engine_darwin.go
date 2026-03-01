//go:build darwin

package wing

import (
	"syscall"
)

// kqueueEngine implements the engine interface using macOS kqueue.
type kqueueEngine struct {
	kqfd    int
	changes []syscall.Kevent_t // pending kqueue changes
	kevents []syscall.Kevent_t // reusable kevent result buffer

	recvBufs map[int32]recvInfo
	sendBufs map[int32][]byte

	listenFd int

	pipeFd  int    // read end of wake pipe
	pipeW   int    // write end of wake pipe for PostWake
	pipeBuf []byte
}

type recvInfo struct {
	buf    []byte
	offset int
}

func newEngine() engine {
	return &kqueueEngine{
		recvBufs: make(map[int32]recvInfo, 1024),
		sendBufs: make(map[int32][]byte, 1024),
	}
}

func (e *kqueueEngine) Init(cfg engineConfig) error {
	kqfd, err := syscall.Kqueue()
	if err != nil {
		return err
	}
	e.kqfd = kqfd

	sz := int(cfg.RingSize)
	if sz == 0 {
		sz = 4096
	}
	e.kevents = make([]syscall.Kevent_t, sz)
	e.changes = make([]syscall.Kevent_t, 0, 64) // pre-allocate
	e.pipeW = cfg.PipeW
	return nil
}

func (e *kqueueEngine) PostWake() {
	syscall.Write(e.pipeW, []byte{1})
}

func (e *kqueueEngine) SubmitAccept(listenFd int) {
	e.listenFd = listenFd
	e.changes = append(e.changes, syscall.Kevent_t{
		Ident:  uint64(listenFd),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD | syscall.EV_ONESHOT,
	})
}

func (e *kqueueEngine) SubmitRecv(fd int32, buf []byte, offset int) {
	e.recvBufs[fd] = recvInfo{buf: buf, offset: offset}
	e.changes = append(e.changes, syscall.Kevent_t{
		Ident:  uint64(fd),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD | syscall.EV_ONESHOT,
	})
}

func (e *kqueueEngine) SubmitSend(fd int32, data []byte) {
	e.sendBufs[fd] = data
	e.changes = append(e.changes, syscall.Kevent_t{
		Ident:  uint64(fd),
		Filter: syscall.EVFILT_WRITE,
		Flags:  syscall.EV_ADD | syscall.EV_ONESHOT,
	})
}

func (e *kqueueEngine) SubmitClose(fd int32) {
	delete(e.recvBufs, fd)
	delete(e.sendBufs, fd)
	syscall.Close(int(fd))
}

func (e *kqueueEngine) SubmitPipeRecv(pipeFd int, buf []byte) {
	e.pipeFd = pipeFd
	e.pipeBuf = buf
	e.changes = append(e.changes, syscall.Kevent_t{
		Ident:  uint64(pipeFd),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD | syscall.EV_ONESHOT,
	})
}

func (e *kqueueEngine) Wait(events []event) (int, error) {
	var changes []syscall.Kevent_t
	if len(e.changes) > 0 {
		changes = e.changes
		e.changes = e.changes[:0]
	}

	maxEvents := len(e.kevents)
	if maxEvents > len(events) {
		maxEvents = len(events)
	}

	n, err := syscall.Kevent(e.kqfd, changes, e.kevents[:maxEvents], nil)
	if err != nil {
		if err == syscall.EINTR {
			return 0, nil
		}
		return 0, err
	}

	count := 0
	for i := 0; i < n; i++ {
		kev := &e.kevents[i]
		fd := int32(kev.Ident)

		if kev.Flags&syscall.EV_ERROR != 0 {
			events[count] = event{Op: opRecv, Fd: fd, Res: -1}
			count++
			continue
		}

		// Accept — listen socket readable (EV_CLEAR: stays registered).
		if int(kev.Ident) == e.listenFd && kev.Filter == syscall.EVFILT_READ {
			nfd, _, err := syscall.Accept(e.listenFd)
			if err != nil {
				events[count] = event{Op: opAccept, Fd: 0, Res: -1}
			} else {
				syscall.SetNonblock(nfd, true)
				syscall.CloseOnExec(nfd)
				events[count] = event{Op: opAccept, Fd: 0, Res: int32(nfd)}
			}
			count++
			continue
		}

		// Pipe wakeup (EV_CLEAR: stays registered).
		if int(kev.Ident) == e.pipeFd && kev.Filter == syscall.EVFILT_READ {
			syscall.Read(e.pipeFd, e.pipeBuf)
			events[count] = event{Op: opWake, Fd: 0, Res: 1}
			count++
			continue
		}

		// Read event.
		if kev.Filter == syscall.EVFILT_READ {
			info, ok := e.recvBufs[fd]
			if !ok {
				continue
			}
			delete(e.recvBufs, fd)
			nr, err := syscall.Read(int(fd), info.buf[info.offset:])
			if err != nil || nr <= 0 {
				if nr == 0 {
					events[count] = event{Op: opRecv, Fd: fd, Res: 0}
				} else {
					events[count] = event{Op: opRecv, Fd: fd, Res: -1}
				}
			} else {
				events[count] = event{Op: opRecv, Fd: fd, Res: int32(nr)}
			}
			count++
			continue
		}

		// Write event.
		if kev.Filter == syscall.EVFILT_WRITE {
			data, ok := e.sendBufs[fd]
			if !ok {
				continue
			}
			delete(e.sendBufs, fd)
			nw, err := syscall.Write(int(fd), data)
			if err != nil || nw < 0 {
				events[count] = event{Op: opSend, Fd: fd, Res: -1}
			} else {
				events[count] = event{Op: opSend, Fd: fd, Res: int32(nw)}
			}
			count++
			continue
		}
	}

	return count, nil
}

func (e *kqueueEngine) Flush() error {
	if len(e.changes) > 0 {
		_, err := syscall.Kevent(e.kqfd, e.changes, nil, nil)
		e.changes = e.changes[:0]
		return err
	}
	return nil
}

func (e *kqueueEngine) Close() {
	if e.kqfd > 0 {
		syscall.Close(e.kqfd)
	}
}
