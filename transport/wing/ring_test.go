//go:build linux

package wing

import (
	"syscall"
	"testing"
	"unsafe"
)

// TestSQESize verifies the SQE struct matches kernel's 64-byte layout.
// If this fails, every io_uring operation will corrupt memory.
func TestSQESize(t *testing.T) {
	if got := unsafe.Sizeof(SQE{}); got != 64 {
		t.Fatalf("SQE size = %d, want 64 (kernel struct mismatch)", got)
	}
}

// TestCQESize verifies the CQE struct matches kernel's 16-byte layout.
func TestCQESize(t *testing.T) {
	if got := unsafe.Sizeof(CQE{}); got != 16 {
		t.Fatalf("CQE size = %d, want 16 (kernel struct mismatch)", got)
	}
}

// TestSQEFieldOffsets verifies each SQE field is at the correct offset.
// Reference: include/uapi/linux/io_uring.h struct io_uring_sqe
func TestSQEFieldOffsets(t *testing.T) {
	var sqe SQE
	base := uintptr(unsafe.Pointer(&sqe))

	tests := []struct {
		name   string
		offset uintptr
		want   uintptr
	}{
		{"Opcode", uintptr(unsafe.Pointer(&sqe.Opcode)) - base, 0},
		{"Flags", uintptr(unsafe.Pointer(&sqe.Flags)) - base, 1},
		{"Ioprio", uintptr(unsafe.Pointer(&sqe.Ioprio)) - base, 2},
		{"Fd", uintptr(unsafe.Pointer(&sqe.Fd)) - base, 4},
		{"Off", uintptr(unsafe.Pointer(&sqe.Off)) - base, 8},
		{"Addr", uintptr(unsafe.Pointer(&sqe.Addr)) - base, 16},
		{"Len", uintptr(unsafe.Pointer(&sqe.Len)) - base, 24},
		{"OpcFlags", uintptr(unsafe.Pointer(&sqe.OpcFlags)) - base, 28},
		{"UserData", uintptr(unsafe.Pointer(&sqe.UserData)) - base, 32},
		{"BufIndex", uintptr(unsafe.Pointer(&sqe.BufIndex)) - base, 40},
		{"Personality", uintptr(unsafe.Pointer(&sqe.Personality)) - base, 42},
		{"SpliceFdIn", uintptr(unsafe.Pointer(&sqe.SpliceFdIn)) - base, 44},
		{"Addr3", uintptr(unsafe.Pointer(&sqe.Addr3)) - base, 48},
	}

	for _, tt := range tests {
		if tt.offset != tt.want {
			t.Errorf("SQE.%s offset = %d, want %d", tt.name, tt.offset, tt.want)
		}
	}
}

// TestCQEFieldOffsets verifies CQE field offsets match kernel layout.
func TestCQEFieldOffsets(t *testing.T) {
	var cqe CQE
	base := uintptr(unsafe.Pointer(&cqe))

	tests := []struct {
		name   string
		offset uintptr
		want   uintptr
	}{
		{"UserData", uintptr(unsafe.Pointer(&cqe.UserData)) - base, 0},
		{"Res", uintptr(unsafe.Pointer(&cqe.Res)) - base, 8},
		{"Flags", uintptr(unsafe.Pointer(&cqe.Flags)) - base, 12},
	}

	for _, tt := range tests {
		if tt.offset != tt.want {
			t.Errorf("CQE.%s offset = %d, want %d", tt.name, tt.offset, tt.want)
		}
	}
}

// TestIOUringParamsSize verifies ioUringParams struct size.
func TestIOUringParamsSize(t *testing.T) {
	if got := unsafe.Sizeof(ioUringParams{}); got != 120 {
		t.Fatalf("ioUringParams size = %d, want 120", got)
	}
}

// TestNewRingBasic verifies we can create and close a ring without error.
// This is the most important smoke test — if io_uring_setup fails,
// the kernel doesn't support it or the syscall number is wrong.
func TestNewRingBasic(t *testing.T) {
	ring, err := NewRing(32)
	if err != nil {
		if err == syscall.ENOSYS {
			t.Skip("io_uring not supported on this kernel")
		}
		t.Fatalf("NewRing(32) failed: %v", err)
	}
	defer ring.Close()

	// Verify ring pointers are non-nil (mmap succeeded).
	if ring.sqKHead == nil || ring.sqKTail == nil {
		t.Fatal("SQ ring head/tail pointers are nil after mmap")
	}
	if ring.cqKHead == nil || ring.cqKTail == nil {
		t.Fatal("CQ ring head/tail pointers are nil after mmap")
	}
}

