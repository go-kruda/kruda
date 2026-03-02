//go:build linux

package wing

import "testing"

// TestGetSQENilSqes verifies that GetSQE returns nil when r.sqes is nil
// (e.g., after a partial mmap failure where SQE mapping was never set up).
// Validates: R4.4
func TestGetSQENilSqes(t *testing.T) {
	r := &Ring{} // zero-value: sqes == nil
	sqe := r.GetSQE()
	if sqe != nil {
		t.Fatal("GetSQE should return nil when sqes is nil")
	}
}

// TestPeekCQENilCqesPtr verifies that PeekCQE returns nil when r.cqesPtr is nil
// (e.g., after a partial mmap failure where CQ mapping was never set up).
// Validates: R4.5
func TestPeekCQENilCqesPtr(t *testing.T) {
	r := &Ring{} // zero-value: cqesPtr == nil
	cqe := r.PeekCQE()
	if cqe != nil {
		t.Fatal("PeekCQE should return nil when cqesPtr is nil")
	}
}

// TestCloseZeroValueRing verifies that Close does not panic on a zero-value Ring
// where mapRings was never called (all memory regions are nil).
// Validates: R4.6
func TestCloseZeroValueRing(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close panicked on zero-value Ring: %v", r)
		}
	}()

	r := &Ring{fd: -1} // invalid fd, all mmap regions nil
	r.Close()
}

// TestClosePartialMmapFailure verifies that Close handles partial mmap state
// where only sqRingMem was mapped but cqRingMem and sqeMem are nil.
// Validates: R4.6
func TestClosePartialMmapFailure(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Close panicked on partial mmap Ring: %v", r)
		}
	}()

	r := &Ring{
		fd:        -1,
		sqRingMem: nil, // simulate: all mappings failed
		cqRingMem: nil,
		sqeMem:    nil,
	}
	r.Close()
}
