//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// sendRaw opens a loopback conn to a Wing server at addr, writes raw bytes,
// and returns the first HTTP response status line + whether the peer closed.
func sendRaw(t *testing.T, addr, raw string) (status string, closed bool) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", true // peer closed with no response
	}
	return strings.TrimSpace(line), false
}

func TestWingConfig_DerivesReadBufFromHeaderLimit(t *testing.T) {
	c := WingConfig{HeaderLimit: 16384}
	c.defaults()
	if c.ReadBufSize < 16384 {
		t.Fatalf("ReadBufSize=%d, want >= HeaderLimit 16384", c.ReadBufSize)
	}
	if c.MaxHeaderSize != 16384 {
		t.Fatalf("MaxHeaderSize=%d, want 16384", c.MaxHeaderSize)
	}
}

// postRaw builds a POST with an explicit Content-Length body of n 'x' bytes.
func postRaw(path string, n int) string {
	body := strings.Repeat("x", n)
	return fmt.Sprintf("POST %s HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n%s", path, n, body)
}

func TestClassifyIncomplete(t *testing.T) {
	lim := parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192, bodyLimit: 1024}

	// headers complete, body not yet arrived, within limit -> NeedBody(need)
	raw := []byte("POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: 500\r\n\r\n")
	st, need, _ := classifyIncomplete(raw, lim)
	if st != parseNeedBody || need != 500 {
		t.Fatalf("got (%v,%d) want (NeedBody,500)", st, need)
	}

	// content-length over bodyLimit -> BodyTooLarge
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: 99999\r\n\r\n")
	if st, _, _ := classifyIncomplete(raw, lim); st != parseBodyTooLarge {
		t.Fatalf("got %v want BodyTooLarge", st)
	}

	// chunked body -> Chunked
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\nTransfer-Encoding: chunked\r\n\r\n")
	if st, _, _ := classifyIncomplete(raw, lim); st != parseChunked {
		t.Fatalf("got %v want Chunked", st)
	}

	// no end-of-headers yet, buffer not full -> NeedHeaderMore
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\n")
	if st, _, _ := classifyIncomplete(raw, lim); st != parseNeedHeaderMore {
		t.Fatalf("got %v want NeedHeaderMore", st)
	}
}

func TestWingStatusClose(t *testing.T) {
	for _, code := range []int{413, 431, 501} {
		b := string(wingStatusClose(code))
		if !strings.Contains(b, fmt.Sprintf(" %d ", code)) || !strings.Contains(b, "Connection: close") {
			t.Fatalf("status %d: bad response %q", code, b)
		}
	}
}

// TestWing_Expect100_OversizedGets413BeforeBody tests that Expect: 100-continue
// with Content-Length > BodyLimit results in 413 immediately (before body arrives).
func TestWing_Expect100_OversizedGets413BeforeBody(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	})
	addr, stop := startBodyLimitWingServer(t, 1024, handler)
	defer stop()
	// Expect: 100-continue + oversized CL: server must answer 413 without waiting for a body.
	raw := "POST /u HTTP/1.1\r\nHost: h\r\nExpect: 100-continue\r\nContent-Length: 100000\r\n\r\n"
	st, _ := sendRaw(t, addr, raw)
	if !strings.Contains(st, "413") {
		t.Fatalf("expect+oversized: want 413, got %q", st)
	}
}

// startBodyLimitWingServer starts a Wing server with the given body limit.
func startBodyLimitWingServer(t *testing.T, bodyLimit int, handler transport.Handler) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cfg := WingConfig{
		Workers:     1,
		RingSize:    256,
		ReadBufSize: 8192,
		BodyLimit:   bodyLimit,
	}
	cfg.defaults()
	tr := NewWingTransport(cfg)
	errCh := make(chan error, 1)
	go func() { errCh <- tr.ListenAndServe(addr, handler) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return addr, func() { tr.Shutdown(context.Background()) }
}

// TestWing_AcceptsLegalBodyOverBuffer sends a 64KB POST body to a server with
// a 1MB limit, verifying that Wing correctly accumulates across multiple recvs
// and passes the body to the handler (response 200).
func TestWing_AcceptsLegalBodyOverBuffer(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	})
	addr, stop := startBodyLimitWingServer(t, 1<<20, handler)
	defer stop()

	st, _ := sendRaw(t, addr, postRaw("/u", 64*1024))
	if !strings.Contains(st, "200") {
		t.Fatalf("64KB body: want 200, got %q", st)
	}
}

// TestWing_BodyLimitBoundary checks the exact boundary: a body at-limit gets 200,
// a body one byte over limit gets 413.
func TestWing_BodyLimitBoundary(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	})
	addr, stop := startBodyLimitWingServer(t, 16*1024, handler)
	defer stop()

	if st, _ := sendRaw(t, addr, postRaw("/u", 16*1024)); !strings.Contains(st, "200") {
		t.Fatalf("exact limit: want 200, got %q", st)
	}
	if st, _ := sendRaw(t, addr, postRaw("/u", 16*1024+1)); !strings.Contains(st, "413") {
		t.Fatalf("limit+1: want 413, got %q", st)
	}
}

