//go:build linux

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	ioringOffSQRing = 0
	ioringOffCQRing = 0x8000000
	ioringOffSQEs   = 0x10000000

	ioringEnterGetEvents = 1
	ioringOpNop          = 0
)

type ioSqringOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	flags       uint32
	dropped     uint32
	array       uint32
	resv1       uint32
	userAddr    uint64
}

type ioCqringOffsets struct {
	head        uint32
	tail        uint32
	ringMask    uint32
	ringEntries uint32
	overflow    uint32
	cqes        uint32
	flags       uint32
	resv1       uint32
	userAddr    uint64
}

type ioUringParams struct {
	sqEntries    uint32
	cqEntries    uint32
	flags        uint32
	sqThreadCPU  uint32
	sqThreadIdle uint32
	features     uint32
	wqFD         uint32
	resv         [3]uint32
	sqOff        ioSqringOffsets
	cqOff        ioCqringOffsets
}

type ioUringSqe struct {
	opcode      uint8
	flags       uint8
	ioprio      uint16
	fd          int32
	off         uint64
	addr        uint64
	len         uint32
	rwFlags     uint32
	userData    uint64
	bufIndex    uint16
	personality uint16
	spliceFdIn  int32
	addr3       uint64
	pad2        [1]uint64
}

type ioUringCqe struct {
	userData uint64
	res      int32
	flags    uint32
}

type ring struct {
	fd     int
	params ioUringParams
	sqRing []byte
	cqRing []byte
	sqes   []byte
}

func main() {
	entries := flag.Uint("entries", 64, "io_uring queue entries")
	nops := flag.Int("nops", 1000, "NOP operations to submit and complete")
	flag.Parse()

	if *entries == 0 || *nops <= 0 {
		fmt.Fprintln(os.Stderr, "entries and nops must be positive")
		os.Exit(2)
	}

	r, err := setupRing(uint32(*entries))
	if err != nil {
		fmt.Fprintf(os.Stderr, "io_uring_setup: %v\n", err)
		os.Exit(1)
	}
	defer r.close()

	start := time.Now()
	for i := 0; i < *nops; i++ {
		if err := r.submitNOP(uint64(i + 1)); err != nil {
			fmt.Fprintf(os.Stderr, "submit nop %d: %v\n", i+1, err)
			os.Exit(1)
		}
	}
	elapsed := time.Since(start)

	fmt.Printf("io_uring_probe=ok\n")
	fmt.Printf("entries=%d\n", r.params.sqEntries)
	fmt.Printf("cq_entries=%d\n", r.params.cqEntries)
	fmt.Printf("nops=%d\n", *nops)
	fmt.Printf("elapsed_ms=%.3f\n", float64(elapsed.Microseconds())/1000.0)
	fmt.Printf("nop_per_sec=%.2f\n", float64(*nops)/elapsed.Seconds())
}

func setupRing(entries uint32) (*ring, error) {
	var p ioUringParams
	fd, _, errno := syscall.RawSyscall(
		uintptr(unix.SYS_IO_URING_SETUP),
		uintptr(entries),
		uintptr(unsafe.Pointer(&p)),
		0,
	)
	if errno != 0 {
		return nil, errno
	}

	r := &ring{fd: int(fd), params: p}
	var err error
	sqRingSize := int(p.sqOff.array) + int(p.sqEntries)*4
	cqRingSize := int(p.cqOff.cqes) + int(p.cqEntries)*int(unsafe.Sizeof(ioUringCqe{}))
	sqesSize := int(p.sqEntries) * int(unsafe.Sizeof(ioUringSqe{}))

	r.sqRing, err = unix.Mmap(r.fd, ioringOffSQRing, sqRingSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap sq ring: %w", err)
	}
	r.cqRing, err = unix.Mmap(r.fd, ioringOffCQRing, cqRingSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Munmap(r.sqRing)
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap cq ring: %w", err)
	}
	r.sqes, err = unix.Mmap(r.fd, ioringOffSQEs, sqesSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED|unix.MAP_POPULATE)
	if err != nil {
		_ = unix.Munmap(r.cqRing)
		_ = unix.Munmap(r.sqRing)
		_ = unix.Close(r.fd)
		return nil, fmt.Errorf("mmap sqes: %w", err)
	}
	return r, nil
}

func (r *ring) submitNOP(userData uint64) error {
	sqHead := r.u32(r.sqRing, r.params.sqOff.head)
	sqTail := r.u32(r.sqRing, r.params.sqOff.tail)
	sqMask := r.u32(r.sqRing, r.params.sqOff.ringMask)
	sqEntries := r.u32(r.sqRing, r.params.sqOff.ringEntries)

	head := atomic.LoadUint32(sqHead)
	tail := atomic.LoadUint32(sqTail)
	if tail-head >= atomic.LoadUint32(sqEntries) {
		return fmt.Errorf("submission queue full")
	}

	idx := tail & atomic.LoadUint32(sqMask)
	sqe := r.sqe(idx)
	*sqe = ioUringSqe{opcode: ioringOpNop, userData: userData}
	*r.u32(r.sqRing, r.params.sqOff.array+idx*4) = idx
	atomic.StoreUint32(sqTail, tail+1)

	_, _, errno := syscall.RawSyscall6(
		uintptr(unix.SYS_IO_URING_ENTER),
		uintptr(r.fd),
		1,
		1,
		ioringEnterGetEvents,
		0,
		0,
	)
	if errno != 0 {
		return errno
	}
	return r.waitCQE(userData)
}

func (r *ring) waitCQE(userData uint64) error {
	cqHead := r.u32(r.cqRing, r.params.cqOff.head)
	cqTail := r.u32(r.cqRing, r.params.cqOff.tail)
	cqMask := r.u32(r.cqRing, r.params.cqOff.ringMask)

	for spins := 0; ; spins++ {
		head := atomic.LoadUint32(cqHead)
		tail := atomic.LoadUint32(cqTail)
		if head != tail {
			idx := head & atomic.LoadUint32(cqMask)
			cqe := r.cqe(idx)
			if cqe.userData != userData {
				return fmt.Errorf("cqe user_data=%d, want %d", cqe.userData, userData)
			}
			if cqe.res != 0 {
				return syscall.Errno(-cqe.res)
			}
			atomic.StoreUint32(cqHead, head+1)
			return nil
		}
		if spins > 1000 {
			return fmt.Errorf("completion queue timeout")
		}
		runtime.Gosched()
	}
}

func (r *ring) u32(buf []byte, off uint32) *uint32 {
	return (*uint32)(unsafe.Pointer(&buf[off]))
}

func (r *ring) sqe(idx uint32) *ioUringSqe {
	size := unsafe.Sizeof(ioUringSqe{})
	return (*ioUringSqe)(unsafe.Pointer(&r.sqes[uintptr(idx)*size]))
}

func (r *ring) cqe(idx uint32) *ioUringCqe {
	size := unsafe.Sizeof(ioUringCqe{})
	return (*ioUringCqe)(unsafe.Pointer(&r.cqRing[uintptr(r.params.cqOff.cqes)+uintptr(idx)*size]))
}

func (r *ring) close() {
	if r.sqes != nil {
		_ = unix.Munmap(r.sqes)
	}
	if r.cqRing != nil {
		_ = unix.Munmap(r.cqRing)
	}
	if r.sqRing != nil {
		_ = unix.Munmap(r.sqRing)
	}
	if r.fd >= 0 {
		_ = unix.Close(r.fd)
	}
}
