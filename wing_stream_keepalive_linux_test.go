//go:build linux

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestWingStreamKeepAliveResponses(t *testing.T) {
	app := New(Wing())
	opt := Stream.With(StreamKeepAlive())
	app.Get("/segment", func(c *Ctx) error {
		c.SetHeader("Content-Length", "4")
		return c.Status(http.StatusPartialContent).Stream(strings.NewReader("data"))
	}, opt)
	app.Get("/unknown", func(c *Ctx) error {
		return c.Stream(strings.NewReader("tail"))
	}, opt)
	app.Get("/short", func(c *Ctx) error {
		c.SetHeader("Content-Length", "4")
		return c.Stream(strings.NewReader("ab"))
	}, opt)
	addr, shutdown := startWingApp(t, app)
	defer shutdown()

	dial := func() net.Conn {
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		return conn
	}
	read := func(r *bufio.Reader, status int, closing bool, want string, incomplete bool) {
		t.Helper()
		res, err := http.ReadResponse(r, &http.Request{Method: http.MethodGet})
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != status || (err != nil) != incomplete || string(body) != want {
			t.Fatalf("got status %d, body %q, err %v", res.StatusCode, body, err)
		}
		if !incomplete && res.Close != closing {
			t.Fatalf("close = %v, want %v", res.Close, closing)
		}
	}
	conn := dial()
	br := bufio.NewReader(conn)
	const segment = "GET /segment HTTP/1.1\r\nHost: localhost\r\n\r\n"
	if _, err := io.WriteString(conn, segment+segment+"GET /unknown HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	read(br, http.StatusPartialContent, false, "data", false)
	read(br, http.StatusPartialContent, false, "data", false)
	read(br, http.StatusOK, true, "tail", false)

	conn = dial()
	if _, err := io.WriteString(conn, "GET /short HTTP/1.1\r\nHost: localhost\r\n\r\n"+segment); err != nil {
		t.Fatal(err)
	}
	read(bufio.NewReader(conn), http.StatusOK, false, "ab", true)
}

type disconnectTestReader struct {
	ctx     context.Context
	unblock <-chan struct{}
	started chan<- struct{}
}

func (r *disconnectTestReader) Read([]byte) (int, error) {
	close(r.started)
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	case <-r.unblock:
		return 0, io.EOF
	}
}

func TestWingStreamKeepAliveCancelsAfterQueuedRequest(t *testing.T) {
	app := New(Wing())
	reading := make(chan struct{})
	stopped := make(chan error, 1)
	unblock := make(chan struct{})
	app.Get("/media", func(c *Ctx) error {
		c.SetHeader("Content-Length", "8")
		err := c.Stream(&disconnectTestReader{ctx: c.Context(), started: reading, unblock: unblock})
		stopped <- err
		return err
	}, Stream.With(StreamKeepAlive()))
	addr, shutdown := startWingApp(t, app)
	defer shutdown()
	defer close(unblock)

	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	const get = "GET /media HTTP/1.1\r\nHost: localhost\r\n\r\n"
	if _, err := fmt.Fprint(conn, get); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reading:
	case <-time.After(2 * time.Second):
		t.Fatal("stream did not start reading")
	}
	if _, err := fmt.Fprint(conn, get); err != nil {
		t.Fatal(err)
	}
	// Let the watcher observe the queued request before the client disconnects.
	time.Sleep(150 * time.Millisecond)
	conn.Close()
	select {
	case err := <-stopped:
		if err != context.Canceled {
			t.Fatalf("stream read returned %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel the blocked stream")
	}
}

