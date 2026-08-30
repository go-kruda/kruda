//go:build linux || darwin

package kruda

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"syscall"
	"testing"
	"time"
)

func TestWingAsyncPipelineClosesAfterItsResponse(t *testing.T) {
	presets := []struct {
		name   string
		preset Preset
	}{
		{name: "Pool", preset: Arrow},
		{name: "Spawn", preset: Preset{Dispatch: Spawn}},
	}

	for _, tt := range presets {
		t.Run(tt.name, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			app := New(Wing())
			app.Get("/inline", func(c *Ctx) error { return c.Text("inline") }, Bolt)
			app.Get("/async", func(c *Ctx) error {
				close(started)
				<-release
				return c.Text("async")
			}, tt.preset)

			addr, stop := startWingApp(t, app)
			defer stop()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			pipeline := "GET /inline HTTP/1.1\r\nHost: test\r\n\r\n" +
				"GET /async HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
			if _, err := io.WriteString(conn, pipeline); err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("async handler did not start")
			}

			reader := bufio.NewReader(conn)
			assertWingPipelineResponse(t, reader, "inline")
			close(release)
			assertWingPipelineResponse(t, reader, "async")

			if err := conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
			var one [1]byte
			if _, err := conn.Read(one[:]); err == nil {
				t.Fatal("connection remained open after Connection: close response")
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				t.Fatal("connection did not close after async response")
			}
		})
	}
}

func assertWingPipelineResponse(t *testing.T, reader *bufio.Reader, wantBody string) {
	t.Helper()
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != wantBody {
		t.Fatalf("response = (%d, %q), want (200, %q)", resp.StatusCode, body, wantBody)
	}
}

func TestDirectSendRetainsConnectionWhileAsyncHandlerOwnsFD(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	if err := syscall.Close(fds[1]); err != nil {
		t.Fatal(err)
	}

	w, eng := newTestWorker(0)
	c := newTestConn(int32(fds[0]))
	c.pending = 1
	c.sendBuf = append(c.sendBuf, "prior response"...)
	w.conns[c.fd] = c

	w.directSend(c)

	if len(eng.closedFds) != 0 {
		t.Fatal("connection fd closed while async handler still owned it")
	}
	if w.conns[c.fd] == nil {
		t.Fatal("connection bookkeeping removed before async completion")
	}
}
