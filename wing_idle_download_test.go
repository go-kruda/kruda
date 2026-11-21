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

func TestWingIdleTimeoutLeavesAccumulatingBodyToReadTimeout(t *testing.T) {
	w, _ := newTestWorker(0)
	w.idleTimeout = int64(time.Second)
	w.readTimeout = int64(5 * time.Second)
	c := newTestConn(12345)
	c.lastActive = 1
	c.readDeadline = c.lastActive + w.readTimeout
	c.bodyNeed = 32 << 10
	w.conns[c.fd] = c

	w.sweepTimeouts(c.lastActive + int64(2*time.Second))
	if w.conns[c.fd] != c {
		t.Fatal("idle timeout closed a request with a body in progress")
	}
	w.sweepTimeouts(c.readDeadline + 1)
	if w.conns[c.fd] != nil {
		t.Fatal("read timeout did not close the incomplete upload")
	}
}

func TestWingIdleTimeoutWithoutBodyReadDeadline(t *testing.T) {
	for _, readTimeout := range []time.Duration{0, 5 * time.Second} {
		t.Run(readTimeout.String(), func(t *testing.T) {
			w, _ := newTestWorker(0)
			w.idleTimeout = int64(time.Second)
			w.readTimeout = int64(readTimeout)
			c := newTestConn(12345)
			c.lastActive = 1
			c.bodyNeed = 32 << 10
			w.conns[c.fd] = c

			w.sweepTimeouts(c.lastActive + int64(2*time.Second))
			if w.conns[c.fd] != nil {
				t.Fatal("idle timeout did not close an upload without a read deadline")
			}
		})
	}
}
