//go:build windows

package wing

import (
	"syscall"
	"unsafe"
)

// SO_UPDATE_ACCEPT_CONTEXT must be set on accepted sockets after AcceptEx
// so that shutdown/setsockopt/etc. work correctly on the accepted socket.
const soUpdateAcceptContext = 0x700B

// acceptExAddrLen is sizeof(sockaddr_in6) + 16, large enough for both
// IPv4 and IPv6 addresses as required by AcceptEx.
const acceptExAddrLen = 28 + 16 // = 44

// iocpWakeKey is the completion key used by PostWake to signal the event loop.
// Must be uintptr to match Go's syscall signatures (ULONG_PTR on Windows).
const iocpWakeKey uint32 = 0xFFFF

// iocpOp represents an in-flight overlapped I/O operation.
// The overlapped field MUST be first for safe pointer arithmetic if needed.
// All iocpOp instances are stored in the ops map to prevent GC collection
// while the kernel holds a reference to the Overlapped.
type iocpOp struct {
	ovlp       syscall.Overlapped
	op         uint8
	fd         int32
	stale      bool           // marked true when SubmitClose invalidates pending ops
	wsabuf     syscall.WSABuf // for WSARecv/WSASend
	bufRef     []byte         // prevents GC of the Go slice backing the WSABuf
	flags      uint32         // receive flags for WSARecv
	acceptSock syscall.Handle // pre-created socket for AcceptEx
	acceptBuf  [128]byte      // AcceptEx address output buffer (>= 2 * acceptExAddrLen)
}

// iocpEngine implements the engine interface using Windows IOCP.
type iocpEngine struct {
	port     syscall.Handle // IOCP handle
	ops      map[*syscall.Overlapped]*iocpOp
	listenFd syscall.Handle // cached listen socket for AcceptEx / SO_UPDATE_ACCEPT_CONTEXT
	family   int            // AF_INET or AF_INET6, detected from listen socket
}

func newEngine() engine {
	return &iocpEngine{
		ops: make(map[*syscall.Overlapped]*iocpOp, 1024),
	}
}

func (e *iocpEngine) Init(cfg engineConfig) error {
	// Create an IOCP with 1 concurrent thread (we use LockOSThread per worker).
	port, err := syscall.CreateIoCompletionPort(syscall.InvalidHandle, 0, 0, 1)
	if err != nil {
		return err
	}
	e.port = port
	return nil
}

func (e *iocpEngine) SubmitAccept(listenFd int) {
	h := syscall.Handle(listenFd)

	// First call: associate listen socket with IOCP and detect address family.
	if e.listenFd == 0 {
		e.listenFd = h
		e.family = getSocketFamily(h)
		if _, err := syscall.CreateIoCompletionPort(h, e.port, 0, 0); err != nil {
			return
		}
	}

	// Create a new socket for AcceptEx (must match listen socket family).
	sock, err := syscall.Socket(e.family, syscall.SOCK_STREAM, 0)
	if err != nil {
		return
	}

	op := &iocpOp{
		op:         opAccept,
		acceptSock: sock,
	}
	e.ops[&op.ovlp] = op

	var received uint32
	err = syscall.AcceptEx(
		h, sock,
		&op.acceptBuf[0],
		0,                // no data receive
		acceptExAddrLen,  // local address length
		acceptExAddrLen,  // remote address length
		&received,
		&op.ovlp,
	)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		syscall.Closesocket(sock)
		delete(e.ops, &op.ovlp)
	}
}

func (e *iocpEngine) SubmitRecv(fd int32, buf []byte, offset int) {
	op := &iocpOp{
		op:     opRecv,
		fd:     fd,
		bufRef: buf,
	}
	op.wsabuf.Buf = &buf[offset]
	op.wsabuf.Len = uint32(len(buf) - offset)
	e.ops[&op.ovlp] = op

	var received uint32
	err := syscall.WSARecv(
		syscall.Handle(fd),
		&op.wsabuf,
		1,
		&received,
		&op.flags,
		&op.ovlp,
		nil,
	)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		delete(e.ops, &op.ovlp)
	}
}

func (e *iocpEngine) SubmitSend(fd int32, data []byte) {
	if len(data) == 0 {
		return // nothing to send
	}
	op := &iocpOp{
		op:     opSend,
		fd:     fd,
		bufRef: data,
	}
	op.wsabuf.Buf = &data[0]
	op.wsabuf.Len = uint32(len(data))
	e.ops[&op.ovlp] = op

	var sent uint32
	err := syscall.WSASend(
		syscall.Handle(fd),
		&op.wsabuf,
		1,
		&sent,
		0, // flags
		&op.ovlp,
		nil,
	)
	if err != nil && err != syscall.ERROR_IO_PENDING {
		delete(e.ops, &op.ovlp)
	}
}

