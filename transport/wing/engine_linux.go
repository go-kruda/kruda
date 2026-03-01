//go:build linux

package wing

import (
	"syscall"
	"unsafe"
)

// User-data encoding: upper 8 bits = operation, lower 56 bits = fd.
const (
	udAccept uint64 = 1 << 56
	udRecv   uint64 = 2 << 56
	udSend   uint64 = 3 << 56
	udClose  uint64 = 4 << 56
	udWake   uint64 = 5 << 56
)

// ioUringEngine wraps Ring behind the engine interface.
type ioUringEngine struct {
	ring  *Ring
	pipeW int // write end of wake pipe for PostWake
}

func newEngine() engine {
	return &ioUringEngine{}
}

func (e *ioUringEngine) Init(cfg engineConfig) error {
	ring, err := NewRing(cfg.RingSize)
	if err != nil {
		return err
	}
	e.ring = ring
	e.pipeW = cfg.PipeW
	return nil
}

func (e *ioUringEngine) PostWake() {
	syscall.Write(e.pipeW, []byte{1})
}

func (e *ioUringEngine) SubmitAccept(listenFd int) {
	sqe := e.getSQE()
	if sqe == nil {
		return
	}
	sqe.PrepareAccept(listenFd, syscall.SOCK_NONBLOCK|syscall.SOCK_CLOEXEC)
	sqe.UserData = udAccept
}

func (e *ioUringEngine) SubmitRecv(fd int32, buf []byte, offset int) {
	sqe := e.getSQE()
	if sqe == nil {
		return
	}
	sqe.PrepareRecv(int(fd), unsafe.Pointer(&buf[offset]), uint32(len(buf)-offset))
	sqe.UserData = udRecv | uint64(fd)
}

func (e *ioUringEngine) SubmitSend(fd int32, data []byte) {
	sqe := e.getSQE()
	if sqe == nil {
		return
	}
	sqe.PrepareSend(int(fd), unsafe.Pointer(&data[0]), uint32(len(data)))
	sqe.UserData = udSend | uint64(fd)
}

func (e *ioUringEngine) SubmitClose(fd int32) {
	sqe := e.ring.GetSQE() // direct, avoid recursion via getSQE
	if sqe == nil {
		// Last resort: synchronous close.
		syscall.Close(int(fd))
		return
	}
	sqe.PrepareClose(int(fd))
	sqe.UserData = udClose | uint64(fd)
}

func (e *ioUringEngine) SubmitPipeRecv(pipeFd int, buf []byte) {
	sqe := e.getSQE()
	if sqe == nil {
		return
	}
	sqe.PrepareRecv(pipeFd, unsafe.Pointer(&buf[0]), uint32(len(buf)))
	sqe.UserData = udWake
}

func (e *ioUringEngine) Wait(events []event) (int, error) {
	// Block until at least one CQE is ready.
	cqe, err := e.ring.WaitCQE()
	if err != nil {
		return 0, err
	}

	// Drain all available CQEs into events slice.
	n := 0
	for cqe != nil && n < len(events) {
		events[n] = decodeCQE(cqe)
		e.ring.SeenCQE()
		n++
		cqe = e.ring.PeekCQE()
	}
	return n, nil
}

func (e *ioUringEngine) Flush() error {
	_, err := e.ring.Submit()
	return err
}

func (e *ioUringEngine) Close() {
	if e.ring != nil {
		e.ring.Close()
	}
}

// getSQE returns an SQE, flushing the ring if full (one retry).
func (e *ioUringEngine) getSQE() *SQE {
	if sqe := e.ring.GetSQE(); sqe != nil {
		return sqe
	}
	// SQ full — flush pending submissions, then retry.
	e.ring.Submit()
	return e.ring.GetSQE()
}

// decodeCQE translates a kernel CQE into a generic event.
func decodeCQE(cqe *CQE) event {
	ud := cqe.UserData
	opBits := ud & (0xFF << 56)
	fd := int32(ud & 0x00FFFFFFFFFFFFFF)

	var op uint8
	switch opBits {
	case udAccept:
		op = opAccept
	case udRecv:
		op = opRecv
	case udSend:
		op = opSend
	case udClose:
		op = opClose
	case udWake:
		op = opWake
	}
	return event{Op: op, Fd: fd, Res: cqe.Res}
}
