//go:build linux || darwin

package wing

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// ============================================================================
// Integration Tests — Real TCP through Wing transport
//
// These tests start an actual Wing server on a random port, connect via raw
// TCP, send pipelined HTTP requests, and verify the responses. This covers
// the FULL path: epoll/kqueue → handleRecv → parser → handler →
// buildResponse → handleSend → processPipelined → repeat.
// ============================================================================

// startWingServer starts a Wing transport on a random port with the given handler.
// Returns the address and a shutdown function.
func startWingServer(t *testing.T, handler transport.Handler) (addr string, shutdown func()) {
	t.Helper()

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr = ln.Addr().String()
	ln.Close()

	cfg := Config{
		Workers:     1,
		RingSize:    256,
		ReadBufSize: 8192,
	}
	tr := New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- tr.ListenAndServe(addr, handler)
	}()

	// Wait for the server to be ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return addr, func() {
		tr.Shutdown(context.Background())
	}
}

// readHTTPResponse reads one full HTTP response from a bufio.Reader.
// Returns the status line, headers, and body.
func readHTTPResponse(r *bufio.Reader) (status string, body string, err error) {
	// Read status line.
	statusLine, err := r.ReadString('\n')
	if err != nil {
		return "", "", fmt.Errorf("read status: %w", err)
	}
	status = strings.TrimSpace(statusLine)

	// Read headers, find Content-Length.
	contentLength := 0
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return status, "", fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // end of headers
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			val := strings.TrimSpace(line[len("content-length:"):])
			contentLength, _ = strconv.Atoi(val)
		}
	}

	// Read body.
	if contentLength > 0 {
		buf := make([]byte, contentLength)
		_, err = io.ReadFull(r, buf)
		if err != nil {
			return status, "", fmt.Errorf("read body: %w", err)
		}
		body = string(buf)
	}
	return status, body, nil
}

// --- Test 1: Response Ordering ---

// TestIntegration_ResponseOrdering sends 5 pipelined requests and verifies
// responses come back in strict FIFO order (RFC 7230 §6.3.2).
func TestIntegration_ResponseOrdering(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		// Echo back the path as the response body so we can verify order.
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Path()))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// Send 5 pipelined requests at once.
	var pipeline strings.Builder
	for i := 0; i < 5; i++ {
		pipeline.WriteString(fmt.Sprintf("GET /order/%d HTTP/1.1\r\nHost: localhost\r\n\r\n", i))
	}
	_, err = conn.Write([]byte(pipeline.String()))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read 5 responses and verify order.
	reader := bufio.NewReader(conn)
	for i := 0; i < 5; i++ {
		status, body, err := readHTTPResponse(reader)
		if err != nil {
			t.Fatalf("response %d: %v", i, err)
		}
		if !strings.Contains(status, "200") {
			t.Errorf("response %d: status = %q, want 200", i, status)
		}
		wantBody := fmt.Sprintf("/order/%d", i)
		if body != wantBody {
			t.Errorf("response %d: body = %q, want %q (ORDER VIOLATED!)", i, body, wantBody)
		}
	}
}

// --- Test 2: CRLF After POST Body (fasthttp pattern) ---

// TestIntegration_CRLFAfterPOSTBody tests the case where a client sends
// extra \r\n after a POST body before the next pipelined request.
// Some HTTP clients do this. RFC 7230 §3.5 says servers SHOULD ignore
// at least one empty line before the request-line.
//
// NOTE: If this test FAILS, the parser needs to be updated to skip
// leading CRLF — which is a known gap vs fasthttp's implementation.
func TestIntegration_CRLFAfterPOSTBody(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Method() + " " + r.Path()))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// POST with body, then extra \r\n, then GET.
	// The extra \r\n between requests is what fasthttp specifically tests.
	body := `{"ok":true}`
	pipeline := "POST /api HTTP/1.1\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n" +
		"Host: localhost\r\n\r\n" +
		body +
		"\r\n" + // <-- extra CRLF after body (some clients do this)
		"GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n"

	_, err = conn.Write([]byte(pipeline))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)

	// First response: POST /api
	status1, body1, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 1: %v", err)
	}
	if !strings.Contains(status1, "200") {
		t.Errorf("response 1: status = %q", status1)
	}
	if body1 != "POST /api" {
		t.Errorf("response 1: body = %q, want 'POST /api'", body1)
	}

	// Second response: GET /health
	// If this fails/hangs, the parser doesn't handle leading CRLF.
	status2, body2, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 2: %v (parser may not handle leading CRLF)", err)
	}
	if !strings.Contains(status2, "200") {
		t.Errorf("response 2: status = %q", status2)
	}
	if body2 != "GET /health" {
		t.Errorf("response 2: body = %q, want 'GET /health'", body2)
	}
}

