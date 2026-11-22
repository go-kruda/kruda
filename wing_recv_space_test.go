//go:build linux || darwin

package kruda

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingFullReadBufferWhileSending fills the read buffer behind a response
// the client is not reading yet, then makes the socket readable again.
func TestWingFullReadBufferWhileSending(t *testing.T) {
	const pipelined = 700 // more than one read buffer of requests
	next := "GET /next HTTP/1.1\r\nHost: x\r\n\r\n"
	get := "GET /big HTTP/1.1\r\nHost: x\r\n\r\n"
	post := "POST /big HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\n"
	batch := strings.Repeat(next, pipelined)
	for _, tc := range []struct {
		name               string
		preset             Preset
		first, fill, again string
		responses          int // 0: only the server's survival matters
		halfClose          bool
	}{
		{"takeover waits for the send", Spear, get, batch, next, pipelined + 4, false},
		{"takeover then half-close", Spear, get, batch, next, pipelined + 4, true},
		{"split body then Inline", Bolt, post, "body" + batch, next, pipelined + 4, false},
		{"split body then half-close", Bolt, post, "body" + batch, next, pipelined + 4, true},
		{"split body then Pool", Arrow, post, "body" + batch, next, pipelined + 4, false},
		{"split body then Pool half-close", Arrow, post, "body" + batch, next, pipelined + 4, true},
		{"split body then Spawn", Preset{Dispatch: Spawn}, post, "body" + batch, next, pipelined + 4, false},
		{"split body then Takeover", Spear, post, "body" + batch, next, pipelined + 4, false},
		{"header too large", Bolt, get, "GET /x HTTP/1.1\r\nX-Fill: " + strings.Repeat("a", 9000), "a", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			big := strings.Repeat("b", 16<<20)
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/big" {
					_, _ = w.Write([]byte(big))
					return
				}
				_, _ = w.Write([]byte("ok"))
			})
			addr, stop := startWingServerWithConfig(t, WingConfig{
				Workers:         1,
				HandlerPoolSize: 2,
				DefaultPreset:   Bolt,
				Presets:         map[string]Preset{"GET /next": tc.preset},
			}, handler)
			defer stop()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
			for i, part := range []string{tc.first, tc.fill, tc.again, tc.again, tc.again} {
				if i > 0 {
					time.Sleep(150 * time.Millisecond) // a separate read event for each part
				}
				if _, err := io.WriteString(conn, part); err != nil {
					t.Fatalf("write %d: %v", i, err)
				}
			}
			time.Sleep(150 * time.Millisecond)
			if tc.halfClose {
				if err := conn.(*net.TCPConn).CloseWrite(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(50 * time.Millisecond)
			}

			br := bufio.NewReader(conn)
			for i := range tc.responses {
				res, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatalf("response %d of %d: %v", i, tc.responses, err)
				}
				n, err := io.Copy(io.Discard, res.Body)
				want := int64(len("ok"))
				if i == 0 {
					want = int64(len(big))
				}
				if err != nil || n != want || res.StatusCode != 200 {
					t.Fatalf("response %d: status %d, %d of %d bytes, %v", i, res.StatusCode, n, want, err)
				}
			}
			if tc.halfClose {
				if _, err := br.ReadByte(); !errors.Is(err, io.EOF) {
					t.Fatalf("connection held after the client's half-close: %v", err)
				}
			}

			probe, err := net.DialTimeout("tcp", addr, time.Second)
			if err != nil {
				t.Fatalf("server gone: %v", err)
			}
			defer probe.Close()
			_ = probe.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err := io.WriteString(probe, "GET /ok HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := probe.Read(make([]byte, 64)); err != nil {
				t.Fatalf("server stopped answering: %v", err)
			}
		})
	}
}

// TestWingRecvLeavesInputWhenBufferIsFull needs no timing: the socket is
// readable and the buffer has no room.
func TestWingRecvLeavesInputWhenBufferIsFull(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[0], true); err != nil {
		t.Fatal(err)
	}
	if _, err := syscall.Write(fds[1], []byte("next")); err != nil {
		t.Fatal(err)
	}
	c := &conn{fd: int32(fds[0]), keepAlive: true, readBuf: make([]byte, 64), readN: 64, sendBuf: []byte("queued")}
	w := &worker{conns: map[int32]*conn{c.fd: c}}
	w.handleRecv(event{Fd: c.fd})
	if !c.readSuppressed || c.readN != len(c.readBuf) || string(c.sendBuf) != "queued" {
		t.Fatalf("suppressed=%v readN=%d sendBuf=%q", c.readSuppressed, c.readN, c.sendBuf)
	}
	var got [8]byte
	if n, err := syscall.Read(fds[0], got[:]); err != nil || string(got[:n]) != "next" {
		t.Fatalf("socket input consumed: %q %v", got[:max(n, 0)], err)
	}
}
