//go:build linux || darwin

package kruda

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingIdleTimeoutSparesActiveDownload reads a large Inline response slowly
// with a short IdleTimeout and no WriteTimeout. The connection is sending, not
// idle, so the idle sweep must not close it mid-body.
func TestWingIdleTimeoutSparesActiveDownload(t *testing.T) {
	body := strings.Repeat("d", 6<<20)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		_, _ = w.Write([]byte(body))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, DefaultPreset: Bolt, IdleTimeout: 300 * time.Millisecond}, handler)
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(64 << 10)
	}
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReaderSize(conn, 64<<10), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	start := time.Now()
	buf := make([]byte, 128<<10)
	got := 0
	for {
		n, err := resp.Body.Read(buf)
		got += n
		if err != nil {
			break
		}
		time.Sleep(60 * time.Millisecond)
	}
	if got != len(body) {
		t.Fatalf("download cut at %d of %d bytes after %v", got, len(body), time.Since(start))
	}
	if time.Since(start) < 1500*time.Millisecond {
		t.Skipf("download finished in %v, before the idle sweep could run", time.Since(start))
	}
}

// TestWingIdleTimeoutClosesStalledDownload requests a large body and never
// reads it, with no WriteTimeout. The response stops making progress, so the
// idle sweep must still close the connection instead of holding it forever.
func TestWingIdleTimeoutClosesStalledDownload(t *testing.T) {
	body := strings.Repeat("s", 32<<20)
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		_, _ = w.Write([]byte(body))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, DefaultPreset: Bolt, IdleTimeout: 300 * time.Millisecond}, handler)
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetReadBuffer(64 << 10)
	}
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond) // stall past IdleTimeout and a sweep
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	got, _ := io.Copy(io.Discard, conn)
	if got >= int64(len(body)) {
		t.Fatalf("stalled connection was kept open and delivered all %d bytes", got)
	}
}