// readStatusLine reads just the first line of an HTTP response.
func readStatusLine(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	return strings.TrimSpace(line), err
}

// TestWing_SlowBodyTimesOut verifies that a connection stalled mid-body
// is closed by the read timeout's sweep mechanism.
func TestWing_SlowBodyTimesOut(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		_, _ = r.Body()
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	// 100ms read timeout
	cfg := WingConfig{
		Workers:     1,
		RingSize:    256,
		ReadBufSize: 8192,
		BodyLimit:   1 << 20,
		ReadTimeout: 100 * time.Millisecond,
	}
	cfg.defaults()
	tr := NewWingTransport(cfg)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	errCh := make(chan error, 1)
	go func() { errCh <- tr.ListenAndServe(addr, handler) }()
	// Wait for server ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err2 := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err2 == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer tr.Shutdown(context.Background())

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send headers promising 100KB but never send the body.
	_, err = fmt.Fprintf(conn, "POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: 100000\r\n\r\n")
	if err != nil {
		t.Fatalf("write headers: %v", err)
	}

	// Expect the server to close within ~3× the read timeout.
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected server to close the stalled body connection")
	}
}

// TestWing_ChunkedRejected501 verifies that Transfer-Encoding: chunked
// request bodies are rejected with 501 Not Implemented (HTTP/1.1 does
// not support chunked requests, only responses).
func TestWing_ChunkedRejected501(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	})
	addr, stop := startBodyLimitWingServer(t, 1<<20, handler)
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	raw := "POST /u HTTP/1.1\r\nHost: h\r\nTransfer-Encoding: chunked\r\n\r\n5\r\nhello\r\n0\r\n\r\n"
	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read the entire response to avoid race conditions with server close.
	r := bufio.NewReader(conn)
	status, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	status = strings.TrimSpace(status)
	if !strings.Contains(status, "501") {
		t.Fatalf("chunked body: want 501, got %q", status)
	}
}

// startWingServerWithConfig starts a Wing server with a fully-specified WingConfig.
func startWingServerWithConfig(t *testing.T, cfg WingConfig, handler transport.Handler) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find port: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	cfg.defaults()
	tr := NewWingTransport(cfg)
	go tr.ListenAndServe(addr, handler)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return addr, func() { tr.Shutdown(context.Background()) }
}

// readHTTPBody reads all response data from conn and returns the body after the
// blank header line.
func readHTTPBody(t *testing.T, conn net.Conn) string {
	t.Helper()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, err := conn.Read(tmp)
		buf.Write(tmp[:n])
		if err != nil {
			break
		}
	}
	resp := buf.String()
	idx := strings.Index(resp, "\r\n\r\n")
	if idx < 0 {
		return resp
	}
	return strings.TrimSpace(resp[idx+4:])
}

func TestWing_TrustProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	makeServer := func(trust bool) (string, func()) {
		cfg := WingConfig{
			Workers:     1,
			ReadBufSize: 8192,
			TrustProxy:  trust,
		}
		addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
			ip := r.RemoteAddr()
			w.WriteHeader(200)
			w.Write([]byte(ip))
		}))
		return addr, stop
	}

	t.Run("trusted_xff_multi", func(t *testing.T) {
		addr, stop := makeServer(true)
		defer stop()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For: 1.2.3.4, 5.6.7.8\r\n\r\n")
		body := readHTTPBody(t, conn)
		if body != "1.2.3.4" {
			t.Fatalf("expected 1.2.3.4, got %q", body)
		}
	})

	t.Run("trusted_xff_single", func(t *testing.T) {
		addr, stop := makeServer(true)
		defer stop()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For:  1.2.3.4 \r\n\r\n")
		body := readHTTPBody(t, conn)
		if body != "1.2.3.4" {
			t.Fatalf("expected trimmed 1.2.3.4, got %q", body)
		}
	})

	t.Run("trusted_xri", func(t *testing.T) {
		addr, stop := makeServer(true)
		defer stop()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: h\r\nX-Real-IP: 9.9.9.9\r\n\r\n")
		body := readHTTPBody(t, conn)
		if body != "9.9.9.9" {
			t.Fatalf("expected 9.9.9.9, got %q", body)
		}
	})

	t.Run("untrusted_ignores_xff", func(t *testing.T) {
		addr, stop := makeServer(false)
		defer stop()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For: 1.2.3.4\r\n\r\n")
		body := readHTTPBody(t, conn)
		// body should be 127.0.0.1:PORT (loopback socket addr), not 1.2.3.4
		if body == "1.2.3.4" {
			t.Fatalf("TrustProxy=false must not use XFF; got %q", body)
		}
		if !strings.HasPrefix(body, "127.0.0.1:") && !strings.HasPrefix(body, "[::1]:") {
			t.Fatalf("expected loopback addr, got %q", body)
		}
	})
}
