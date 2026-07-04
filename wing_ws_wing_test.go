//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// startWingHijackServer compiles app and serves it over a real Wing transport
// on a loopback port, returning the dial address and a stop func. app MUST be
// built with New(Wing()) so app.transport is the Wing transport. Mirrors the
// bring-up idiom in startWingApp (wing_stream_integration_test.go); reused by
// later WebSocket-on-Wing tasks.
func startWingHijackServer(t *testing.T, app *App) (string, func()) {
	t.Helper()
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

// TestWingHijack_RejectUpgradeWithBody verifies that a Hijack-preset route
// (WebSocket upgrade) rejects a request carrying a body with a clean 400
// instead of dispatching inline (where the accumulated-body writer is not an
// http.Hijacker and Upgrade() would fail opaquely).
func TestWingHijack_RejectUpgradeWithBody(t *testing.T) {
	app := New(Wing()) // force Wing on every platform (kqueue on macOS, epoll on Linux)
	app.Get("/ws", func(c *Ctx) error { return c.Text("unreachable") }, Hijack)
	app.Compile()
	addr, stop := startWingHijackServer(t, app)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Split header/body across two writes so the server's first parse attempt
	// sees an incomplete body and enters accumulation (beginBodyAccum /
	// dispatchAccumulated) instead of parsing the whole request in one shot.
	head := "GET /ws HTTP/1.1\r\nHost: x\r\nContent-Length: 5\r\n\r\n"
	conn.Write([]byte(head))
	time.Sleep(50 * time.Millisecond)
	conn.Write([]byte("hello"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 for hijack route with body, got %d", resp.StatusCode)
	}
}

// wsHijackDoner is the interface wingHijackConn exposes so a handler blocked in
// app logic (no pending socket call) can select on server shutdown. Mirrors the
// "doner" interface contrib/ws.Conn asserts against its rwc.
type wsHijackDoner interface {
	Done() <-chan struct{}
}

// dialRawHijack dials addr and performs the plain-HTTP handshake that the /ws
// route expects (no real WebSocket framing needed — the handler hijacks the
// connection before ever looking at the request beyond routing, so a minimal
// GET is enough to reach the handler).
func dialRawHijack(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	req := "GET /ws HTTP/1.1\r\nHost: x\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write handshake: %v", err)
	}
	return conn
}

// TestWSOverWing_AsyncHijackOutlivesHandler proves the http.Hijacker ownership
// fix: a handler may hijack, hand the connection to a goroutine, and RETURN,
// with the goroutine still using the connection afterward. hijackTakeover must
// hold the fd open until the app closes it — if it reclaimed the fd on
// handler-return (and the kernel could recycle it), the goroutine's later write
// would target the wrong fd. The goroutine writes AFTER the handler returns; the
// client must receive those exact bytes.
func TestWSOverWing_AsyncHijackOutlivesHandler(t *testing.T) {
	app := New(Wing())
	app.Get("/ws", func(c *Ctx) error {
		w := c.ResponseWriter()
		hj, ok := w.(http.Hijacker)
		if !ok {
			return c.Text("no hijack")
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			return nil
		}
		// Hand the connection to a goroutine that writes only AFTER this handler
		// has returned (it blocks on handlerReturned, which the defer closes), so
		// the after-return use is deterministic, not timing-dependent.
		handlerReturned := make(chan struct{})
		go func() {
			<-handlerReturned
			_, _ = nc.Write([]byte("late"))
			_ = nc.Close()
		}()
		defer close(handlerReturned)
		return nil
	}, Hijack)
	app.Compile()
	addr, stop := startWingHijackServer(t, app)
	defer stop()

	conn := dialRawHijack(t, addr)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, 0, 8)
	buf := make([]byte, 8)
	for len(got) < 4 {
		n, err := conn.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	if string(got) != "late" {
		t.Fatalf("async write after handler-return = %q, want \"late\" — fd was reclaimed early?", got)
	}
}

