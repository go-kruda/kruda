//go:build linux || darwin

package kruda

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingResponseConnectionClose pipelines a second request behind a response
// that asks to close. The second must not run, whatever the dispatch.
func TestWingResponseConnectionClose(t *testing.T) {
	set := func(h transport.HeaderMap) { h.Set("Connection", "close") }
	for _, tc := range []struct {
		name   string
		preset Preset
		header func(transport.HeaderMap)
		split  bool // the body arrives after the headers
		stream bool // the body is a file stream
		reuse  bool
	}{
		{name: "Inline", preset: Bolt, header: set},
		{name: "Pool", preset: Arrow, header: set},
		{name: "Spawn", preset: Preset{Dispatch: Spawn}, header: set},
		{name: "Takeover", preset: Spear, header: set},
		{name: "Inline split body", preset: Bolt, header: set, split: true},
		{name: "Pool split body", preset: Arrow, header: set, split: true},
		{name: "Inline file stream", preset: Bolt, header: set, stream: true},
		{name: "Pool file stream", preset: Arrow, header: set, stream: true},
		{name: "Takeover file stream", preset: Spear, header: set, stream: true},
		{name: "token list", preset: Bolt, header: func(h transport.HeaderMap) { h.Set("connection", "x-hop, Close") }},
		{name: "second line", preset: Bolt, header: func(h transport.HeaderMap) {
			h.Add("Connection", "x-hop")
			h.Add("Connection", "close")
		}},
		{name: "past eight headers", preset: Bolt, header: func(h transport.HeaderMap) {
			for i := range 8 {
				h.Set("X-Fill-"+strconv.Itoa(i), "v")
			}
			h.Set("Connection", "close")
		}},
		{name: "keep-alive is kept", preset: Bolt, reuse: true, header: func(h transport.HeaderMap) { h.Set("Connection", "keep-alive") }},
		{name: "close is only a substring", preset: Spear, reuse: true, header: func(h transport.HeaderMap) { h.Set("Connection", "x-closed") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ranSecond atomic.Int32
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/second" {
					ranSecond.Add(1)
					_, _ = w.Write([]byte("second"))
					return
				}
				tc.header(w.Header())
				if tc.stream {
					w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader("first")), 5)
					return
				}
				_, _ = w.Write([]byte("first"))
			})
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: tc.preset}, handler)
			defer stop()
			conn, br := framingDial(t, addr)
			first, rest := framingGet("/first"), framingGet("/second")
			if tc.split {
				first, rest = "POST /first HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\n", "body"+rest
			}
			var err error
			if tc.split {
				if _, err = io.WriteString(conn, first); err == nil {
					time.Sleep(50 * time.Millisecond)
					_, err = io.WriteString(conn, rest)
				}
			} else {
				_, err = io.WriteString(conn, first+rest)
			}
			if err != nil {
				t.Fatal(err)
			}
			read := func(want string) {
				res, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatalf("%s: %v", want, err)
				}
				body, err := io.ReadAll(res.Body)
				if err != nil || string(body) != want {
					t.Fatalf("body %q err %v, want %q", body, err, want)
				}
			}
			read("first")
			if tc.reuse {
				read("second")
				return
			}
			_, err = br.ReadByte()
			var ne net.Error
			if err == nil || errors.As(err, &ne) && ne.Timeout() || ranSecond.Load() != 0 {
				t.Fatalf("connection not closed: err %v, second request ran %d times", err, ranSecond.Load())
			}
		})
	}
}

// TestWingNoRequestRunsAfterFinalResponse sends another request while a final
// response is still draining. It must neither run nor revive the connection.
func TestWingNoRequestRunsAfterFinalResponse(t *testing.T) {
	big := strings.Repeat("b", 16<<20)
	for _, tc := range []struct {
		name          string
		preset        Preset
		requestClose  bool
		responseClose bool
	}{
		{"Inline request close", Bolt, true, false},
		{"Inline response close", Bolt, false, true},
		{"Pool request close", Arrow, true, false},
		{"Pool response close", Arrow, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ranSecond atomic.Int32
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/second" {
					ranSecond.Add(1)
					_, _ = w.Write([]byte("second"))
					return
				}
				if tc.responseClose {
					w.Header().Set("Connection", "close")
				}
				_, _ = w.Write([]byte(big))
			})
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: tc.preset}, handler)
			defer stop()
			conn, br := framingDial(t, addr)
			first := framingGet("/first")
			if tc.requestClose {
				first = strings.TrimSuffix(first, "\r\n") + "Connection: close\r\n\r\n"
			}
			if _, err := io.WriteString(conn, first); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(conn, framingGet("/second")); err != nil {
				t.Fatal(err)
			}
			time.Sleep(100 * time.Millisecond) // it arrives while the body drains
			body, err := io.ReadAll(res.Body)
			if err != nil || len(body) != len(big) {
				t.Fatalf("final response cut at %d of %d: %v", len(body), len(big), err)
			}
			_, err = br.ReadByte()
			var ne net.Error
			if err == nil || errors.As(err, &ne) && ne.Timeout() || ranSecond.Load() != 0 {
				t.Fatalf("connection not closed: err %v, second request ran %d times", err, ranSecond.Load())
			}
		})
	}
}

// TestWingFinalSendKeepsBufferOfRunningHandler makes a send ready while a
// handler still runs. Its request may alias readBuf, so nothing may read into it.
func TestWingFinalSendKeepsBufferOfRunningHandler(t *testing.T) {
	socks, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(socks[0])
	defer syscall.Close(socks[1])
	if err := syscall.SetNonblock(socks[0], true); err != nil {
		t.Fatal(err)
	}
	if _, err := syscall.Write(socks[1], []byte("next request")); err != nil {
		t.Fatal(err)
	}
	w, _ := newTestWorker(0)
	c := newTestConn(int32(socks[0]))
	c.pending = 1
	c.sendBuf = []byte("earlier response")
	copy(c.readBuf, "/running")
	w.conns[c.fd] = c
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if got := string(c.readBuf[:len("/running")]); got != "/running" {
		t.Fatalf("readBuf overwritten while the handler runs: %q", got)
	}
}
