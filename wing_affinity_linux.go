//go:build linux

package kruda

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	linuxCPUSetBytes = 128
)

func pinCurrentThreadToCPU(workerID int) error {
	cpu, ok := wingAffinityCPU(workerID)
	if !ok {
		return nil
	}
	var set [linuxCPUSetBytes]byte
	set[cpu/8] |= 1 << uint(cpu%8)
	_, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_SETAFFINITY, 0, uintptr(len(set)), uintptr(unsafe.Pointer(&set[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func wingAffinityCPU(workerID int) (int, bool) {
	var mask [linuxCPUSetBytes]byte
	_, _, errno := syscall.RawSyscall(syscall.SYS_SCHED_GETAFFINITY, 0, uintptr(len(mask)), uintptr(unsafe.Pointer(&mask[0])))
	if errno != 0 {
		n := runtime.NumCPU()
		if n <= 0 {
			return 0, false
		}
		return workerID % n, true
	}

	allowed := 0
	for cpu := 0; cpu < len(mask)*8; cpu++ {
		if mask[cpu/8]&(1<<uint(cpu%8)) != 0 {
			allowed++
		}
	}
	if allowed == 0 {
		return 0, false
	}

	want := workerID % allowed
	seen := 0
	for cpu := 0; cpu < len(mask)*8; cpu++ {
		if mask[cpu/8]&(1<<uint(cpu%8)) == 0 {
			continue
		}
		if seen == want {
			return cpu, true
		}
		seen++
	}
	return 0, false
}
