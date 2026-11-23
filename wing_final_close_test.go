//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
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
		{name: "rejected Hijack", preset: Hijack, body: small, requestClose: true},
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
// response waits to be sent. Wing drops at most maxDiscardInput of that input
// in all, leaves the rest unread, still sends the response, and then closes.
func TestWingDiscardBehindFinalResponseIsBounded(t *testing.T) {
	w, c, peer, _ := socketWorker(t, 4096, nil)
	c.keepAlive = false
	const response = "final response the peer has not read"
	c.sendBuf = []byte(response)
	chunk := make([]byte, 16<<10)
	for sent := 0; sent <= maxDiscardInput+len(chunk); {
		n, err := peer.Write(chunk)
		if err != nil {
			t.Fatal(err)
		}
		sent += n
		w.handleRecv(event{Op: opRecv, Fd: c.fd})
		if c.discarded > maxDiscardInput {
			t.Fatalf("dropped %d bytes, over the %d budget", c.discarded, maxDiscardInput)
		}
	}
	if w.conns[c.fd] != c || c.discarded != maxDiscardInput {
		t.Fatalf("open %v, dropped %d: want the response still queued after %d", w.conns[c.fd] == c, c.discarded, maxDiscardInput)
	}
	w.handleSend(event{Op: opSend, Fd: c.fd})
	if w.conns[c.fd] == c {
		t.Fatal("still open after the response was sent past the budget")
	}
	got := make([]byte, len(response))
	if _, err := io.ReadFull(peer, got); err != nil || string(got) != response {
		t.Fatalf("peer read %q, %v", got, err)
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
// body while an earlier response is still queued. The response must arrive
// whole, and the connection must end once it is sent.
func TestWingUploadEOFKeepsQueuedResponse(t *testing.T) {
	for _, early := range []bool{false, true} {
		t.Run(map[bool]string{false: "EOF after the upload starts", true: "EOF before"}[early], func(t *testing.T) { uploadEOFKeepsQueuedResponse(t, early) })
	}
}

func uploadEOFKeepsQueuedResponse(t *testing.T, early bool) {
	big := strings.Repeat("b", 200<<10)
	h := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { _, _ = w.Write([]byte(big)) })
	w, c, peer, _ := socketWorker(t, 4096, h)
	c.keepAlive = true
	if err := syscall.SetsockoptInt(int(c.fd), syscall.SOL_SOCKET, syscall.SO_SNDBUF, 4096); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(peer, framingGet("/big")+"POST /upload HTTP/1.1\r\nHost: x\r\nContent-Length: 10\r\n\r\nabc"); err != nil {
		t.Fatal(err)
	}
	if early {
		// The EOF is already there when the upload starts: the body drain
		// finds it, not a later read event.
		if err := peer.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
			t.Fatal(err)
		}
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if !early {
		if c.bodyNeed == 0 || c.sendN >= len(c.sendBuf) {
			t.Fatalf("setup: want an upload behind a queued response, got body %d, queued %d", c.bodyNeed, len(c.sendBuf)-c.sendN)
		}
		if err := peer.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
			t.Fatal(err)
		}
		w.handleRecv(event{Op: opRecv, Fd: c.fd, Flags: evPeerShut})
	} else if w.conns[c.fd] != c || c.sendN >= len(c.sendBuf) {
		t.Fatal("the EOF found by the body drain dropped the queued response")
	}
	var got bytes.Buffer
	buf := make([]byte, 64<<10)
	for {
		if w.conns[c.fd] != c {
			_ = peer.SetReadDeadline(time.Now().Add(100 * time.Millisecond)) // the mock engine leaves the fd open
		}
		n, err := peer.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			if w.conns[c.fd] == c {
				t.Fatal(err)
			}
			break
		}
		if w.conns[c.fd] == c {
			w.handleSend(event{Op: opSend, Fd: c.fd})
		}
	}
	res, err := http.ReadResponse(bufio.NewReader(&got), nil)
	if err != nil {
		t.Fatalf("no whole response in %d bytes: %v", got.Len(), err)
	}
	if body, err := io.ReadAll(res.Body); err != nil || string(body) != big {
		t.Fatalf("response body %d of %d bytes: %v", len(body), len(big), err)
	}
	if w.conns[c.fd] == c {
		t.Fatal("still open after the response was sent")
	}
}