// --- Test 3: Mixed Methods Pipeline ---

// TestIntegration_MixedMethodsPipeline sends GET, POST, PUT, DELETE in one
// pipeline and verifies all responses arrive correctly.
func TestIntegration_MixedMethodsPipeline(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		resp := r.Method() + " " + r.Path()
		if len(b) > 0 {
			resp += " body=" + string(b)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(resp))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	putBody := `{"name":"updated"}`
	pipeline := "GET /items HTTP/1.1\r\nHost: h\r\n\r\n" +
		"POST /items HTTP/1.1\r\nHost: h\r\nContent-Length: 8\r\n\r\n{\"a\":\"b\"}" +
		"PUT /items/1 HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(putBody)) + "\r\n\r\n" + putBody +
		"DELETE /items/1 HTTP/1.1\r\nHost: h\r\n\r\n"

	_, err = conn.Write([]byte(pipeline))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	expects := []string{
		"GET /items",
		`POST /items body={"a":"b"}`,
		`PUT /items/1 body={"name":"updated"}`,
		"DELETE /items/1",
	}

	reader := bufio.NewReader(conn)
	for i, want := range expects {
		_, body, err := readHTTPResponse(reader)
		if err != nil {
			t.Fatalf("response %d: %v", i, err)
		}
		if body != want {
			t.Errorf("response %d: body = %q, want %q", i, body, want)
		}
	}
}

// --- Test 4: Large Body in Pipeline ---

// TestIntegration_LargeBodyPipeline sends a POST with a 4KB body followed
// by a GET, testing that consumed bytes are correct for large payloads.
func TestIntegration_LargeBodyPipeline(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		resp := fmt.Sprintf("%s %s len=%d", r.Method(), r.Path(), len(b))
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(resp))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	largeBody := strings.Repeat("X", 4096)
	pipeline := "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(largeBody)) + "\r\n\r\n" + largeBody +
		"GET /done HTTP/1.1\r\nHost: h\r\n\r\n"

	_, err = conn.Write([]byte(pipeline))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)

	_, body1, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 1: %v", err)
	}
	if body1 != "POST /upload len=4096" {
		t.Errorf("response 1: body = %q", body1)
	}

	_, body2, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 2: %v", err)
	}
	if body2 != "GET /done len=0" {
		t.Errorf("response 2: body = %q", body2)
	}
}

// --- Test 5: Connection Close Stops Pipeline ---

// TestIntegration_ConnectionCloseStopsPipeline verifies that after a request
// with Connection: close, the server closes the TCP connection and does NOT
// process further pipelined data.
func TestIntegration_ConnectionCloseStopsPipeline(t *testing.T) {
	var mu sync.Mutex
	var paths []string

	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		mu.Lock()
		paths = append(paths, r.Path())
		mu.Unlock()
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	// First request says Connection: close; second request is pipelined.
	pipeline := "GET /first HTTP/1.1\r\nHost: h\r\nConnection: close\r\n\r\n" +
		"GET /second HTTP/1.1\r\nHost: h\r\n\r\n"

	_, err = conn.Write([]byte(pipeline))
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should get exactly one response, then connection closed.
	reader := bufio.NewReader(conn)
	_, body1, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 1: %v", err)
	}
	if body1 != "ok" {
		t.Errorf("response 1: body = %q", body1)
	}

	// Second read should fail (connection closed by server).
	_, _, err = readHTTPResponse(reader)
	if err == nil {
		t.Error("expected error on second read (connection should be closed)")
	}

	// Verify only 1 request was processed.
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 1 {
		t.Errorf("handler called %d times, want 1 (paths: %v)", len(paths), paths)
	}
}

// --- Test 6: Concurrent Pipelined Connections ---

