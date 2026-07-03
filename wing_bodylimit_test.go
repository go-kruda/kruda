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

// TestWing_Takeover_BodyAndLimit exercises body-limit enforcement and legal body
// accumulation on the Takeover (DB/Spear) dispatch path specifically.
func TestWing_Takeover_BodyAndLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const bodyLimit = 16 * 1024

	cfg := WingConfig{
		Workers:       1,
		ReadBufSize:   8192,
		BodyLimit:     bodyLimit,
		DefaultPreset: DB, // Takeover dispatch
	}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		body, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(body))))
	}))
	defer stop()

	t.Run("takeover_legal_body", func(t *testing.T) {
		st, _ := sendRaw(t, addr, postRaw("/", bodyLimit/2))
		if !strings.Contains(st, "200") {
			t.Fatalf("legal body want 200, got %q", st)
		}
	})

	t.Run("takeover_over_limit_413", func(t *testing.T) {
		st, _ := sendRaw(t, addr, postRaw("/", bodyLimit+1))
		if !strings.Contains(st, "413") {
			t.Fatalf("over-limit on takeover path want 413, got %q", st)
		}
	})
}

// TestWing_ConcurrentPartialUploads_Bounded verifies that the per-worker in-flight
// body budget (MaxInflightBodyBytes = 64 * BodyLimit) prevents unbounded memory
// accumulation when many connections each promise a large body but never send it.
// After the flood is drained by ReadTimeout, the server must still respond normally.
func TestWing_ConcurrentPartialUploads_Bounded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}

	const bodyLimit = 1 << 20 // 1MB per request
	cfg := WingConfig{
		Workers:     1,
		ReadBufSize: 8192,
		BodyLimit:   bodyLimit,
		ReadTimeout: time.Second,
	}
	cfg.defaults()
	// MaxInflightBodyBytes is now derived: 64 * 1MB = 64MB.
	// Opening >64 connections each promising 1MB should hit the budget.

	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		_, _ = r.Body()
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer stop()

	// Open connections that each promise 1MB body but never send it.
	var conns []net.Conn
	for i := 0; i < 200; i++ {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			break
		}
		// Send only the headers, withhold the body.
		fmt.Fprintf(c, "POST / HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n", bodyLimit)
		conns = append(conns, c)
	}
	// Close all partial connections.
	for _, c := range conns {
		c.Close()
	}

	// Wait for read timeouts to expire (ReadTimeout = 1s), then verify server is still alive.
	time.Sleep(1500 * time.Millisecond)

	// The server must still respond to a normal request.
	if st, closed := sendRaw(t, addr, "GET / HTTP/1.1\r\nHost: h\r\n\r\n"); closed || st == "" {
		t.Fatalf("server unresponsive after partial-upload flood; closed=%v status=%q", closed, st)
	}
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

// TestWing_TrustProxy_ManyHeaders guards that X-Forwarded-For is honored even when
// the request carries more than the 8 generic extra-header slots — XFF is parsed
// into a dedicated field, so it can no longer be dropped by slot overflow.
func TestWing_TrustProxy_ManyHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, TrustProxy: true}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte(r.RemoteAddr()))
	}))
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var b strings.Builder
	b.WriteString("GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For: 1.2.3.4\r\n")
	for i := 0; i < 12; i++ { // 12 unknown headers — well past the 8-slot extra store
		fmt.Fprintf(&b, "X-Custom-%d: v%d\r\n", i, i)
	}
	b.WriteString("\r\n")
	if _, err := conn.Write([]byte(b.String())); err != nil {
		t.Fatal(err)
	}
	body := readHTTPBody(t, conn)
	if body != "1.2.3.4" {
		t.Fatalf("XFF must survive >8 extra headers, got %q", body)
	}
}

// TestWing_HeaderLineTooLarge431 guards that a single header line over HeaderLimit
// (but within the read buffer) is rejected with 431, not a silent close.
func TestWing_HeaderLineTooLarge431(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, HeaderLimit: 1024}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer stop()
	// One ~2KB header line exceeds HeaderLimit 1024 but the whole request fits
	// the 8KB read buffer, so it takes the "buffer has room" classifier path.
	big := strings.Repeat("a", 2048)
	raw := "GET / HTTP/1.1\r\nHost: h\r\nX-Big: " + big + "\r\n\r\n"
	st, _ := sendRaw(t, addr, raw)
	if !strings.Contains(st, "431") {
		t.Fatalf("oversized header line want 431, got %q", st)
	}
}

