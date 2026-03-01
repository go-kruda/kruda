package wing

// engine abstracts the OS-specific async I/O backend.
// On Linux: io_uring (completion-based).
// On macOS: kqueue (readiness-based, engine does syscall internally).
// On Windows: IOCP (completion-based).
type engine interface {
	// Init sets up the I/O backend.
	Init(cfg engineConfig) error

	// SubmitAccept arms an accept on the listen fd.
	SubmitAccept(listenFd int)

	// SubmitRecv arms a read on fd into buf[offset:].
	SubmitRecv(fd int32, buf []byte, offset int)

	// SubmitSend arms a write of data to fd.
	SubmitSend(fd int32, data []byte)

	// SubmitClose arms a close on fd.
	SubmitClose(fd int32)

	// SubmitPipeRecv arms a read on the wakeup pipe fd.
	// No-op on Windows (IOCP uses PostQueuedCompletionStatus instead).
	SubmitPipeRecv(pipeFd int, buf []byte)

	// PostWake wakes the event loop from another goroutine.
	// On Linux/macOS: writes to the wake pipe.
	// On Windows: calls PostQueuedCompletionStatus.
	PostWake()

	// Wait blocks until at least one event completes.
	// Writes completed events into the provided slice and returns count.
	Wait(events []event) (int, error)

	// Flush submits all pending operations to the kernel.
	Flush() error

	// Close tears down the engine and frees resources.
	Close()
}

// engineConfig holds engine initialization parameters.
type engineConfig struct {
	RingSize uint32 // io_uring: SQE entries; kqueue: initial event list capacity; IOCP: ignored
	PipeW    int    // write end of wake pipe (-1 on Windows, which uses PostWake directly)
}

// event is a completed I/O event from the kernel.
type event struct {
	Op  uint8 // operation type
	Fd  int32 // file descriptor
	Res int32 // bytes transferred (>0) or negative errno
}

// Operation types for events.
const (
	opAccept uint8 = iota + 1
	opRecv
	opSend
	opClose
	opWake
)