// TestWingLingerEndsWithoutTimeouts leaves a lingering connection open on the
// client side, with every Wing timeout off, and a limit of one connection.
// Wing must still close it, or the next client is never served.
func TestWingLingerEndsWithoutTimeouts(t *testing.T) {
	entered, release := make(chan struct{}, 1), make(chan struct{})
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/first" {
			entered <- struct{}{}
			<-release
			w.Header().Set("Connection", "close")
		}
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, MaxConns: 1}, handler)
	defer stop()
	var conn net.Conn
	var br *bufio.Reader
	for attempt := 0; conn == nil; attempt++ { // the start-up probe may still hold the slot
		if attempt == 20 {
			t.Fatal("never admitted")
		}
		c, r := framingDial(t, addr)
		if _, err := io.WriteString(c, framingGet("/first")); err != nil {
			t.Fatal(err)
		}
		select {
		case <-entered:
			conn, br = c, r
		case <-time.After(100 * time.Millisecond):
			c.Close()
		}
	}
	if _, err := io.WriteString(conn, framingGet("/second")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	if _, err := br.ReadByte(); err != io.EOF {
		t.Fatalf("after the response: %v, want EOF", err)
	}
	time.Sleep(2500 * time.Millisecond) // lingerDelay plus the sweep and wait intervals; conn stays open here
	next, nbr := framingDial(t, addr)
	if _, err := io.WriteString(next, framingGet("/next")); err != nil {
		t.Fatal(err)
	}
	if _, err := http.ReadResponse(nbr, nil); err != nil {
		t.Fatalf("next client not served, the lingering connection still holds the only slot: %v", err)
	}
}

// TestWingShutdownStartsNoTakeoverRequest sends a new request on an idle
// Takeover connection after Shutdown began, while a Pool handler still holds it
// up. That request must not run.
func TestWingShutdownStartsNoTakeoverRequest(t *testing.T) {
	poolEntered, poolRelease := make(chan struct{}), make(chan struct{})
	var ranTake atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/pool" {
			close(poolEntered)
			<-poolRelease
		} else {
			ranTake.Add(1)
		}
		_, _ = w.Write([]byte("ok"))
	})
	cfg := WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: Spear, Presets: map[string]Preset{"GET /pool": Arrow}}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	takeConn, br := framingDial(t, addr)
	if _, err := io.WriteString(takeConn, framingGet("/take")); err != nil {
		t.Fatal(err)
	}
	if res, err := http.ReadResponse(br, nil); err != nil {
		t.Fatal(err)
	} else {
		_, _ = io.ReadAll(res.Body)
	}
	poolConn, _ := framingDial(t, addr)
	if _, err := io.WriteString(poolConn, framingGet("/pool")); err != nil {
		t.Fatal(err)
	}
	<-poolEntered
	stopped := make(chan struct{})
	go func() { stop(); close(stopped) }()
	time.Sleep(100 * time.Millisecond) // Shutdown has started and waits for the Pool
	_, _ = io.WriteString(takeConn, framingGet("/take"))
	time.Sleep(100 * time.Millisecond)
	close(poolRelease)
	<-stopped
	if n := ranTake.Load(); n != 1 {
		t.Fatalf("Takeover ran %d requests, want only the one before Shutdown", n)
	}
}

