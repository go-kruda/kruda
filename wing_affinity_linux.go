//go:build linux

package kruda

import (
	"runtime"
	"syscall"
	"unsafe"
)

func pinWorkerThread(workerID int) {
	cpus := runtime.NumCPU()
	if cpus <= 0 {
		return
	}
	cpu := workerID % cpus
	var mask [128]byte
	mask[cpu/8] = 1 << uint(cpu%8)
	_, _, errno := syscall.RawSyscall(
		syscall.SYS_SCHED_SETAFFINITY,
		0,
		uintptr(len(mask)),
		uintptr(unsafe.Pointer(&mask[0])),
	)
	_ = errno
}
