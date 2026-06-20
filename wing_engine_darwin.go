//go:build darwin

package kruda

import (
	"net/netip"
	"strconv"
	"syscall"
	"unsafe"
)

type kqueueEngine struct {
	kqfd    int
	changes []syscall.Kevent_t
	kevents []syscall.Kevent_t

	recvBufs map[int32]recvInfo
	sendBufs map[int32][]byte

	listenFd int
	pipeR    int
	pipeW    int
	pipeBuf  [8]byte
}

type recvInfo struct {
	buf    []byte
	offset int
}

func newEngine() engine {
	return &kqueueEngine{
		recvBufs: make(map[int32]recvInfo, 1024),
		sendBufs: make(map[int32][]byte, 1024),
		pipeR:    -1,
		pipeW:    -1,
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
	e.changes = make([]syscall.Kevent_t, 0, 64)

	// Internal wake pipe — not exposed to transport.
	r, w, err := createPipe()
	if err != nil {
		syscall.Close(kqfd)
		return err
	}
	e.pipeR = r
	e.pipeW = w

	// Register pipe read end with kqueue.
	e.changes = append(e.changes, syscall.Kevent_t{
		Ident:  uint64(r),
		Filter: syscall.EVFILT_READ,
		Flags:  syscall.EV_ADD,
	})
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

func (e *kqueueEngine) Detach(fd int32) {
	delete(e.recvBufs, fd)
	delete(e.sendBufs, fd)
}

func (e *kqueueEngine) SubmitPipeRecv(_ int, _ []byte) {}

func (e *kqueueEngine) RegisterConn(_ int32, _ unsafe.Pointer) {}

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

		if int(kev.Ident) == e.listenFd && kev.Filter == syscall.EVFILT_READ {
			// Raw accept(2) with a stack-allocated RawSockaddrAny so the peer
			// address can be captured without syscall.Accept's per-call boxed
			// Sockaddr, which would heap-allocate on the accept path. Darwin's
			// accept() has no flags argument, so non-block/cloexec are set after.
			var rsa syscall.RawSockaddrAny
			salen := socklen(unsafe.Sizeof(rsa))
			r1, _, errno := syscall.Syscall(syscall.SYS_ACCEPT,
				uintptr(e.listenFd),
				uintptr(unsafe.Pointer(&rsa)),
				uintptr(unsafe.Pointer(&salen)))
			if errno != 0 {
				events[count] = event{Op: opAccept, Fd: 0, Res: -1}
			} else {
				nfd := int(r1)
				ip, ok := parseRawSockaddr(&rsa)
				syscall.SetNonblock(nfd, true)
				syscall.CloseOnExec(nfd)
				events[count] = event{Op: opAccept, Fd: 0, Res: int32(nfd), PeerIP: ip, HasPeer: ok}
			}
			count++
			continue
		}

		// Internal wake pipe — drain and re-arm, emit nothing.
		if int(kev.Ident) == e.pipeR && kev.Filter == syscall.EVFILT_READ {
			syscall.Read(e.pipeR, e.pipeBuf[:])
			e.changes = append(e.changes, syscall.Kevent_t{
				Ident:  uint64(e.pipeR),
				Filter: syscall.EVFILT_READ,
				Flags:  syscall.EV_ADD,
			})
			continue
		}

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
		}
	}

	return count, nil
}

func (e *kqueueEngine) WaitNonBlock(events []event) (int, error) { return e.Wait(events) }

func (e *kqueueEngine) Flush() error {
	if len(e.changes) > 0 {
		_, err := syscall.Kevent(e.kqfd, e.changes, nil, nil)
		e.changes = e.changes[:0]
		return err
	}
	return nil
}

func (e *kqueueEngine) Close() {
	if e.pipeR >= 0 {
		syscall.Close(e.pipeR)
	}
	if e.pipeW >= 0 {
		syscall.Close(e.pipeW)
	}
	if e.kqfd > 0 {
		syscall.Close(e.kqfd)
	}
}

// socklen mirrors the kernel socklen_t passed to accept(2). syscall's own
// _Socklen is unexported, so we declare a local alias.
type socklen = uint32

// parseRawSockaddr extracts the peer IP from a kernel sockaddr without heap
// allocation (no interface boxing, unlike syscall.Accept's Sockaddr return).
// The IPv4 path is zero-alloc; the rare IPv6+scope path may allocate in WithZone.
func parseRawSockaddr(rsa *syscall.RawSockaddrAny) (netip.Addr, bool) {
	switch rsa.Addr.Family {
	case syscall.AF_INET:
		p := (*syscall.RawSockaddrInet4)(unsafe.Pointer(rsa))
		return netip.AddrFrom4(p.Addr), true
	case syscall.AF_INET6:
		p := (*syscall.RawSockaddrInet6)(unsafe.Pointer(rsa))
		a := netip.AddrFrom16(p.Addr)
		if p.Scope_id != 0 {
			a = a.WithZone(strconv.FormatUint(uint64(p.Scope_id), 10))
		}
		return a, true
	}
	return netip.Addr{}, false
}