func (e *iocpEngine) SubmitClose(fd int32) {
	// Mark all pending operations for this fd as stale so that when their
	// cancelled completions arrive they are silently discarded. This prevents
	// a race where a recycled handle receives a stale completion meant for
	// the old socket.
	for _, op := range e.ops {
		if op.fd == fd {
			op.stale = true
		}
	}
	syscall.Closesocket(syscall.Handle(fd))
}

// SubmitPipeRecv is a no-op on Windows.
// IOCP uses PostQueuedCompletionStatus for wake instead of pipes.
func (e *iocpEngine) SubmitPipeRecv(_ int, _ []byte) {}

func (e *iocpEngine) PostWake() {
	syscall.PostQueuedCompletionStatus(e.port, 0, iocpWakeKey, nil)
}

func (e *iocpEngine) Wait(events []event) (int, error) {
	count := 0
	timeout := uint32(syscall.INFINITE)

	for count < len(events) {
		var bytes uint32
		var key uint32
		var ovlp *syscall.Overlapped

		err := syscall.GetQueuedCompletionStatus(e.port, &bytes, &key, &ovlp, timeout)

		// After the first event, drain remaining with non-blocking poll.
		if count > 0 {
			timeout = 0
		}

		// Wake signal from PostQueuedCompletionStatus (ovlp == nil, key == wake).
		if key == iocpWakeKey && ovlp == nil && err == nil {
			events[count] = event{Op: opWake, Fd: 0, Res: 1}
			count++
			timeout = 0
			continue
		}

		// No overlapped: timeout or port error.
		if ovlp == nil {
			if count > 0 {
				return count, nil
			}
			if err != nil {
				return 0, err
			}
			return 0, nil
		}

		// Look up the operation. If unknown or stale, discard and continue.
		op, ok := e.ops[ovlp]
		if !ok {
			timeout = 0
			continue
		}
		delete(e.ops, ovlp)

		if op.stale {
			timeout = 0
			continue
		}

		// I/O error (socket closed, cancelled, etc.).
		if err != nil {
			events[count] = event{Op: op.op, Fd: op.fd, Res: -1}
			count++
			timeout = 0
			continue
		}

		// Successful completion — translate to generic event.
		switch op.op {
		case opAccept:
			// Associate accepted socket with IOCP.
			if _, e2 := syscall.CreateIoCompletionPort(op.acceptSock, e.port, 0, 0); e2 != nil {
				syscall.Closesocket(op.acceptSock)
				events[count] = event{Op: opAccept, Fd: 0, Res: -1}
			} else {
				// Set SO_UPDATE_ACCEPT_CONTEXT so the accepted socket inherits
				// listen socket properties (required by Winsock).
				lh := e.listenFd
				syscall.Setsockopt(op.acceptSock, syscall.SOL_SOCKET, soUpdateAcceptContext,
					(*byte)(unsafe.Pointer(&lh)), int32(unsafe.Sizeof(lh)))
				events[count] = event{Op: opAccept, Fd: 0, Res: int32(op.acceptSock)}
			}

		case opRecv:
			// bytes == 0 means graceful close (EOF).
			events[count] = event{Op: opRecv, Fd: op.fd, Res: int32(bytes)}

		case opSend:
			events[count] = event{Op: opSend, Fd: op.fd, Res: int32(bytes)}
		}

		count++
		timeout = 0
	}

	return count, nil
}

// Flush is a no-op on Windows.
// IOCP operations (WSARecv, WSASend, AcceptEx) are submitted directly to the
// kernel — there is no submission queue to flush.
func (e *iocpEngine) Flush() error { return nil }

func (e *iocpEngine) Close() {
	if e.port != 0 {
		syscall.CloseHandle(e.port)
	}
}

// getSocketFamily detects the address family (AF_INET/AF_INET6) of a socket.
func getSocketFamily(fd syscall.Handle) int {
	sa, err := syscall.Getsockname(fd)
	if err != nil {
		return syscall.AF_INET
	}
	switch sa.(type) {
	case *syscall.SockaddrInet6:
		return syscall.AF_INET6
	default:
		return syscall.AF_INET
	}
}