// TestWing_SplitHeadersNotWronglyRejected guards that a header block of small,
// individually-legal lines is served even when a partial read snapshot exceeds
// HeaderLimit before the terminating CRLF arrives. The verdict must not depend on
// TCP segmentation (regression: the classifier's total-bytes early-out 431'd it).
func TestWing_SplitHeadersNotWronglyRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, HeaderLimit: 1024}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var b strings.Builder
	b.WriteString("GET / HTTP/1.1\r\nHost: h\r\n")
	for i := 0; i < 60; i++ { // ~60 small lines, each well under HeaderLimit 1024
		fmt.Fprintf(&b, "X-H-%d: vvvvvvvvvv\r\n", i)
	}
	full := b.String() // > 1024 bytes total, no terminating CRLF yet
	// Send a >1024 partial first (snapshot exceeds HeaderLimit), pause, then finish.
	if _, err := conn.Write([]byte(full[:1100])); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := conn.Write([]byte(full[1100:] + "\r\n")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(line, "200") {
		t.Fatalf("split small-header block wrongly rejected: got %q", strings.TrimSpace(line))
	}
}

// TestWing_Takeover_HeaderLineTooLarge431 guards that the takeover keep-alive path
// answers 431 for an oversized header line on a pipelined request, matching the
// event-loop path (regression: takeover grouped it with malformed and closed silently).
func TestWing_Takeover_HeaderLineTooLarge431(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, HeaderLimit: 1024, DefaultPreset: DB}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Pipeline req1 (valid → enters takeover) + req2 (oversized header → 431 via takeover).
	big := strings.Repeat("a", 2048)
	pipelined := "GET / HTTP/1.1\r\nHost: h\r\n\r\n" +
		"GET / HTTP/1.1\r\nHost: h\r\nX-Big: " + big + "\r\n\r\n"
	if _, err := conn.Write([]byte(pipelined)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var resp bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, rerr := conn.Read(tmp)
		resp.Write(tmp[:n])
		if rerr != nil {
			break
		}
	}
	s := resp.String()
	if !strings.Contains(s, " 200 ") {
		t.Fatalf("expected req1 to be served 200, got: %q", s)
	}
	if !strings.Contains(s, " 431 ") {
		t.Fatalf("expected req2 oversized header to get 431 on takeover path, got: %q", s)
	}
}

// TestWing_TrustProxy_EmptyFirstHeaderWins guards net/http Get semantics: when a
// request sends an empty X-Forwarded-For before a non-empty one, the first
// (empty) header wins and RemoteAddr falls back to the socket peer — a later
// attacker-supplied value must not override it.
func TestWing_TrustProxy_EmptyFirstHeaderWins(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, TrustProxy: true}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte(r.RemoteAddr()))
	}))
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For: \r\nX-Forwarded-For: 1.2.3.4\r\n\r\n")
	body := readHTTPBody(t, conn)
	if body == "1.2.3.4" {
		t.Fatalf("empty first XFF must win; second value must not override, got %q", body)
	}
	if !strings.HasPrefix(body, "127.0.0.1:") && !strings.HasPrefix(body, "[::1]:") {
		t.Fatalf("expected loopback fallback, got %q", body)
	}
}

// TestWing_Takeover_LargeHeaderWithinLimit guards that the takeover path accepts
// a header block larger than the 8 KB default pool buffer but within HeaderLimit,
// matching the event loop (regression: takeover used a fixed 8 KB buffer and
// wrongly 431'd / truncated large legal headers when HeaderLimit > 8 KB).
func TestWing_Takeover_LargeHeaderWithinLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 16384, HeaderLimit: 16384, DefaultPreset: DB}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// req1 valid (enters takeover) + req2 with a ~12 KB header line (< HeaderLimit,
	// > 8 KB default pool buffer); req2 closes the conn so the read ends promptly.
	big := strings.Repeat("a", 12000)
	pipelined := "GET / HTTP/1.1\r\nHost: h\r\n\r\n" +
		"GET / HTTP/1.1\r\nHost: h\r\nConnection: close\r\nX-Big: " + big + "\r\n\r\n"
	if _, err := conn.Write([]byte(pipelined)); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var resp bytes.Buffer
	tmp := make([]byte, 4096)
	for {
		n, rerr := conn.Read(tmp)
		resp.Write(tmp[:n])
		if rerr != nil {
			break
		}
	}
	s := resp.String()
	if strings.Contains(s, " 431 ") {
		t.Fatalf("legal large header on takeover wrongly 431'd: %q", s)
	}
	if strings.Count(s, " 200 ") < 2 {
		t.Fatalf("expected both requests served 200, got: %q", s)
	}
}

// TestWingRequest_HeaderXFFCaseInsensitive guards that the dedicated XFF/X-Real-IP
// fields are reachable via Header()/RawHeader() regardless of key casing, matching
// the case-insensitive lookup they had as generic extra headers.
func TestWingRequest_HeaderXFFCaseInsensitive(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nHost: h\r\nX-Forwarded-For: 9.9.9.9\r\nX-Real-IP: 8.8.8.8\r\n\r\n")
	r, _, ok := parseHTTPRequest(raw, parserLimits{maxHeaderCount: 100, maxHeaderSize: 8192})
	if !ok {
		t.Fatal("parse failed")
	}
	defer releaseRequest(r)
	for _, k := range []string{"X-Forwarded-For", "x-forwarded-for", "X-FORWARDED-FOR"} {
		if got := r.Header(k); got != "9.9.9.9" {
			t.Fatalf("Header(%q) = %q, want 9.9.9.9", k, got)
		}
	}
	for _, k := range []string{"X-Real-IP", "x-real-ip", "X-REAL-IP"} {
		if got := r.Header(k); got != "8.8.8.8" {
			t.Fatalf("Header(%q) = %q, want 8.8.8.8", k, got)
		}
	}
}

