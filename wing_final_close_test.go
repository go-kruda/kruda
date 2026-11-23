//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// TestWingFinalResponseSurvivesUnreadInput queues a request in the kernel while
// a final response is produced. Closing over that input resets the connection,
// which can cut the response: the client must get all of it, then a clean EOF.
func TestWingFinalResponseSurvivesUnreadInput(t *testing.T) {
	small, big := "first", strings.Repeat("b", 8<<20)
	for _, tc := range []struct {
		name         string
		preset       Preset
		body         string
		stream       bool
		requestClose bool
	}{
		{name: "Inline", preset: Bolt, body: small},
		{name: "Inline request close", preset: Bolt, body: small, requestClose: true},
		{name: "Inline large", preset: Bolt, body: big},
		{name: "Inline file stream", preset: Bolt, body: small, stream: true},
		{name: "Pool", preset: Arrow, body: small},
		{name: "Pool large", preset: Arrow, body: big},
		{name: "Spawn", preset: Preset{Dispatch: Spawn}, body: small},
		{name: "Takeover", preset: Spear, body: small},
		{name: "Takeover request close", preset: Spear, body: small, requestClose: true},
		{name: "Takeover large", preset: Spear, body: big},
		{name: "Takeover file stream", preset: Spear, body: small, stream: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entered, release := make(chan struct{}, 1), make(chan struct{})
			var ranSecond atomic.Int32
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() != "/first" {
					ranSecond.Add(1)
					return
				}
				entered <- struct{}{}
				<-release
				if !tc.requestClose {
					w.Header().Set("Connection", "close")
				}
				if tc.stream {
					w.(transport.FileStreamer).SetFileStream(io.NopCloser(strings.NewReader(tc.body)), int64(len(tc.body)))
					return
				}
				_, _ = w.Write([]byte(tc.body))
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
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("handler never ran")
			}
			if _, err := io.WriteString(conn, framingGet("/second")); err != nil {
				t.Fatal(err)
			}
			time.Sleep(50 * time.Millisecond) // the request reaches the server's receive queue
			close(release)
			time.Sleep(100 * time.Millisecond) // a reset reaches the client before it reads
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatalf("response lost: %v", err)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil || string(body) != tc.body {
				t.Fatalf("response cut at %d of %d bytes: %v", len(body), len(tc.body), err)
			}
			if _, err := br.ReadByte(); err != io.EOF {
				t.Fatalf("after the response: %v, want a clean EOF", err)
			}
			if n := ranSecond.Load(); n != 0 {
				t.Fatalf("request behind the final response ran %d times", n)
			}
		})
	}
}

// TestWingDiscardBehindFinalResponseIsBounded keeps sending while a final
// response waits to be sent. Dropping that input costs the worker a copy per
// byte, so the connection closes once the peer has sent too much of it.
func TestWingDiscardBehindFinalResponseIsBounded(t *testing.T) {
	w, c, peer, _ := socketWorker(t, 4096, nil)
	c.keepAlive = false
	c.sendBuf = []byte("final response the peer has not read")
	chunk := make([]byte, 16<<10)
	sent := 0
	for w.conns[c.fd] == c && sent <= maxDiscardInput+len(chunk) {
		n, err := peer.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		sent += n
		w.handleRecv(event{Op: opRecv, Fd: c.fd})
	}
	if w.conns[c.fd] == c {
		t.Fatalf("still open after the peer sent %d bytes behind its final response", sent)
	}
}

// TestWingFinalCloseLingersOverUnreadInput finishes a final response with input
// still unread. Wing must end its side, wait for the peer to close, and give up
// after lingerDelay.
func TestWingFinalCloseLingersOverUnreadInput(t *testing.T) {
	const response = "HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
	for _, peerCloses := range []bool{true, false} {
		w, c, peer, _ := socketWorker(t, 4096, nil)
		w.hasTimeout = true
		c.keepAlive = false
		c.sendBuf = []byte(response)
		if _, err := io.WriteString(peer, framingGet("/unread")); err != nil {
			t.Fatal(err)
		}
		w.directSend(c)
		if w.conns[c.fd] != c {
			t.Fatal("closed over unread input")
		}
		if got, err := io.ReadAll(peer); err != nil || string(got) != response {
			t.Fatalf("peer read %q, %v; want the response and EOF", got, err)
		}
		if peerCloses {
			peer.Close()
			w.handleRecv(event{Op: opRecv, Fd: c.fd})
			if w.conns[c.fd] == c {
				t.Fatal("still open after the peer closed")
			}
			continue
		}
		w.sweepTimeouts(time.Now().UnixNano())
		if w.conns[c.fd] != c {
			t.Fatal("closed before lingerDelay")
		}
		w.sweepTimeouts(time.Now().UnixNano() + lingerDelay + 1)
		if w.conns[c.fd] == c {
			t.Fatal("still open after lingerDelay")
		}
	}
}