// TestWSOverWing_ShutdownDrainsBlockedHandlers is the Phase 4 shutdown arbiter:
// a hijacked handler blocked in read, in write, or in app logic (no socket call
// at all) must ALL drain within the Shutdown deadline, proving cleanup()'s
// SHUT_RDWR + conn-context cancel (commit bc5de18) actually wake every shape of
// blocked handler contrib/ws can produce.
func TestWSOverWing_ShutdownDrainsBlockedHandlers(t *testing.T) {
	const shutdownDeadline = 2 * time.Second

	t.Run("blocked_in_read", func(t *testing.T) {
		ready := make(chan struct{})
		handlerDone := make(chan struct{})

		app := New(Wing())
		app.Get("/ws", func(c *Ctx) error {
			w := c.ResponseWriter()
			hj, ok := w.(http.Hijacker)
			if !ok {
				return c.Text("no hijack")
			}
			nc, _, err := hj.Hijack()
			if err != nil {
				return nil
			}
			defer close(handlerDone)
			close(ready)
			buf := make([]byte, 16)
			nc.Read(buf) // client sends nothing; SHUT_RDWR must wake this with EOF/error
			return nil
		}, Hijack)
		app.Compile()
		addr, _ := startWingHijackServer(t, app) // Shutdown driven manually below

		conn := dialRawHijack(t, addr)
		defer conn.Close()

		select {
		case <-ready:
		case <-time.After(2 * time.Second):
			t.Fatal("handler never reached the blocked-read state")
		}
		// Guard against a vacuous pass: the client sent nothing after the
		// handshake, so the handler must still be parked in Read when we call
		// Shutdown — only SHUT_RDWR can wake it.
		select {
		case <-handlerDone:
			t.Fatal("read handler exited before Shutdown — Read did not block, drain is unproven")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), shutdownDeadline)
		defer cancel()
		start := time.Now()
		err := app.transport.Shutdown(ctx)
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
		if elapsed > shutdownDeadline {
			t.Errorf("Shutdown took %v, exceeding deadline %v", elapsed, shutdownDeadline)
		}

		select {
		case <-handlerDone:
		case <-time.After(shutdownDeadline):
			t.Fatal("handler blocked in Read did not return within the shutdown deadline")
		}
	})

	t.Run("blocked_in_write", func(t *testing.T) {
		ready := make(chan struct{})
		handlerDone := make(chan struct{})

		app := New(Wing())
		app.Get("/ws", func(c *Ctx) error {
			w := c.ResponseWriter()
			hj, ok := w.(http.Hijacker)
			if !ok {
				return c.Text("no hijack")
			}
			nc, _, err := hj.Hijack()
			if err != nil {
				return nil
			}
			defer close(handlerDone)
			close(ready)
			big := make([]byte, 64*1024)
			// Client never reads; once the kernel send buffer + the client's
			// unread-but-full receive buffer back up, Write blocks. SHUT_RDWR
			// must wake it (EPIPE/ECONNRESET) rather than let it hang forever.
			for {
				if _, werr := nc.Write(big); werr != nil {
					return nil
				}
			}
		}, Hijack)
		app.Compile()
		addr, _ := startWingHijackServer(t, app)

		conn := dialRawHijack(t, addr)
		defer conn.Close()
		// Shrink the client's receive buffer and never read, so the server's
		// write side backs up and blocks quickly instead of needing megabytes.
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetReadBuffer(1024)
		}

		select {
		case <-ready:
		case <-time.After(2 * time.Second):
			t.Fatal("handler never reached the blocked-write state")
		}
		// Give the handler a moment to actually fill the socket buffers and
		// land inside a blocking Write, not just start its first (non-blocking)
		// write iteration.
		time.Sleep(100 * time.Millisecond)

		// Guard against a vacuous pass: if the handler has already finished, the
		// Write never actually blocked (nothing for Shutdown to drain), so this
		// subtest would prove nothing about SHUT_RDWR waking a blocked writer.
		select {
		case <-handlerDone:
			t.Fatal("write handler exited before Shutdown — Write never blocked, drain is unproven")
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), shutdownDeadline)
		defer cancel()
		start := time.Now()
		err := app.transport.Shutdown(ctx)
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
		if elapsed > shutdownDeadline {
			t.Errorf("Shutdown took %v, exceeding deadline %v", elapsed, shutdownDeadline)
		}

		select {
		case <-handlerDone:
		case <-time.After(shutdownDeadline):
			t.Fatal("handler blocked in Write did not return within the shutdown deadline")
		}
	})

	t.Run("blocked_in_app_logic", func(t *testing.T) {
		ready := make(chan struct{})
		handlerDone := make(chan struct{})
		doneFired := make(chan struct{})

		app := New(Wing())
		app.Get("/ws", func(c *Ctx) error {
			w := c.ResponseWriter()
			hj, ok := w.(http.Hijacker)
			if !ok {
				return c.Text("no hijack")
			}
			nc, _, err := hj.Hijack()
			if err != nil {
				return nil
			}
			defer close(handlerDone)
			d, ok := nc.(wsHijackDoner)
			if !ok {
				close(ready)
				return fmt.Errorf("hijacked conn does not expose Done()")
			}
			close(ready)
			// No socket call at all: only the conn-context cancel (Task 6 /
			// commit bc5de18) can wake this handler.
			select {
			case <-d.Done():
				close(doneFired)
				return nil
			case <-time.After(time.Hour):
				return nil // test fails below on the doneFired/handlerDone assertions
			}
		}, Hijack)
		app.Compile()
		addr, _ := startWingHijackServer(t, app)

		conn := dialRawHijack(t, addr)
		defer conn.Close()

		select {
		case <-ready:
		case <-time.After(2 * time.Second):
			t.Fatal("handler never reached the blocked-app-logic state")
		}

		ctx, cancel := context.WithTimeout(context.Background(), shutdownDeadline)
		defer cancel()
		start := time.Now()
		err := app.transport.Shutdown(ctx)
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("Shutdown returned error: %v", err)
		}
		if elapsed > shutdownDeadline {
			t.Errorf("Shutdown took %v, exceeding deadline %v", elapsed, shutdownDeadline)
		}

		select {
		case <-doneFired:
		case <-time.After(shutdownDeadline):
			t.Fatal("conn.Done() did not fire within the shutdown deadline")
		}
		select {
		case <-handlerDone:
		case <-time.After(shutdownDeadline):
			t.Fatal("handler blocked in app logic did not return within the shutdown deadline")
		}
	})
}