// TestWing_HeaderLineLengthBoundary pins the exact off-by-one contract of the
// classifyIncomplete per-line check: len(hline) == HeaderLimit is accepted;
// len(hline) == HeaderLimit+1 is rejected with 431.
// hline is the header line after \r stripping (e.g. "X-Test: <value>").
func TestWing_HeaderLineLengthBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const headerLimit = 512
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, HeaderLimit: headerLimit}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.WriteHeader(200)
	}))
	defer stop()

	prefix := "X-Test: " // 8 bytes; hline = prefix + value
	baseLen := len(prefix)

	for _, tc := range []struct {
		name     string
		valLen   int
		wantCode string
	}{
		{"at_limit", headerLimit - baseLen, "200"},       // len(hline)==headerLimit → OK
		{"over_limit", headerLimit - baseLen + 1, "431"}, // len(hline)==headerLimit+1 → 431
	} {
		t.Run(tc.name, func(t *testing.T) {
			hdr := prefix + strings.Repeat("a", tc.valLen)
			raw := "GET / HTTP/1.1\r\nHost: h\r\n" + hdr + "\r\n\r\n"
			st, _ := sendRaw(t, addr, raw)
			if !strings.Contains(st, " "+tc.wantCode+" ") {
				t.Fatalf("header line len=%d want %s, got %q", len(hdr), tc.wantCode, st)
			}
		})
	}
}

// TestWing_BodyLimitInBuffer guards the case where a complete request body fits
// entirely inside the read buffer but exceeds BodyLimit. The over-buffer slow
// path alone never sees these, so the parser must reject them so the classifier
// emits a 413 (regression: such bodies previously reached the handler).
func TestWing_BodyLimitInBuffer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	})
	// BodyLimit 512, default 8KB read buffer: a 2KB body is fully in-buffer.
	addr, stop := startBodyLimitWingServer(t, 512, handler)
	defer stop()

	if st, _ := sendRaw(t, addr, postRaw("/u", 512)); !strings.Contains(st, "200") {
		t.Fatalf("at-limit in-buffer body: want 200, got %q", st)
	}
	if st, _ := sendRaw(t, addr, postRaw("/u", 513)); !strings.Contains(st, "413") {
		t.Fatalf("over-limit in-buffer body: want 413, got %q", st)
	}
	if st, _ := sendRaw(t, addr, postRaw("/u", 2048)); !strings.Contains(st, "413") {
		t.Fatalf("over-limit in-buffer body (2KB): want 413, got %q", st)
	}
}

// TestWing_BudgetReclaimedAfterClose verifies the per-worker in-flight body
// budget is returned when a connection closes mid-accumulation. Two stalled
// uploads exhaust the budget; after they close, a fresh legal upload that needs
// the budget must still be accepted (regression: closeConn leaked the reservation).
func TestWing_BudgetReclaimedAfterClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const bodyLimit = 16 * 1024
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          8192,
		BodyLimit:            bodyLimit,
		MaxInflightBodyBytes: 2 * bodyLimit, // exactly two concurrent uploads
		ReadTimeout:          time.Second,   // backup reclaim path
	}
	addr, stop := startWingServerWithConfig(t, cfg, transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("%d", len(b))))
	}))
	defer stop()

	// Open two connections that promise a full body (exceeding the 8KB buffer so
	// each reserves budget) but never send it — exhausting MaxInflightBodyBytes.
	var stalled []net.Conn
	for i := 0; i < 2; i++ {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			t.Fatalf("dial stalled %d: %v", i, err)
		}
		fmt.Fprintf(c, "POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n", bodyLimit)
		stalled = append(stalled, c)
	}
	// Give the server time to register both reservations.
	time.Sleep(200 * time.Millisecond)

	// Close them — the server's pending recv hits EOF and closeConn must reclaim.
	for _, c := range stalled {
		c.Close()
	}
	time.Sleep(500 * time.Millisecond)

	// A fresh legal upload that needs the budget (body exceeds the buffer) must
	// be accepted. If the reservation leaked, this would be rejected with 503.
	if st, _ := sendRaw(t, addr, postRaw("/u", bodyLimit)); !strings.Contains(st, "200") {
		t.Fatalf("legal upload after budget reclaim: want 200, got %q (budget leaked?)", st)
	}
}
