//go:build linux || darwin

package kruda

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingReadDeadlineWaitsForSend leaves part of the next request behind a
// reply still being sent. Wing does not read then, so the clock must not run.
func TestWingReadDeadlineWaitsForSend(t *testing.T) {
	w, eng := newTestWorker(0)
	w.readTimeout = int64(time.Second)
	c := newTestConn(42)
	c.keepAlive = true
	c.sendBuf = []byte("reply")
	c.readN = copy(c.readBuf, "GET /par")
	c.readDeadline = 100
	w.conns[c.fd] = c

	now := int64(5 * time.Second)
	w.sweepTimeouts(now)
	if len(eng.closedFds) != 0 {
		t.Fatal("read timeout cut a reply that was being sent")
	}
	c.sendN = len(c.sendBuf)
	w.sweepTimeouts(now + int64(time.Second)/2)
	if len(eng.closedFds) != 0 {
		t.Fatal("client got no time to finish the request after the send")
	}
	w.sweepTimeouts(now + 2*int64(time.Second))
	if len(eng.closedFds) != 1 {
		t.Fatal("partial request kept after the send and its timeout")
	}
}

// TestWingTimeoutsBoundPipelinedInput covers what a client can leave behind a
// complete request to hold a connection.
func TestWingTimeoutsBoundPipelinedInput(t *testing.T) {
	big := strings.Repeat("b", 16<<20)
	get := "GET /small HTTP/1.1\r\nHost: x\r\n\r\n"
	for _, tc := range []struct {
		name, payload string
		read, write   time.Duration
	}{
		{"partial header", get + "GET /par", time.Second, 0},
		{"partial body", get + "POST /p HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\nabc", time.Second, 0},
		{"unread response", "GET /big HTTP/1.1\r\nHost: x\r\n\r\n" + strings.Repeat(get, 50), 0, time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			addr, stop := startWingServerWithConfig(t, WingConfig{
				Workers: 1, DefaultPreset: Bolt, ReadTimeout: tc.read, WriteTimeout: tc.write,
			}, transport.HandlerFunc(func(rw transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/big" {
					_, _ = rw.Write([]byte(big))
					return
				}
				_, _ = rw.Write([]byte("ok"))
			}))
			defer stop()
			conn, br := framingDial(t, addr)
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			if _, err := io.WriteString(conn, tc.payload); err != nil {
				t.Fatal(err)
			}
			if tc.write > 0 {
				for range 35 { // keep sending, past the timeout and a sweep
					time.Sleep(100 * time.Millisecond)
					if _, err := io.WriteString(conn, get); err != nil {
						break
					}
				}
			}
			n, err := io.Copy(io.Discard, br)
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				t.Fatal("connection held past its timeout")
			}
			if tc.write > 0 && n >= int64(len(big)) {
				t.Fatalf("write timeout did not cut a client that read nothing: %d bytes", n)
			}
		})
	}
}
