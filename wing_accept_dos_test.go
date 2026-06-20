//go:build linux || darwin

package kruda

import (
	"net"
	"net/netip"
	"testing"
	"time"
)

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
