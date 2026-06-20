//go:build linux || darwin

package kruda

import (
	"net"
	"net/netip"
	"testing"
	"time"
)

// TestWingAccept_PerIPCap verifies that the per-IP connection limit closes the
// fd immediately (no HTTP response) once a source IP reaches its cap.
//
// The test forces 1 worker via KRUDA_WORKERS=1 so SO_REUSEPORT does not scatter
// loopback connections across workers. With a single worker every loopback
// connection lands in the same per-worker map, making the cap deterministic.
func TestWingAccept_PerIPCap(t *testing.T) {
	t.Setenv("KRUDA_WORKERS", "1")

	app := New(Wing(), WithMaxConnsPerIP(2))
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if app.transportType != "wing" {
		t.Skip("wing only")
	}
	addr, shutdown := startSmokeApp(t, app, "/")
	defer shutdown()

	// Wait for any readiness-probe connection to drain so its slot does not
	// consume from the per-IP budget.
	waitFor(t, 2*time.Second, func() bool { return app.wingConnCount() == 0 })

	var held []net.Conn
	for i := 0; i < 2; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		if _, err := c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		readOnce(t, c)
		held = append(held, c)
	}
	// 3rd connection from the same loopback IP must be refused (fd closed, no response).
	c3, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial 3: %v", err)
	}
	_ = c3.SetReadDeadline(time.Now().Add(time.Second))
	_, _ = c3.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
	if n, _ := c3.Read(make([]byte, 64)); n > 0 {
		t.Fatal("3rd conn from same IP over per-IP cap got response, want refusal")
	}
	c3.Close()
	for _, c := range held {
		c.Close()
	}
}

// TestWingAccept_CapturesPeerIP drives a real connection through the Wing
// transport and asserts the peer IP captured at accept time (conn.peerIP),
// not the lazy getpeername that c.IP() would otherwise use. The accept-time
// value is observed through testAcceptPeerHook so the assertion is pinned to
// the new accept path.
func TestWingAccept_CapturesPeerIP(t *testing.T) {
	app := New(Wing())
	app.Get("/ip", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if app.transportType != "wing" {
		t.Skipf("wing not selected: %q", app.transportType)
	}

	peerCh := make(chan netip.Addr, 4)
	testAcceptPeerHook = func(ip netip.Addr, ok bool) {
		if ok {
			peerCh <- ip
		}
	}
	t.Cleanup(func() { testAcceptPeerHook = nil })

	addr, shutdown := startSmokeApp(t, app, "/ip")
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("GET /ip HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 256)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Read(buf)

	// The readiness probe in startSmokeApp also accepts connections, so drain
	// until we observe the loopback peer captured at accept time.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ip := <-peerCh:
			if ip.Unmap().String() == "127.0.0.1" {
				return
			}
		case <-deadline:
			t.Fatalf("did not observe accept-time peer IP 127.0.0.1")
		}
	}
}

// TestWingAccept_HandleAcceptWiresPeerIP pins the event→conn wiring directly:
// handleAccept must copy ev.PeerIP onto the created conn.peerIP.
func TestWingAccept_HandleAcceptWiresPeerIP(t *testing.T) {
	w, _ := newTestWorker(0)
	want := netip.AddrFrom4([4]byte{203, 0, 113, 7})
	// Use a fake fd that the speculative read will EAGAIN/err on; conn is still
	// created and stored before the read attempt.
	const fakeFd int32 = 0x7ffffff0
	w.handleAccept(event{Op: opAccept, Res: fakeFd, Flags: cqeFMore, PeerIP: want, HasPeer: true})
	c := w.conns[fakeFd]
	if c == nil {
		t.Fatalf("conn not registered for fd %d", fakeFd)
	}
	if c.peerIP != want {
		t.Fatalf("conn.peerIP = %v, want %v", c.peerIP, want)
	}
}

// TestWingAccept_TotalCapAndNoLeak drives real sockets through the Wing accept
// path: the global total cap (WithMaxConns) must refuse the connection past the
// cap (RST/immediate close, no HTTP response), and the shared connCount must
// return to 0 once every admitted connection closes.
func TestWingAccept_TotalCapAndNoLeak(t *testing.T) {
	app := New(Wing(), WithMaxConns(3))
	app.Get("/", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if app.transportType != "wing" {
		t.Skipf("wing not selected: %q", app.transportType)
	}
	addr, shutdown := startSmokeApp(t, app, "/")
	defer shutdown()

	// The readiness probe's (now keep-alive-disabled) connection must fully
	// close server-side before we fill the cap, or its lingering slot skews
	// the count.
	waitFor(t, 2*time.Second, func() bool { return app.wingConnCount() == 0 })

	// Hold 3 keep-alive conns open.
	var held []net.Conn
	for i := 0; i < 3; i++ {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
			t.Fatal(err)
		}
		readOnce(t, c)
		held = append(held, c)
	}
	// 4th must be refused (RST / immediate close on write+read).
	c4, err := net.Dial("tcp", addr)
	if err == nil {
		_ = c4.SetReadDeadline(time.Now().Add(time.Second))
		_, _ = c4.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n"))
		if n, _ := c4.Read(make([]byte, 64)); n > 0 {
			t.Fatal("4th connection over cap got a response, want refusal")
		}
		c4.Close()
	}
	// Release all; counter must return to 0.
	for _, c := range held {
		c.Close()
	}
	waitFor(t, 2*time.Second, func() bool { return app.wingConnCount() == 0 })
}

// readOnce reads a single response chunk with a short deadline, failing the
// test if nothing arrives. Used to confirm a held keep-alive conn was served.
func readOnce(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := c.Read(make([]byte, 256)); n == 0 {
		t.Fatalf("readOnce: no response bytes: %v", err)
	}
	_ = c.SetReadDeadline(time.Time{})
}

// waitFor polls cond until it returns true or the bound elapses.
func waitFor(t *testing.T, bound time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(bound)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitFor: condition not met within %s", bound)
}

// wingConnCount reads the shared accepted-connection counter for tests. Returns
// -1 if the active transport is not Wing.
func (a *App) wingConnCount() int64 {
	if tr, ok := a.transport.(*Transport); ok {
		return tr.connCount()
	}
	return -1
}