// TestGetSQEAndSubmit verifies basic SQE allocation and submission.
func TestGetSQEAndSubmit(t *testing.T) {
	ring, err := NewRing(32)
	if err != nil {
		if err == syscall.ENOSYS {
			t.Skip("io_uring not supported on this kernel")
		}
		t.Fatalf("NewRing(32) failed: %v", err)
	}
	defer ring.Close()

	// Should be able to get at least one SQE.
	sqe := ring.GetSQE()
	if sqe == nil {
		t.Fatal("GetSQE returned nil on empty ring")
	}

	// Prepare a NOP operation (opcode 0) — simplest possible SQE.
	sqe.Opcode = 0 // IORING_OP_NOP
	sqe.Fd = -1
	sqe.UserData = 42

	// Submit should not error.
	n, err := ring.Submit()
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("Submit returned %d, want 1", n)
	}

	// Wait for CQE.
	cqe, err := ring.WaitCQE()
	if err != nil {
		t.Fatalf("WaitCQE failed: %v", err)
	}
	if cqe == nil {
		t.Fatal("WaitCQE returned nil CQE")
	}
	if cqe.UserData != 42 {
		t.Fatalf("CQE.UserData = %d, want 42", cqe.UserData)
	}
	if cqe.Res != 0 {
		t.Fatalf("CQE.Res = %d, want 0 for NOP", cqe.Res)
	}

	ring.SeenCQE()
}

// TestRingSQFull verifies behavior when SQ ring is full.
func TestRingSQFull(t *testing.T) {
	ring, err := NewRing(4) // very small ring
	if err != nil {
		if err == syscall.ENOSYS {
			t.Skip("io_uring not supported on this kernel")
		}
		t.Fatalf("NewRing(4) failed: %v", err)
	}
	defer ring.Close()

	// Fill up the SQ ring.
	filled := 0
	for {
		sqe := ring.GetSQE()
		if sqe == nil {
			break
		}
		sqe.Opcode = 0 // NOP
		sqe.Fd = -1
		sqe.UserData = uint64(filled)
		filled++
	}

	if filled == 0 {
		t.Fatal("could not get any SQEs from ring of size 4")
	}
	if filled > 4 {
		t.Fatalf("got %d SQEs from ring of size 4", filled)
	}

	// One more should return nil.
	if sqe := ring.GetSQE(); sqe != nil {
		t.Fatal("GetSQE returned non-nil on full ring")
	}

	// Submit + drain should allow new SQEs.
	if _, err := ring.Submit(); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Drain CQEs.
	for {
		cqe := ring.PeekCQE()
		if cqe == nil {
			break
		}
		ring.SeenCQE()
	}

	// Now should be able to get SQEs again.
	sqe := ring.GetSQE()
	if sqe == nil {
		t.Fatal("GetSQE returned nil after draining")
	}
}

// TestUserDataEncoding verifies our op+fd packing scheme.
func TestUserDataEncoding(t *testing.T) {
	tests := []struct {
		op   uint64
		fd   int32
		name string
	}{
		{udAccept, 0, "accept/fd0"},
		{udRecv, 1, "recv/fd1"},
		{udSend, 42, "send/fd42"},
		{udClose, 65535, "close/fd65535"},
		{udWake, 100, "wake/fd100"},
	}

	for _, tt := range tests {
		ud := tt.op | uint64(tt.fd)

		// Extract op (upper 8 bits).
		gotOp := ud & (0xFF << 56)
		if gotOp != tt.op {
			t.Errorf("%s: op = %x, want %x", tt.name, gotOp, tt.op)
		}

		// Extract fd (lower 56 bits).
		gotFd := int32(ud & 0x00FFFFFFFFFFFFFF)
		if gotFd != tt.fd {
			t.Errorf("%s: fd = %d, want %d", tt.name, gotFd, tt.fd)
		}
	}
}

// TestSyscallNumbers verifies the syscall numbers match the kernel.
// These are x86_64 specific — ARM64 uses 425/426/427 too (unified since 5.1).
func TestSyscallNumbers(t *testing.T) {
	// The syscall numbers for io_uring are the same across architectures
	// since they were added in kernel 5.1 (after the great syscall unification).
	if sysIOUringSetup != 425 {
		t.Errorf("sysIOUringSetup = %d, want 425", sysIOUringSetup)
	}
	if sysIOUringEnter != 426 {
		t.Errorf("sysIOUringEnter = %d, want 426", sysIOUringEnter)
	}
	if sysIOUringRegister != 427 {
		t.Errorf("sysIOUringRegister = %d, want 427", sysIOUringRegister)
	}
}
