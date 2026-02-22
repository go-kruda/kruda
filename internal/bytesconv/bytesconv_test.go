package bytesconv

import (
	"testing"
)

// --- UnsafeString tests ---

func TestUnsafeString_NilSlice(t *testing.T) {
	if got := UnsafeString(nil); got != "" {
		t.Errorf("expected empty string for nil slice, got %q", got)
	}
}

func TestUnsafeString_EmptySlice(t *testing.T) {
	if got := UnsafeString([]byte{}); got != "" {
		t.Errorf("expected empty string for empty slice, got %q", got)
	}
}

func TestUnsafeString_ASCII(t *testing.T) {
	input := []byte("hello, world")
	if got := UnsafeString(input); got != "hello, world" {
		t.Errorf("expected %q, got %q", "hello, world", got)
	}
}

func TestUnsafeString_Unicode(t *testing.T) {
	input := []byte("สวัสดี 🌏")
	if got := UnsafeString(input); got != "สวัสดี 🌏" {
		t.Errorf("expected %q, got %q", "สวัสดี 🌏", got)
	}
}

// --- UnsafeBytes tests ---

func TestUnsafeBytes_EmptyString(t *testing.T) {
	if got := UnsafeBytes(""); got != nil {
		t.Errorf("expected nil for empty string, got %v", got)
	}
}

func TestUnsafeBytes_ASCII(t *testing.T) {
	input := "hello, world"
	got := UnsafeBytes(input)
	if string(got) != input {
		t.Errorf("expected %q, got %q", input, string(got))
	}
}

func TestUnsafeBytes_Unicode(t *testing.T) {
	input := "สวัสดี 🌏"
	got := UnsafeBytes(input)
	if string(got) != input {
		t.Errorf("expected %q, got %q", input, string(got))
	}
}

// --- Benchmarks ---

var sinkStr string
var sinkBytes []byte

func BenchmarkUnsafeString(b *testing.B) {
	b.ReportAllocs()
	input := []byte("benchmark input string")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkStr = UnsafeString(input)
	}
	if b.Elapsed() > 0 && false {
		_ = sinkStr // prevent optimisation
	}
}

func BenchmarkUnsafeBytes(b *testing.B) {
	b.ReportAllocs()
	input := "benchmark input string"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkBytes = UnsafeBytes(input)
	}
}
