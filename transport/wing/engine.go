package wing

// engine abstracts the OS-specific async I/O backend.
// On Linux: io_uring (completion-based).
// On macOS: kqueue (readiness-based, engine does syscall internally).
type engine interface {
	Init(cfg engineConfig) error
	SubmitAccept(listenFd int)
	SubmitRecv(fd int32, buf []byte, offset int)
	SubmitSend(fd int32, data []byte)
	SubmitClose(fd int32)
	SubmitPipeRecv(pipeFd int, buf []byte)
	PostWake()
	Wait(events []event) (int, error)
	Flush() error
	Close()
}

// engineConfig holds engine initialization parameters.
type engineConfig struct {
	RingSize uint32 // io_uring: SQE entries; kqueue: initial event list capacity
	PipeW    int    // write end of wake pipe
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