func TestWingStreamKeepAliveAdvertisesWhatItDoes(t *testing.T) {
	const get = "GET /x HTTP/1.1\r\nHost: localhost\r\n\r\n"
	for _, tc := range []struct {
		name, request string
		handler       HandlerFunc
		reuse         bool
	}{
		{"buffered response", get, func(c *Ctx) error { return c.Text("data") }, false},
		{"direct write", get, func(c *Ctx) error {
			w := c.ResponseWriter()
			w.Header().Set("Content-Length", "4")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("data"))
			return err
		}, false},
		// WriteHeader after the preamble is a no-op and must not drop the body.
		{"second WriteHeader", get, func(c *Ctx) error {
			w := c.ResponseWriter()
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			return c.Stream(strings.NewReader("data"))
		}, false},
		// An HTTP/1.0 client reuses only when the response says keep-alive.
		{"HTTP/1.0 keep-alive", "GET /x HTTP/1.0\r\nConnection: keep-alive\r\n\r\n", func(c *Ctx) error {
			c.SetHeader("Content-Length", "4")
			return c.Stream(strings.NewReader("data"))
		}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/x", tc.handler, Stream.With(StreamKeepAlive()))
			addr, shutdown := startWingApp(t, app)
			defer shutdown()
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(conn, tc.request); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil || string(body) != "data" {
				t.Fatalf("body %q, err %v", body, err)
			}
			if tc.reuse && res.Header.Get("Connection") != "keep-alive" {
				t.Fatal("reused connection without Connection: keep-alive")
			}
			if !tc.reuse && !res.Close {
				t.Fatal("closing response without Connection: close")
			}
		})
	}
}

type stepReader struct {
	first   string
	release <-chan struct{}
	err     error
}

func (r *stepReader) Read(p []byte) (int, error) {
	if r.first != "" {
		n := copy(p, r.first)
		r.first = r.first[n:]
		return n, nil
	}
	if r.release != nil {
		<-r.release
	}
	return 0, r.err
}

func TestWingStreamKeepAliveCloseFallbackMatchesStream(t *testing.T) {
	for _, tc := range []struct {
		name     string
		pipeline bool
		want     string
		handler  func(c *Ctx, release <-chan struct{}) error
	}{
		// Closing over unread input would reset the close-delimited body.
		{"queued request ends in EOF, not reset", true, "tail", func(c *Ctx, release <-chan struct{}) error {
			return c.Stream(&stepReader{first: "tail", release: release, err: io.EOF})
		}},
		{"write after copy error", false, "prefix+error", func(c *Ctx, _ <-chan struct{}) error {
			if c.Stream(&stepReader{first: "prefix", err: io.ErrUnexpectedEOF}) == nil {
				return nil
			}
			_, err := c.ResponseWriter().Write([]byte("+error"))
			return err
		}},
		{"write after failed empty copy", false, "data", func(c *Ctx, _ <-chan struct{}) error {
			c.SetHeader("Content-Length", "4")
			if c.Stream(&stepReader{err: io.ErrUnexpectedEOF}) == nil {
				return nil
			}
			_, err := c.ResponseWriter().Write([]byte("data"))
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(Wing())
			release := make(chan struct{})
			app.Get("/x", func(c *Ctx) error { return tc.handler(c, release) }, Stream.With(StreamKeepAlive()))
			addr, shutdown := startWingApp(t, app)
			defer shutdown()
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			const get = "GET /x HTTP/1.1\r\nHost: localhost\r\n\r\n"
			if _, err := io.WriteString(conn, get); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodGet})
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if tc.pipeline {
				if _, err := io.WriteString(conn, get); err != nil {
					t.Fatal(err)
				}
				time.Sleep(150 * time.Millisecond) // let the request reach the kernel queue
			}
			close(release)
			body, err := io.ReadAll(res.Body)
			if err != nil || string(body) != tc.want || !res.Close {
				t.Fatalf("body %q, close %v, err %v; want %q and a clean close", body, res.Close, err, tc.want)
			}
		})
	}
}