// TestWSOverWing_FdLifecycle churns hijack connections (open -> handshake ->
// close) under -race and asserts the server survives cleanly, i.e. the taken-
// over fd is closed exactly once per connection with no leak, double-close, or
// recycle race (mirrors wing_shutdown_fd_test.go's finalizer-based technique,
// scoped here to steady-state churn rather than shutdown).
func TestWSOverWing_FdLifecycle(t *testing.T) {
	const iterations = 40

	handlerDone := make(chan struct{}, iterations)

	app := New(Wing())
	app.Get("/ws", func(c *Ctx) error {
		w := c.ResponseWriter()
		hj, ok := w.(http.Hijacker)
		if !ok {
			return c.Text("no hijack")
		}
		nc, _, err := hj.Hijack()
		if err != nil {
			return nil
		}
		defer func() { handlerDone <- struct{}{} }()
		buf := make([]byte, 16)
		nc.Write([]byte("hello\n"))
		nc.Read(buf) // returns once the client closes its side
		nc.Close()
		return nil
	}, Hijack)
	app.Compile()
	addr, stop := startWingHijackServer(t, app)
	defer stop()

	for i := 0; i < iterations; i++ {
		conn := dialRawHijack(t, addr)
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		br := bufio.NewReader(conn)
		line, err := br.ReadString('\n')
		if err != nil || line != "hello\n" {
			t.Fatalf("iter %d: unexpected handshake read: line=%q err=%v", i, line, err)
		}
		conn.Close() // triggers the server-side Read to return, closing the fd exactly once

		select {
		case <-handlerDone:
		case <-time.After(2 * time.Second):
			t.Fatalf("iter %d: handler did not complete after client close", i)
		}
	}

	// One more connection after the churn proves no fd leak/recycle race left
	// the listener or worker in a bad state.
	conn := dialRawHijack(t, addr)
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil || line != "hello\n" {
		t.Fatalf("post-churn: unexpected handshake read: line=%q err=%v", line, err)
	}
	conn.Close()
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("post-churn: handler did not complete after client close")
	}
}
