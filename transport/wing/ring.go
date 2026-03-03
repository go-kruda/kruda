//go:build linux

// Package wing provides a Kruda transport backed by platform-native async I/O.
// On Linux: io_uring (minimal, zero-dependency syscall layer).
// On macOS: kqueue.
package wing

import (
	"fmt"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Linux syscall numbers (amd64/arm64).
const (
	sysIOUringSetup    = 425
	sysIOUringEnter    = 426
	sysIOUringRegister = 427
)

// io_uring opcodes (networking subset).
const (
	opIOAccept uint8 = 13
	opIOClose  uint8 = 19
	opIOSend   uint8 = 26
	opIORecv   uint8 = 27
)

// io_uring accept flags.
const iouringAcceptMultishot uint16 = 1 << 0

// io_uring_enter flags.
const iouringEnterGetEvents uint32 = 1

// io_uring_setup flags.
const (
	iouringSetupSingleIssuer uint32 = 1 << 12 // kernel 6.0+
	iouringSetupDeferTaskrun uint32 = 1 << 13 // kernel 6.1+
)

// io_uring_setup feature flags.
const featSingleMmap uint32 = 1

// Mmap offsets for io_uring_setup.
const (
	offSQRing int64 = 0
	offCQRing int64 = 0x8000000
	offSQEs   int64 = 0x10000000
)

// ----------------------------- kernel structs -----------------------------

// ioUringParams mirrors struct io_uring_params.
type ioUringParams struct {
	sqEntries    uint32
	cqEntries    uint32
	flags        uint32
	sqThreadCPU  uint32
	sqThreadIdle uint32
	features     uint32
	wqFd         uint32
	resv         [3]uint32
	sqOff        sqringOffsets
	cqOff        cqringOffsets
}

type sqringOffsets struct {
	head, tail, ringMask, ringEntries uint32
	flags, dropped, array, resv1      uint32
	userAddr                          uint64
}

type cqringOffsets struct {
	head, tail, ringMask, ringEntries uint32
	overflow, cqes, flags, resv1      uint32
	userAddr                          uint64
}

// SQE is a Submission Queue Entry (64 bytes, matches kernel layout).
type SQE struct {
	Opcode      uint8
	Flags       uint8
	Ioprio      uint16
	Fd          int32
	Off         uint64
	Addr        uint64
	Len         uint32
	OpcFlags    uint32
	UserData    uint64
	BufIndex    uint16
	Personality uint16
	SpliceFdIn  int32
	Addr3       uint64
	_pad        uint64
}

// CQE is a Completion Queue Event (16 bytes, matches kernel layout).
type CQE struct {
	UserData uint64
	Res      int32
	Flags    uint32
}

// ----------------------------- Ring (io_uring) ----------------------------

// Ring is a minimal io_uring instance for networking I/O.
type Ring struct {
	fd int

	// Local SQE tracking (user-space only).
	sqeHead uint32 // flushed up to here
	sqeTail uint32 // prepared up to here

	// Kernel-shared SQ pointers.
	sqKHead *uint32        // kernel writes (consumed head)
	sqKTail *uint32        // we write (submitted tail)
	sqMask  uint32         // ring_entries - 1
	sqArr   unsafe.Pointer // *[N]uint32 index array
	sqes    unsafe.Pointer // *[N]SQE

	// Kernel-shared CQ pointers.
	cqKHead *uint32 // we write (consumed head)
	cqKTail *uint32 // kernel writes (completed tail)
	cqMask  uint32
	cqesPtr unsafe.Pointer // *[N]CQE

	// For munmap cleanup.
	sqRingMem []byte
	cqRingMem []byte // nil when SINGLE_MMAP
	sqeMem    []byte
}

// NewRing creates a new io_uring with the given number of entries.
func NewRing(entries uint32) (*Ring, error) {
	var p ioUringParams
	fd, _, errno := syscall.Syscall(sysIOUringSetup, uintptr(entries), uintptr(unsafe.Pointer(&p)), 0)
	if errno != 0 {
		return nil, errno
	}

	r := &Ring{fd: int(fd)}
	if err := r.mapRings(&p); err != nil {
		syscall.Close(int(fd))
		return nil, err
	}
	return r, nil
}

func (r *Ring) mapRings(p *ioUringParams) error {
	sq := &p.sqOff
	cq := &p.cqOff
	singleMmap := p.features&featSingleMmap != 0

	// --- SQ ring ---
	sqRingSz := int(sq.array) + int(p.sqEntries)*4
	if singleMmap {
		cqRingSz := int(cq.cqes) + int(p.cqEntries)*int(unsafe.Sizeof(CQE{}))
		if cqRingSz > sqRingSz {
			sqRingSz = cqRingSz
		}
	}

	var err error
	r.sqRingMem, err = syscall.Mmap(r.fd, offSQRing, sqRingSz,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil || len(r.sqRingMem) == 0 {
		return fmt.Errorf("wing: mmap sq ring failed: %w", err)
	}

	// Bounds check SQ offsets before pointer arithmetic.
	sqBufLen := uint32(len(r.sqRingMem))
	if sq.head >= sqBufLen || sq.tail >= sqBufLen || sq.ringMask >= sqBufLen || sq.array >= sqBufLen {
		syscall.Munmap(r.sqRingMem)
		r.sqRingMem = nil
		return fmt.Errorf("wing: sq ring offsets out of bounds")
	}

	base := unsafe.Pointer(&r.sqRingMem[0])
	r.sqKHead = (*uint32)(unsafe.Add(base, sq.head))
	r.sqKTail = (*uint32)(unsafe.Add(base, sq.tail))
	r.sqMask = *(*uint32)(unsafe.Add(base, sq.ringMask))
	r.sqArr = unsafe.Add(base, sq.array)

	// --- CQ ring ---
	var cqBase unsafe.Pointer
	if singleMmap {
		r.cqRingMem = nil // same mapping
		cqBase = unsafe.Pointer(&r.sqRingMem[0])
	} else {
		cqRingSz := int(cq.cqes) + int(p.cqEntries)*int(unsafe.Sizeof(CQE{}))
		r.cqRingMem, err = syscall.Mmap(r.fd, offCQRing, cqRingSz,
			syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
		if err != nil || len(r.cqRingMem) == 0 {
			syscall.Munmap(r.sqRingMem)
			r.sqRingMem = nil
			return fmt.Errorf("wing: mmap cq ring failed: %w", err)
		}
		cqBase = unsafe.Pointer(&r.cqRingMem[0])
	}

	// Bounds check CQ offsets before pointer arithmetic.
	var cqBufLen uint32
	if singleMmap {
		cqBufLen = sqBufLen
	} else {
		cqBufLen = uint32(len(r.cqRingMem))
	}
	if cq.head >= cqBufLen || cq.tail >= cqBufLen || cq.ringMask >= cqBufLen || cq.cqes >= cqBufLen {
		if r.cqRingMem != nil {
			syscall.Munmap(r.cqRingMem)
			r.cqRingMem = nil
		}
		syscall.Munmap(r.sqRingMem)
		r.sqRingMem = nil
		return fmt.Errorf("wing: cq ring offsets out of bounds")
	}

	r.cqKHead = (*uint32)(unsafe.Add(cqBase, cq.head))
	r.cqKTail = (*uint32)(unsafe.Add(cqBase, cq.tail))
	r.cqMask = *(*uint32)(unsafe.Add(cqBase, cq.ringMask))
	r.cqesPtr = unsafe.Add(cqBase, cq.cqes)

	// --- SQE array ---
	sqeSz := int(p.sqEntries) * int(unsafe.Sizeof(SQE{}))
	r.sqeMem, err = syscall.Mmap(r.fd, offSQEs, sqeSz,
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED|syscall.MAP_POPULATE)
	if err != nil || len(r.sqeMem) == 0 {
		if r.cqRingMem != nil {
			syscall.Munmap(r.cqRingMem)
			r.cqRingMem = nil
		}
		syscall.Munmap(r.sqRingMem)
		r.sqRingMem = nil
		return fmt.Errorf("wing: mmap sqes failed: %w", err)
	}
	r.sqes = unsafe.Pointer(&r.sqeMem[0])
	return nil
}

// ----------------------------- SQ operations -----------------------------

// GetSQE returns the next available SQE, or nil if the submission queue is full.
func (r *Ring) GetSQE() *SQE {
	if r.sqes == nil {
		return nil
	}
	head := atomic.LoadUint32(r.sqKHead)
	if r.sqeTail-head > r.sqMask {
		return nil
	}
	idx := r.sqeTail & r.sqMask
	sqe := (*SQE)(unsafe.Add(r.sqes, uintptr(idx)*unsafe.Sizeof(SQE{})))
	*sqe = SQE{} // zero-out
	r.sqeTail++
	return sqe
}

// flushSQ copies prepared SQE indices into the kernel SQ array.
func (r *Ring) flushSQ() uint32 {
	toSubmit := r.sqeTail - r.sqeHead
	if toSubmit == 0 {
		return 0
	}
	ktail := atomic.LoadUint32(r.sqKTail)
	for i := r.sqeHead; i < r.sqeTail; i++ {
		*(*uint32)(unsafe.Add(r.sqArr, uintptr(ktail&r.sqMask)*4)) = i & r.sqMask
		ktail++
	}
	atomic.StoreUint32(r.sqKTail, ktail)
	r.sqeHead = r.sqeTail
	return toSubmit
}

// Submit flushes prepared SQEs to the kernel. Non-blocking.
func (r *Ring) Submit() (int, error) {
	n := r.flushSQ()
	if n == 0 {
		return 0, nil
	}
	ret, _, errno := syscall.Syscall6(sysIOUringEnter,
		uintptr(r.fd), uintptr(n), 0, 0, 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(ret), nil
}

// SubmitAndWait flushes and blocks until at least waitNr CQEs are ready.
func (r *Ring) SubmitAndWait(waitNr uint32) (int, error) {
	n := r.flushSQ()
	var flags uint32
	if waitNr > 0 {
		flags = iouringEnterGetEvents
	}
	ret, _, errno := syscall.Syscall6(sysIOUringEnter,
		uintptr(r.fd), uintptr(n), uintptr(waitNr), uintptr(flags), 0, 0)
	if errno != 0 {
		if errno == syscall.EINTR {
			return 0, nil // caller retries
		}
		return 0, errno
	}
	return int(ret), nil
}

// ----------------------------- CQ operations -----------------------------

// PeekCQE returns the next CQE without consuming it, or nil.
func (r *Ring) PeekCQE() *CQE {
	if r.cqesPtr == nil {
		return nil
	}
	head := atomic.LoadUint32(r.cqKHead)
	tail := atomic.LoadUint32(r.cqKTail)
	if head == tail {
		return nil
	}
	return (*CQE)(unsafe.Add(r.cqesPtr, uintptr(head&r.cqMask)*unsafe.Sizeof(CQE{})))
}

// WaitCQE blocks until at least one CQE is ready and returns it (unconsumed).
func (r *Ring) WaitCQE() (*CQE, error) {
	if cqe := r.PeekCQE(); cqe != nil {
		return cqe, nil
	}
	if _, err := r.SubmitAndWait(1); err != nil {
		return nil, err
	}
	if cqe := r.PeekCQE(); cqe != nil {
		return cqe, nil
	}
	return nil, syscall.EAGAIN
}

// SeenCQE marks one CQE as consumed.
// SAFETY: Only called from a single worker goroutine (LockOSThread).
// atomic.AddUint32 provides the release ordering required by the kernel.
func (r *Ring) SeenCQE() {
	atomic.AddUint32(r.cqKHead, 1)
}

// ----------------------------- SQE helpers -----------------------------

func (sqe *SQE) PrepareAccept(fd int, flags uint32) {
	sqe.Opcode = opIOAccept
	sqe.Fd = int32(fd)
	sqe.OpcFlags = flags
	sqe.Ioprio = iouringAcceptMultishot
}

func (sqe *SQE) PrepareRecv(fd int, buf unsafe.Pointer, length uint32) {
	sqe.Opcode = opIORecv
	sqe.Fd = int32(fd)
	sqe.Addr = uint64(uintptr(buf))
	sqe.Len = length
}

func (sqe *SQE) PrepareSend(fd int, buf unsafe.Pointer, length uint32) {
	sqe.Opcode = opIOSend
	sqe.Fd = int32(fd)
	sqe.Addr = uint64(uintptr(buf))
	sqe.Len = length
}

func (sqe *SQE) PrepareClose(fd int) {
	sqe.Opcode = opIOClose
	sqe.Fd = int32(fd)
}

// ----------------------------- cleanup -----------------------------

// Close tears down the ring and unmaps memory.
func (r *Ring) Close() {
	if r.sqeMem != nil {
		syscall.Munmap(r.sqeMem)
	}
	if r.cqRingMem != nil {
		syscall.Munmap(r.cqRingMem)
	}
	if r.sqRingMem != nil {
		syscall.Munmap(r.sqRingMem)
	}
	syscall.Close(r.fd)
}