// TestWingStaticHijackRouteWithSplitBody posts to a static route registered
// with the Hijack preset, the body after the headers. The prebuilt response
// must answer, as it does when the body comes with the headers.
func TestWingStaticHijackRouteWithSplitBody(t *testing.T) {
	static := []byte("HTTP/1.1 200 OK\r\nContent-Length: 6\r\n\r\nstatic")
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { _, _ = w.Write([]byte("handler")) })
	cfg := WingConfig{Workers: 1, DefaultPreset: Bolt, Presets: map[string]Preset{"POST /static": Hijack.With(Static(static))}}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	defer stop()
	for _, split := range []bool{false, true} {
		conn, br := framingDial(t, addr)
		head, body := "POST /static HTTP/1.1\r\nHost: x\r\nContent-Length: 4\r\n\r\n", "body"
		if !split {
			head, body = head+body, ""
		}
		if _, err := io.WriteString(conn, head); err != nil {
			t.Fatal(err)
		}
		if split {
			time.Sleep(50 * time.Millisecond)
			if _, err := io.WriteString(conn, body); err != nil {
				t.Fatal(err)
			}
		}
		res, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("split %v: %v", split, err)
		}
		if got, err := io.ReadAll(res.Body); err != nil || string(got) != "static" {
			t.Fatalf("split %v: %d %q %v", split, res.StatusCode, got, err)
		}
	}
}

// TestWingPoolFinalCloseLingers finishes a Pool response that the handler wrote
// itself, with input in the socket that no read event has reported yet.
func TestWingPoolFinalCloseLingers(t *testing.T) {
	w, c, peer, _ := socketWorker(t, 4096, nil)
	w.hasTimeout = true
	c.pending = 1
	if _, err := io.WriteString(peer, framingGet("/unread")); err != nil {
		t.Fatal(err)
	}
	w.handleDone(doneMsg{fd: c.fd, keepAlive: false})
	if w.conns[c.fd] != c || c.lingerUntil == 0 {
		t.Fatal("closed over input the Pool handler left unread")
	}
}

// TestWingLingeringConnectionRunsNothing sends a whole request to a connection
// that is lingering after its final response. It must not reach a handler.
func TestWingLingeringConnectionRunsNothing(t *testing.T) {
	var ran atomic.Int32
	h := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) { ran.Add(1) })
	w, c, peer, _ := socketWorker(t, 4096, h)
	w.hasTimeout = true
	c.keepAlive = false
	c.sendBuf = []byte("HTTP/1.1 200 OK\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
	if _, err := io.WriteString(peer, framingGet("/unread")); err != nil {
		t.Fatal(err)
	}
	w.directSend(c)
	if c.lingerUntil == 0 {
		t.Fatal("setup: not lingering")
	}
	if _, err := io.WriteString(peer, framingGet("/late")); err != nil {
		t.Fatal(err)
	}
	w.handleRecv(event{Op: opRecv, Fd: c.fd})
	if n := ran.Load(); n != 0 {
		t.Fatalf("a request on a lingering connection ran %d times", n)
	}
}

// TestWingLingerEndsWhenClientCloses closes the client side of a lingering
// connection, with a limit of one connection. Wing must notice at once, not
// at the linger deadline, so the next client is served right away.
func TestWingLingerEndsWhenClientCloses(t *testing.T) {
	entered, release := make(chan struct{}, 1), make(chan struct{})
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		if r.Path() == "/first" {
			entered <- struct{}{}
			<-release
			w.Header().Set("Connection", "close")
		}
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServerWithConfig(t, WingConfig{Workers: 1, MaxConns: 1}, handler)
	defer stop()
	var conn net.Conn
	var br *bufio.Reader
	for attempt := 0; conn == nil; attempt++ { // the start-up probe may still hold the slot
		if attempt == 20 {
			t.Fatal("never admitted")
		}
		c, r := framingDial(t, addr)
		if _, err := io.WriteString(c, framingGet("/first")); err != nil {
			t.Fatal(err)
		}
		select {
		case <-entered:
			conn, br = c, r
		case <-time.After(100 * time.Millisecond):
			c.Close()
		}
	}
	if _, err := io.WriteString(conn, framingGet("/second")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(res.Body)
	if _, err := br.ReadByte(); err != io.EOF {
		t.Fatalf("after the response: %v, want EOF", err)
	}
	conn.Close()
	time.Sleep(100 * time.Millisecond) // well inside lingerDelay
	next, nbr := framingDial(t, addr)
	if _, err := io.WriteString(next, framingGet("/next")); err != nil {
		t.Fatal(err)
	}
	if _, err := http.ReadResponse(nbr, nil); err != nil {
		t.Fatalf("next client not served after the lingering one closed: %v", err)
	}
}
