//go:build linux || darwin

package wing

import "unsafe"

// engine abstracts the OS-specific async I/O backend.
// On Linux: epoll (event-driven).
// On macOS: kqueue (readiness-based, engine does syscall internally).
type engine interface {
	Init(cfg engineConfig) error
	SubmitAccept(listenFd int)
	SubmitRecv(fd int32, buf []byte, offset int)
	SubmitSend(fd int32, data []byte)
	SubmitClose(fd int32)
	Detach(fd int32) // remove fd from poll without closing it
	SubmitPipeRecv(pipeFd int, buf []byte)
	RegisterConn(fd int32, ptr unsafe.Pointer) // store *conn for pointer-in-epoll
	PostWake()
	Wait(events []event) (int, error)
	WaitNonBlock(events []event) (int, error)
	Flush() error
	Close()
}

// engineConfig holds engine initialization parameters.
type engineConfig struct {
	RingSize uint32
	PipeW    int // legacy (darwin)
	EventFd  int // eventfd for wake (linux)
	RawMode  bool // use RawSyscall for epoll_wait (requires LockOSThread)
}

// event is a completed I/O event from the kernel.
type event struct {
	Op      uint8          // operation type
	Fd      int32          // file descriptor
	Res     int32          // bytes transferred (>0) or negative errno
	Flags   uint32         // CQE flags (e.g. IORING_CQE_F_MORE for multishot)
	ConnPtr unsafe.Pointer // *conn pointer (epoll data, avoids map lookup)
}

// Operation types for events.
const (
	opAccept uint8 = iota + 1
	opRecv
	opSend
	opClose
	opWake
)

// cqeFMore is set in event.Flags when a multishot operation will produce more completions.
// On non-Linux backends (kqueue) this is always 0, so re-arm happens every time.
const cqeFMore uint32 = 1 << 1