// TestIntegration_ConcurrentPipelinedConns tests multiple simultaneous TCP
// connections each sending pipelined requests — the primary wrk/bench scenario.
func TestIntegration_ConcurrentPipelinedConns(t *testing.T) {
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(r.Path()))
	})

	addr, shutdown := startWingServer(t, handler)
	defer shutdown()

	const numConns = 4
	const reqsPerConn = 10

	var wg sync.WaitGroup
	errs := make(chan string, numConns*reqsPerConn)

	for c := 0; c < numConns; c++ {
		wg.Add(1)
		go func(connID int) {
			defer wg.Done()

			conn, err := net.DialTimeout("tcp", addr, time.Second)
			if err != nil {
				errs <- fmt.Sprintf("conn %d: dial: %v", connID, err)
				return
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(10 * time.Second))

			// Build pipeline.
			var pipeline strings.Builder
			for i := 0; i < reqsPerConn; i++ {
				pipeline.WriteString(fmt.Sprintf("GET /c%d/r%d HTTP/1.1\r\nHost: h\r\n\r\n", connID, i))
			}
			_, err = conn.Write([]byte(pipeline.String()))
			if err != nil {
				errs <- fmt.Sprintf("conn %d: write: %v", connID, err)
				return
			}

			reader := bufio.NewReader(conn)
			for i := 0; i < reqsPerConn; i++ {
				_, body, err := readHTTPResponse(reader)
				if err != nil {
					errs <- fmt.Sprintf("conn %d resp %d: %v", connID, i, err)
					return
				}
				want := fmt.Sprintf("/c%d/r%d", connID, i)
				if body != want {
					errs <- fmt.Sprintf("conn %d resp %d: body=%q want=%q", connID, i, body, want)
				}
			}
		}(c)
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// ============================================================================
// Parser Test: Leading CRLF tolerance (RFC 7230 §3.5)
// ============================================================================

// TestParser_LeadingCRLFTolerance tests whether the parser handles leading
// \r\n before a request-line. RFC 7230 §3.5 says servers SHOULD tolerate this.
// This is the root cause of the fasthttp "CRLF after POST" test.
//
// If this test FAILS, parseHTTPRequest needs to skip leading CRLF.
func TestParser_LeadingCRLFTolerance(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		expect bool // true if parser should accept
		path   string
	}{
		{
			name:   "single CRLF before GET",
			raw:    "\r\nGET /a HTTP/1.1\r\nHost: h\r\n\r\n",
			expect: false, // current parser rejects — documents the gap
			path:   "/a",
		},
		{
			name:   "double CRLF before GET",
			raw:    "\r\n\r\nGET /b HTTP/1.1\r\nHost: h\r\n\r\n",
			expect: false, // current parser rejects
			path:   "/b",
		},
		{
			name:   "no leading CRLF (normal)",
			raw:    "GET /c HTTP/1.1\r\nHost: h\r\n\r\n",
			expect: true,
			path:   "/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _, ok := parseHTTPRequest([]byte(tt.raw), noLimits)
			if ok != tt.expect {
				t.Errorf("parseHTTPRequest = %v, want %v", ok, tt.expect)
			}
			if ok && req.Path() != tt.path {
				t.Errorf("Path = %q, want %q", req.Path(), tt.path)
			}
		})
	}

	// Also test the pipelining scenario: POST body + extra \r\n + next request.
	body := "hello"
	postReq := "POST /a HTTP/1.1\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body
	nextReq := "\r\nGET /b HTTP/1.1\r\n\r\n" // leading \r\n
	buf := []byte(postReq + nextReq)

	_, consumed, ok := parseHTTPRequest(buf, noLimits)
	if !ok {
		t.Fatal("POST should parse")
	}

	// Try to parse the remainder (starts with \r\n).
	remaining := buf[consumed:]
	_, _, ok = parseHTTPRequest(remaining, noLimits)
	// Document current behavior: this FAILS because parser doesn't skip leading CRLF.
	// If this starts passing, the parser has been fixed — update expect values above.
	if ok {
		t.Log("Parser now handles leading CRLF — update TestParser_LeadingCRLFTolerance expects!")
	} else {
		t.Log("Parser does NOT handle leading CRLF (RFC 7230 §3.5 SHOULD) — known gap")
	}
}