// TestWingStaticResponseCloseEndsConnection pipelines a request behind a raw
// static response that says Connection: close. That request must not run.
func TestWingStaticResponseCloseEndsConnection(t *testing.T) {
	static := []byte("HTTP/1.1 200 OK\r\nContent-Length: 6\r\nConnection: close\r\n\r\nstatic")
	for _, tc := range []struct {
		name  string
		paths []string
	}{
		{"fresh connection", []string{"/static"}},
		{"after Takeover", []string{"/db", "/static"}},
		{"split body", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ranNext atomic.Int32
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
				if r.Path() == "/next" {
					ranNext.Add(1)
				}
				_, _ = w.Write([]byte(strings.TrimPrefix(r.Path(), "/")))
			})
			cfg := WingConfig{Workers: 1, DefaultPreset: Bolt, Presets: map[string]Preset{
				"GET /db":      Spear,
				"GET /static":  Bolt.With(Static(static)),
				"POST /static": Bolt.With(Static(static)),
			}}
			addr, stop := startWingServerWithConfig(t, cfg, handler)
			defer stop()
			conn, br := framingDial(t, addr)
			var err error
			if tc.paths == nil {
				if _, err = io.WriteString(conn, "POST /static HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\n"); err == nil {
					time.Sleep(50 * time.Millisecond)
					_, err = io.WriteString(conn, "body"+framingGet("/next"))
				}
			} else {
				var reqs string
				for _, p := range tc.paths {
					reqs += framingGet(p)
				}
				_, err = io.WriteString(conn, reqs+framingGet("/next"))
			}
			if err != nil {
				t.Fatal(err)
			}
			wants := []string{"static"}
			if len(tc.paths) == 2 {
				wants = []string{"db", "static"}
			}
			for _, want := range wants {
				res, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatalf("%s: %v", want, err)
				}
				body, err := io.ReadAll(res.Body)
				if err != nil || string(body) != want {
					t.Fatalf("body %q err %v, want %q", body, err, want)
				}
			}
			if _, err := br.ReadByte(); err != io.EOF || ranNext.Load() != 0 {
				t.Fatalf("connection not closed: err %v, next request ran %d times", err, ranNext.Load())
			}
		})
	}
}

// TestWingStaticStreamRouteAfterTakeover reaches a static route registered with
// the Stream preset through a Takeover connection. The prebuilt response wins,
// as it does on a fresh connection.
func TestWingStaticStreamRouteAfterTakeover(t *testing.T) {
	static := []byte("HTTP/1.1 200 OK\r\nContent-Length: 6\r\n\r\nstatic")
	var ranStatic atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/static" {
			ranStatic.Add(1)
		}
		_, _ = w.Write([]byte(strings.TrimPrefix(r.Path(), "/")))
	})
	cfg := WingConfig{Workers: 1, DefaultPreset: Bolt, Presets: map[string]Preset{
		"GET /db":     Spear,
		"GET /static": Stream.With(Static(static)),
	}}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, framingGet("/db")+framingGet("/static")+framingGet("/next")); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"db", "static", "next"} {
		res, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("%s: %v", want, err)
		}
		body, err := io.ReadAll(res.Body)
		if err != nil || string(body) != want {
			t.Fatalf("body %q err %v, want %q", body, err, want)
		}
	}
	if n := ranStatic.Load(); n != 0 {
		t.Fatalf("static route ran its handler %d times", n)
	}
}

