//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
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
