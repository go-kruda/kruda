//go:build linux || darwin

package kruda

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TestWingAppWithoutRoutesSurvivesFloods drives an app with no routes at all
// the way the crash reports did: pipelined requests read slowly, then an
// oversized header. Every app on Wing was exposed (GHSA-wm8c-h62r-r6mg), so
// the process must keep serving afterwards.
func TestWingAppWithoutRoutesSurvivesFloods(t *testing.T) {
	app := New()
	app.Compile()
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HeaderLimit: 8192, BodyLimit: 4 << 20, ReadTimeout: 30 * time.Second, WriteTimeout: 30 * time.Second}, app)
	defer stop()
	get := strings.Repeat("GET / HTTP/1.1\r\nHost: x\r\n\r\n", 600)
	for _, batches := range []int{8, 32, 128} {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("after %d batches the server is gone: %v", batches, err)
		}
		_ = c.(*net.TCPConn).SetReadBuffer(4096)
		_ = c.SetWriteDeadline(time.Now().Add(6 * time.Second))
		for range batches {
			if _, err := io.WriteString(c, get); err != nil {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = io.WriteString(c, "GET /x HTTP/1.1\r\nX: "+strings.Repeat("a", 9000))
		for range 4 {
			time.Sleep(100 * time.Millisecond)
			_, _ = io.WriteString(c, "a")
		}
		c.Close()
	}
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("server gone after the floods: %v", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.WriteString(c, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 12)
	if _, err := io.ReadFull(c, b); err != nil || !strings.HasPrefix(string(b), "HTTP/1.1 404") {
		t.Fatalf("after the floods: %q, %v; want a 404", b, err)
	}
}
