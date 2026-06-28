//go:build linux || darwin

package kruda

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

type fakeStreamConn struct {
	buf       bytes.Buffer
	deadlines int
}

func (f *fakeStreamConn) Write(p []byte) (int, error)        { return f.buf.Write(p) }
func (f *fakeStreamConn) SetWriteDeadline(t time.Time) error { f.deadlines++; return nil }

func TestWingStreamWriter_HeadersThenChunks(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "text/event-stream")
	if _, err := w.Write([]byte("data: a\n\n")); err != nil {
		t.Fatal(err)
	}
	w.Flush() // no-op
	if _, err := w.Write([]byte("data: b\n\n")); err != nil {
		t.Fatal(err)
	}
	out := fc.buf.String()
	if !strings.HasPrefix(out, "HTTP/1.1 200") {
		t.Fatalf("missing status line: %q", out)
	}
	if !strings.Contains(out, "Content-Type: text/event-stream\r\n") {
		t.Fatalf("missing header: %q", out)
	}
	// The two chunks arrive in order, each written as it was produced.
	if i, j := strings.Index(out, "data: a"), strings.Index(out, "data: b"); i < 0 || j < 0 || i > j {
		t.Fatalf("chunks out of order: %q", out)
	}
}

func TestWingStreamWriter_NoDeadlineWhenZero(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.WriteHeader(200)
	_, _ = w.Write([]byte("data: x\n\n"))
	_, _ = w.Write([]byte("data: y\n\n"))
	if fc.deadlines != 0 {
		t.Fatalf("expected 0 deadline calls with writeTimeout=0, got %d", fc.deadlines)
	}
}

func TestWingStreamWriter_DeadlineSetPerWrite(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 5*time.Second)
	w.WriteHeader(200)
	// First Write: 2 deadline calls (one for headers, one for body).
	_, _ = w.Write([]byte("data: a\n\n"))
	// Second Write: 1 deadline call.
	_, _ = w.Write([]byte("data: b\n\n"))
	if fc.deadlines != 3 {
		t.Fatalf("expected 3 deadline calls, got %d", fc.deadlines)
	}
}

func TestWingStreamWriter_HeadersSentOnce(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.WriteHeader(201)
	_, _ = w.Write([]byte("chunk1"))
	_, _ = w.Write([]byte("chunk2"))
	out := fc.buf.String()
	// Status line must appear exactly once.
	if count := strings.Count(out, "HTTP/1.1"); count != 1 {
		t.Fatalf("status line appeared %d times, want 1: %q", count, out)
	}
	if !strings.Contains(out, "chunk1") || !strings.Contains(out, "chunk2") {
		t.Fatalf("missing chunks: %q", out)
	}
}

func TestWingStreamWriter_FlushIsNoop(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	// Flush before any write — must not panic.
	w.Flush()
	w.WriteHeader(200)
	w.Flush()
	_, _ = w.Write([]byte("ok"))
	w.Flush()
}

func TestWingStreamWriter_Default200(t *testing.T) {
	fc := &fakeStreamConn{}
	// Do not call WriteHeader — default must be 200.
	w := newWingStreamWriter(fc, 0)
	_, _ = w.Write([]byte("hello"))
	out := fc.buf.String()
	if !strings.HasPrefix(out, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("expected default 200 OK status line, got: %q", out)
	}
}
