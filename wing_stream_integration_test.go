//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// startWingApp compiles app and serves it over a real Wing transport on a
// loopback port, returning the dial address and a stop func. The app MUST be
// built with Wing() so app.transport is the Wing transport (the only transport
// that owns the streaming takeover path).
func startWingApp(t *testing.T, app *App) (string, func()) {
	t.Helper()
	if _, ok := app.transport.(transport.PresetConfigurator); !ok {
		t.Skip("Wing transport unavailable on this platform; skipping streaming integration test")
	}
	if err := app.compile(); err != nil {
		t.Fatalf("compile: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // let the transport bind so a successful dial means it is accepting
	go func() { _ = app.transport.ListenAndServe(addr, app) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, derr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if derr == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return addr, func() { _ = app.transport.Shutdown(context.Background()) }
}

// readSSEEvent reads from r until it has consumed one full SSE event
// (terminated by a blank line, i.e. "\n\n"). It returns the raw event text.
func readSSEEvent(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE event: %v (got so far: %q)", err, sb.String())
		}
		sb.WriteString(line)
		if line == "\n" || line == "\r\n" {
			return sb.String()
		}
	}
}

// readHeaders consumes the HTTP status line + header block (up to the blank
// line) and returns the joined header text.
func readHeaders(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		sb.WriteString(line)
		if line == "\r\n" || line == "\n" {
			return sb.String()
		}
	}
}

// TestWingStream_IncrementalDelivery proves the Stream preset flushes each SSE
// event to the socket as the handler produces it, rather than buffering the
// whole response. The handler blocks on a per-event gate that the test only
// releases after it has read the event off the wire — so the read CANNOT
// succeed unless the byte was flushed before the next handler write.
func TestWingStream_IncrementalDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing streaming integration test in short mode")
	}

	const n = 3
	release := make([]chan struct{}, n)
	for i := range release {
		release[i] = make(chan struct{})
	}

	app := New(Wing())
	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			for i := 0; i < n; i++ {
				if err := s.Data(fmt.Sprintf("event-%d", i)); err != nil {
					return err
				}
				// Block until the test confirms it read this event off the
				// socket. A buffered transport would deadlock here because the
				// reader could not observe the event before this gate opens.
				select {
				case <-release[i]:
				case <-time.After(2 * time.Second):
					return fmt.Errorf("event %d gate timed out", i)
				}
			}
			return nil
		})
	}, Stream)

	addr, stop := startWingApp(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("GET /events HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	r := bufio.NewReader(conn)
	hdr := readHeaders(t, r)
	if !strings.HasPrefix(hdr, "HTTP/1.1 200") {
		t.Fatalf("expected 200 status line, got headers: %q", hdr)
	}
	if !strings.Contains(hdr, "Content-Type: text/event-stream") {
		t.Fatalf("missing text/event-stream content type: %q", hdr)
	}

	for i := 0; i < n; i++ {
		ev := readSSEEvent(t, r)
		want := fmt.Sprintf("data: event-%d\n\n", i)
		if ev != want {
			t.Fatalf("event %d = %q, want %q", i, ev, want)
		}
		// Only now release the handler's next write. If the transport had
		// buffered, we'd never have reached this read.
		close(release[i])
	}
}

// TestWingStream_DisconnectCancelsContext proves that closing the client socket
// fires stream.Done() in the handler (via the disconnect read-watcher cancelling
// the request context) and that the handler returns promptly with no leak or
// double-close. Run the file under -race to catch any data race on the shared fd.
func TestWingStream_DisconnectCancelsContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing streaming integration test in short mode")
	}

	doneFired := make(chan struct{})
	handlerReturned := make(chan struct{})
	firstSent := make(chan struct{})

	app := New(Wing())
	app.Get("/events", func(c *Ctx) error {
		defer close(handlerReturned)
		return c.SSE(func(s *SSEStream) error {
			if err := s.Data("hello"); err != nil {
				return err
			}
			close(firstSent)
			select {
			case <-s.Done():
				close(doneFired)
				return nil
			case <-time.After(5 * time.Second):
				return fmt.Errorf("Done() never fired after client disconnect")
			}
		})
	}, Stream)

	addr, stop := startWingApp(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte("GET /events HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	r := bufio.NewReader(conn)
	_ = readHeaders(t, r)
	if ev := readSSEEvent(t, r); ev != "data: hello\n\n" {
		t.Fatalf("first event = %q, want %q", ev, "data: hello\n\n")
	}

	<-firstSent
	// Drop the client. The disconnect watcher must observe EOF and cancel.
	conn.Close()

	select {
	case <-doneFired:
	case <-time.After(5 * time.Second):
		t.Fatal("stream.Done() did not fire after client disconnect")
	}
	select {
	case <-handlerReturned:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}
}

// TestWingStream_WriteDeadlineUnblocksHandler proves a stuck client (connects,
// never reads) cannot pin the handler forever: with a short WriteTimeout the
// streaming write fails once the socket buffer fills, so the handler returns
// with an error within ~the deadline and its goroutine exits.
func TestWingStream_WriteDeadlineUnblocksHandler(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing streaming integration test in short mode")
	}

	handlerErr := make(chan error, 1)

	app := New(Wing(), func(a *App) { a.config.WriteTimeout = 200 * time.Millisecond })
	app.Get("/events", func(c *Ctx) error {
		err := c.SSE(func(s *SSEStream) error {
			// Stream a large volume the stuck client never drains. Once the
			// kernel send buffer fills and the write deadline elapses, the
			// write errors out and we propagate it.
			big := strings.Repeat("x", 16*1024)
			for i := 0; i < 100000; i++ {
				if err := s.Data(big); err != nil {
					return err
				}
			}
			return nil
		})
		handlerErr <- err
		return err
	}, Stream)

	addr, stop := startWingApp(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	// Send the request, then never read the response — let the send buffer fill.
	if _, err := conn.Write([]byte("GET /events HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case err := <-handlerErr:
		if err == nil {
			t.Fatal("expected handler to return a write error from the deadline, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return within deadline despite stuck client")
	}
}
