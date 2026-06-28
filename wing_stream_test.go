//go:build linux || darwin

package kruda

import (
	"bytes"
	"context"
	"io"
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
	// The blank line (\r\n\r\n) must terminate the header section immediately
	// before the first body chunk.
	if !strings.Contains(out, "\r\n\r\ndata: a") {
		t.Fatalf("blank line missing before body: %q", out)
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

// TestWingStreamWriter_FlushEmitsPreambleOnConnect verifies that Flush emits the
// status line + headers before any body byte. This is the SSE-on-connect path:
// c.SSE() flushes immediately so the client receives the text/event-stream
// headers even if the handler blocks before its first event.
func TestWingStreamWriter_FlushEmitsPreambleOnConnect(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.WriteHeader(200)
	w.Header().Set("Content-Type", "text/event-stream")
	// Flush with no body yet — the preamble must be on the wire afterward.
	w.Flush()
	out := fc.buf.String()
	if !strings.HasPrefix(out, "HTTP/1.1 200") {
		t.Fatalf("Flush must emit the status line, got: %q", out)
	}
	if !strings.Contains(out, "Content-Type: text/event-stream\r\n") {
		t.Fatalf("Flush must emit headers set before it, got: %q", out)
	}
	if !strings.HasSuffix(out, "\r\n\r\n") {
		t.Fatalf("Flush must terminate the header section with a blank line, got: %q", out)
	}
	// A second Flush must not re-emit the preamble.
	w.Flush()
	if strings.Count(fc.buf.String(), "HTTP/1.1") != 1 {
		t.Fatalf("preamble emitted more than once: %q", fc.buf.String())
	}
	// A following Write appends the body after the already-sent preamble.
	if _, err := w.Write([]byte("data: x\n\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(fc.buf.String(), "\r\n\r\ndata: x\n\n") {
		t.Fatalf("body must follow the preamble: %q", fc.buf.String())
	}
}

// TestWingStreamWriter_FlushBeforeWriteHeaderUsesDefault200 verifies a Flush
// before any WriteHeader emits a default 200 preamble and does not panic.
func TestWingStreamWriter_FlushBeforeWriteHeaderUsesDefault200(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.Flush() // no WriteHeader, no Write yet
	if !strings.HasPrefix(fc.buf.String(), "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("Flush before WriteHeader must default to 200, got: %q", fc.buf.String())
	}
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

// TestWatchStreamDisconnect_CancelsOnEOF verifies the read-watcher cancels its
// context and returns when the reader hits EOF (the client closed its half).
func TestWatchStreamDisconnect_CancelsOnEOF(t *testing.T) {
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		watchStreamDisconnect(pr, cancel)
		close(done)
	}()
	// Closing the write half makes the watcher's Read return EOF.
	_ = pw.Close()
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("context not cancelled after reader EOF")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher goroutine did not exit after EOF")
	}
}

// TestWatchStreamDisconnect_IgnoresStrayBytes verifies the watcher keeps reading
// past stray client bytes and only cancels once the read half errors.
func TestWatchStreamDisconnect_IgnoresStrayBytes(t *testing.T) {
	pr, pw := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		watchStreamDisconnect(pr, cancel)
		close(done)
	}()
	if _, err := pw.Write([]byte("stray")); err != nil {
		t.Fatalf("write stray: %v", err)
	}
	// Still alive after stray bytes — must not have cancelled yet.
	select {
	case <-ctx.Done():
		t.Fatal("watcher cancelled on stray bytes; must keep watching")
	case <-time.After(50 * time.Millisecond):
	}
	_ = pw.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher goroutine did not exit after EOF")
	}
}

func TestWingStreamWriter_WriteHeaderAfterSentIsNoop(t *testing.T) {
	fc := &fakeStreamConn{}
	w := newWingStreamWriter(fc, 0)
	w.WriteHeader(200)
	_, _ = w.Write([]byte("data: a\n\n"))
	// Too late — the 200 status line is already on the wire.
	w.WriteHeader(500)
	_, _ = w.Write([]byte("data: b\n\n"))
	out := fc.buf.String()
	if !strings.HasPrefix(out, "HTTP/1.1 200") {
		t.Fatalf("status line must stay 200, got: %q", out)
	}
	if strings.Contains(out, "HTTP/1.1 500") {
		t.Fatalf("late WriteHeader(500) must be a no-op: %q", out)
	}
}
