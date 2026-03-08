//go:build !linux && !darwin

package wing

import "unsafe"

// Type stubs so code referencing these types compiles cross-platform.

type engine interface {
	Init(cfg engineConfig) error
	SubmitAccept(listenFd int)
	SubmitRecv(fd int32, buf []byte, offset int)
	SubmitSend(fd int32, data []byte)
	SubmitClose(fd int32)
	Detach(fd int32)
	SubmitPipeRecv(pipeFd int, buf []byte)
	RegisterConn(fd int32, ptr unsafe.Pointer)
	PostWake()
	Wait(events []event) (int, error)
	WaitNonBlock(events []event) (int, error)
	Flush() error
	Close()
}

type engineConfig struct {
	RingSize uint32
	PipeW    int
	EventFd  int
	RawMode  bool
}

type event struct {
	Op      uint8
	Fd      int32
	Res     int32
	Flags   uint32
	ConnPtr unsafe.Pointer
}