// A close-delimited response is not reused. A request that arrives before the
// connection closes must be drained by the final drain the Takeover loop runs,
// or the close resets the connection and cuts the response.
func TestWingStreamKeepAliveDrainsUntilClose(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	client, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	server, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	f, err := server.(*net.TCPConn).File()
	server.Close()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w, _ := newTestWorker(0)
	w.handler = transport.HandlerFunc(func(rw transport.ResponseWriter, _ transport.Request) {
		_, _ = rw.Write([]byte("tail")) // no length, so close-delimited
	})
	req := acquireRequest()
	defer releaseRequest(req)
	req.ctx = context.Background()
	if reuse, sent, delimited := w.writeKeepAliveStream(req, f); reuse || !sent || !delimited {
		t.Fatalf("reuse %v sent %v close-delimited %v; want a sent close-delimited response, not reused", reuse, sent, delimited)
	}
	if _, err := io.WriteString(client, "GET /next HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // the request reaches the server
	raw, err := f.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var fd int32
	if err := raw.Control(func(v uintptr) { fd = int32(v) }); err != nil { // Fd() would make it blocking
		t.Fatal(err)
	}
	lingerTakeover(f, fd, make([]byte, 4096), true) // as takeoverLoop does before the close
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if body, err := io.ReadAll(client); err != nil || !strings.HasSuffix(string(body), "tail") {
		t.Fatalf("body %q, err %v; want a clean EOF", body, err)
	}
}

// TestWingStreamKeepAliveUnframedDrainsDuringStream sends more input than the
// final drain may discard while a close-delimited StreamKeepAlive response is
// still streaming. Once its preamble says close, the input must be drained as
// it arrives, as ordinary Stream does; left queued, the close resets the
// connection and cuts the tail.
func TestWingStreamKeepAliveUnframedDrainsDuringStream(t *testing.T) {
	sent, release := make(chan struct{}), make(chan struct{})
	app := New(Wing())
	app.Get("/live", func(c *Ctx) error {
		w := c.ResponseWriter()
		_, _ = w.Write([]byte("head"))
		close(sent)
		<-release
		_, _ = w.Write([]byte("tail"))
		return nil
	}, Stream.With(StreamKeepAlive()))
	addr, shutdown := startWingApp(t, app)
	defer shutdown()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.WriteString(conn, "GET /live HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		select {
		case <-release:
		default:
			close(release) // a failure below must not leave the handler waiting
		}
	}()
	select {
	case <-sent:
	case <-time.After(3 * time.Second):
		t.Fatal("stream never started")
	}
	junk := make([]byte, 2*maxDiscardInput)
	if _, err := conn.Write(junk); err != nil { // it fits only if the server reads it
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	close(release)
	time.Sleep(100 * time.Millisecond) // a reset reaches the client before it reads
	body, err := io.ReadAll(conn)
	if err != nil || !strings.HasSuffix(string(body), "headtail") {
		t.Fatalf("got %q, %v; want the whole stream and a clean EOF", body[max(0, len(body)-40):], err)
	}
}

// TestWingStreamKeepAliveNeedsTheCopyToFinishTheBody completes a declared
// body outside c.Stream, or panics after an exact copy. Neither handler
// finished as reuse requires, so the pipelined request must not run.
func TestWingStreamKeepAliveNeedsTheCopyToFinishTheBody(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		handler    HandlerFunc
	}{
		{"short copy, then a direct write", "abcd", 200, func(c *Ctx) error {
			c.SetHeader("Content-Length", "4")
			err := c.Stream(strings.NewReader("ab"))
			_, _ = c.ResponseWriter().Write([]byte("cd"))
			return err
		}},
		{"panic after an exact copy", "abcd", 200, func(c *Ctx) error {
			c.SetHeader("Content-Length", "4")
			_ = c.Stream(strings.NewReader("abcd"))
			panic("after the copy")
		}},
		{"panic before any response", `{"code":500,"message":"internal server error"}`, 500, func(c *Ctx) error {
			panic("before the response")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := tc.handler
			app := New(Wing())
			var ranNext atomic.Int32
			app.Get("/s", handler, Stream.With(StreamKeepAlive()))
			app.Get("/next", func(c *Ctx) error { ranNext.Add(1); return c.Text("next") })
			addr, shutdown := startWingApp(t, app)
			defer shutdown()
			conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.WriteString(conn, "GET /s HTTP/1.1\r\nHost: x\r\n\r\nGET /next HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			br := bufio.NewReader(conn)
			res, err := http.ReadResponse(br, nil)
			if err != nil {
				t.Fatal(err)
			}
			if body, err := io.ReadAll(res.Body); err != nil || res.StatusCode != tc.status || strings.TrimSpace(string(body)) != tc.body {
				t.Fatalf("got %d %q, %v; want %d %q in full", res.StatusCode, body, err, tc.status, tc.body)
			}
			if _, err := br.ReadByte(); err != io.EOF || ranNext.Load() != 0 {
				t.Fatalf("after the response: %v, next ran %d times; want the connection closed", err, ranNext.Load())
			}
		})
	}
}
