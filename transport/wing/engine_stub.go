//go:build !linux && !darwin

package wing

import (
	"fmt"
	"unsafe"
)

type stubEngine struct{}

func newEngine() engine {
	return &stubEngine{}
}

func (e *stubEngine) Init(_ engineConfig) error {
	return fmt.Errorf("wing: unsupported platform; use FastHTTP or NetHTTP transport")
}

func (e *stubEngine) SubmitAccept(_ int)                  {}
func (e *stubEngine) SubmitRecv(_ int32, _ []byte, _ int) {}
func (e *stubEngine) SubmitSend(_ int32, _ []byte)        {}
func (e *stubEngine) SubmitClose(_ int32)                 {}
func (e *stubEngine) SubmitPipeRecv(_ int, _ []byte)      {}
func (e *stubEngine) RegisterConn(_ int32, _ unsafe.Pointer) {}
func (e *stubEngine) PostWake()                           {}
func (e *stubEngine) Wait(_ []event) (int, error)         { return 0, fmt.Errorf("wing: unsupported platform") }
func (e *stubEngine) WaitNonBlock(_ []event) (int, error) { return 0, nil }
func (e *stubEngine) Flush() error                        { return nil }
func (e *stubEngine) Close()                              {}
