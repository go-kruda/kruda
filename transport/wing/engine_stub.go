//go:build !linux && !darwin

package wing

import (
	"fmt"
	"unsafe"
)

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

type stubEngine struct{}

func newEngine() engine { return &stubEngine{} }

func (e *stubEngine) Init(_ engineConfig) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}
func (e *stubEngine) SubmitAccept(_ int)                     {}
func (e *stubEngine) SubmitRecv(_ int32, _ []byte, _ int)    {}
func (e *stubEngine) SubmitSend(_ int32, _ []byte)           {}
func (e *stubEngine) SubmitClose(_ int32)                    {}
func (e *stubEngine) Detach(_ int32)                         {}
func (e *stubEngine) SubmitPipeRecv(_ int, _ []byte)         {}
func (e *stubEngine) RegisterConn(_ int32, _ unsafe.Pointer) {}
func (e *stubEngine) PostWake()                              {}
func (e *stubEngine) Wait(_ []event) (int, error)            { return 0, fmt.Errorf("wing: unsupported platform") }
func (e *stubEngine) WaitNonBlock(_ []event) (int, error)    { return 0, nil }
func (e *stubEngine) Flush() error                           { return nil }
func (e *stubEngine) Close()                                 {}