// TestWingShutdownDeliversFinishingTakeoverResponse finishes a Takeover handler
// after Shutdown started and while a Pool handler still holds it up. Its
// response must reach the client, as a Pool response does.
func TestWingShutdownDeliversFinishingTakeoverResponse(t *testing.T) {
	poolEntered, takeEntered := make(chan struct{}), make(chan struct{})
	poolRelease, takeRelease := make(chan struct{}), make(chan struct{})
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/pool" {
			close(poolEntered)
			<-poolRelease
		} else {
			close(takeEntered)
			<-takeRelease
		}
		_, _ = w.Write([]byte("late"))
	})
	cfg := WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Spear, Presets: map[string]Preset{"GET /pool": Arrow}}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	poolConn, _ := framingDial(t, addr)
	takeConn, br := framingDial(t, addr)
	if _, err := io.WriteString(poolConn, framingGet("/pool")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(takeConn, framingGet("/take")); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []chan struct{}{poolEntered, takeEntered} {
		select {
		case <-ch:
		case <-time.After(3 * time.Second):
			t.Fatal("handler never ran")
		}
	}
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	time.Sleep(100 * time.Millisecond) // Shutdown has started and waits for the Pool
	close(takeRelease)
	res, err := http.ReadResponse(br, nil)
	close(poolRelease)
	<-stopped
	if err != nil {
		t.Fatalf("Takeover response lost during shutdown: %v", err)
	}
	if body, err := io.ReadAll(res.Body); err != nil || string(body) != "late" {
		t.Fatalf("body %q err %v", body, err)
	}
}

// TestWingHalfCloseAfterRequestCloses sends a request and shuts the write side
// at once, so the FIN can arrive with the request. Wing must answer and close
// then, not hold the connection until a timeout.
func TestWingHalfCloseAfterRequestCloses(t *testing.T) {
	for name, preset := range map[string]Preset{"Inline": Bolt, "Pool": Arrow, "Takeover": Spear} {
		t.Run(name, func(t *testing.T) {
			handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { _, _ = w.Write([]byte("ok")) })
			addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: preset}, handler)
			defer stop()
			for i := range 30 {
				conn, br := framingDial(t, addr)
				_ = conn.SetDeadline(time.Now().Add(time.Second))
				if _, err := io.WriteString(conn, framingGet("/")); err != nil {
					t.Fatal(err)
				}
				if err := conn.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
					t.Fatal(err)
				}
				res, err := http.ReadResponse(br, nil)
				if err != nil {
					t.Fatalf("connection %d: %v", i, err)
				}
				if body, err := io.ReadAll(res.Body); err != nil || string(body) != "ok" {
					t.Fatalf("connection %d: body %q, %v", i, body, err)
				}
				if _, err := br.ReadByte(); err != io.EOF {
					t.Fatalf("connection %d held open after the half-close: %v", i, err)
				}
				conn.Close()
			}
		})
	}
}

// TestWingPeerShutdownReadinessIsKept reads a request whose FIN came in the same
// readiness event. That event reports no further edge, so the EOF must be read
// once the response is sent.
func TestWingPeerShutdownReadinessIsKept(t *testing.T) {
	h := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { _, _ = w.Write([]byte("ok")) })
	w, c, peer, _ := socketWorker(t, 4096, h)
	c.keepAlive = true
	if _, err := io.WriteString(peer, framingGet("/")); err != nil {
		t.Fatal(err)
	}
	if err := peer.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd, Flags: evPeerShut})
	if w.conns[c.fd] == c {
		t.Fatal("still open after answering a peer that shut its write side")
	}
}

// TestWingUploadEOFKeepsQueuedResponse ends the peer's input in the middle of a
// body while an earlier response is still queued. The response must stay
// queued and the connection must end once it is sent.
func TestWingUploadEOFKeepsQueuedResponse(t *testing.T) {
	big := strings.Repeat("b", 200<<10)
	h := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { _, _ = w.Write([]byte(big)) })
	w, c, peer, _ := socketWorker(t, 4096, h)
	c.keepAlive = true
	for _, fd := range []int{int(c.fd)} {
		if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := io.WriteString(peer, framingGet("/big")+"POST /upload HTTP/1.1\r\nHost: x\r\nContent-Length: 10\r\n\r\nabc"); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if c.bodyNeed == 0 || c.sendN >= len(c.sendBuf) {
		t.Skipf("setup: body %d, queued %d", c.bodyNeed, len(c.sendBuf)-c.sendN)
	}
	if err := peer.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd, Flags: evPeerShut})
	if w.conns[c.fd] != c || c.sendN >= len(c.sendBuf) {
		t.Fatal("the EOF in the upload dropped the queued response")
	}
	got := 0
	buf := make([]byte, 64<<10)
	for w.conns[c.fd] == c {
		n, err := peer.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		got += n
		w.handleSend(event{Op: opSend, Fd: c.fd})
	}
	if got < len(big) {
		t.Fatalf("peer got %d bytes, want the whole response", got)
	}
}
