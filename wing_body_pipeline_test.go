//go:build linux || darwin

package kruda

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// echoLenHandler replies "METHOD PATH len=N" so pipelined responses can be
// matched to their requests and the accumulated body length verified.
func echoLenHandler() transport.Handler {
	return transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		b, _ := r.Body()
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("%s %s len=%d", r.Method(), r.Path(), len(b))))
	})
}

// TestWing_EventLoop_PipelinedAfterAccumulatedBody verifies that a request
// pipelined immediately after a body that required multi-read accumulation is
// not dropped. The body (16KB) exceeds the 8KB read buffer, forcing the
// event-loop accumulation path; the trailing GET arrives in the same byte
// stream and must still be served.
func TestWing_EventLoop_PipelinedAfterAccumulatedBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{
		Workers:     1,
		ReadBufSize: 8192,
		BodyLimit:   64 * 1024, // body well within limit, but > read buffer
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	body := strings.Repeat("x", 16*1024)
	pipeline := "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body +
		"GET /done HTTP/1.1\r\nHost: h\r\n\r\n"
	if _, err := conn.Write([]byte(pipeline)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	_, body1, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 1 (POST): %v", err)
	}
	if body1 != "POST /upload len=16384" {
		t.Fatalf("response 1 body = %q, want POST /upload len=16384", body1)
	}
	_, body2, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 2 (pipelined GET dropped after accumulated body): %v", err)
	}
	if body2 != "GET /done len=0" {
		t.Fatalf("response 2 body = %q, want GET /done len=0", body2)
	}
}

// TestWing_Takeover_PipelinedAfterAccumulatedBody verifies the takeover path
// does not drop a request pipelined after an accumulated body. A first GET on a
// DB-preset (Takeover dispatch) route hands the connection to takeoverLoop; the
// following POST (16KB body, > read buffer) accumulates in the takeover loop and
// the trailing GET must still be served.
func TestWing_Takeover_PipelinedAfterAccumulatedBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{
		Workers:       1,
		ReadBufSize:   8192,
		BodyLimit:     64 * 1024,
		DefaultPreset: DB, // Takeover dispatch
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	body := strings.Repeat("y", 16*1024)
	pipeline := "GET /a HTTP/1.1\r\nHost: h\r\n\r\n" +
		"POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body +
		"GET /done HTTP/1.1\r\nHost: h\r\n\r\n"
	if _, err := conn.Write([]byte(pipeline)); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader := bufio.NewReader(conn)
	wants := []string{"GET /a len=0", "POST /upload len=16384", "GET /done len=0"}
	for i, want := range wants {
		_, b, err := readHTTPResponse(reader)
		if err != nil {
			t.Fatalf("response %d (%s): %v", i, want, err)
		}
		if b != want {
			t.Fatalf("response %d body = %q, want %q", i, b, want)
		}
	}
}

// takeoverStallConn opens a connection, drives the first GET through to the
// Takeover loop, reads its 200, then sends only the headers of a body POST so
// the takeover goroutine charges the in-flight body budget and parks waiting for
// a body that never arrives. Returns the live connection (caller closes it).
func takeoverStallConn(t *testing.T, addr string, contentLen int) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial stall: %v", err)
	}
	c.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := c.Write([]byte("GET /a HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write stall GET: %v", err)
	}
	r := bufio.NewReader(c)
	if st, _, err := readHTTPResponse(r); err != nil || !strings.Contains(st, "200") {
		t.Fatalf("stall GET response = %q err=%v, want 200", st, err)
	}
	// Headers only — withhold the body so the takeover loop stays parked with the
	// budget charged.
	fmt.Fprintf(c, "POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n", contentLen)
	return c
}

// TestWing_Takeover_BodyBudget503 verifies that takeover body accumulation
// charges the shared per-worker in-flight budget (MaxInflightBodyBytes), so a
// flood of concurrent takeover uploads cannot allocate unbounded memory: once
// the budget is exhausted, a further upload is rejected with 503.
func TestWing_Takeover_BodyBudget503(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const bodyLimit = 16 * 1024
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          8192,
		BodyLimit:            bodyLimit,
		MaxInflightBodyBytes: 2 * bodyLimit, // room for exactly two concurrent uploads
		DefaultPreset:        DB,            // Takeover dispatch
		ReadTimeout:          2 * time.Second,
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	// Two stalled takeover uploads charge the whole budget (2 * 16KB).
	s1 := takeoverStallConn(t, addr, bodyLimit)
	defer s1.Close()
	s2 := takeoverStallConn(t, addr, bodyLimit)
	defer s2.Close()
	// Let both takeover goroutines reach parseNeedBody and charge the budget.
	time.Sleep(300 * time.Millisecond)

	// A third upload exceeds the budget and must be rejected with 503.
	c := net.Conn(nil)
	{
		var err error
		c, err = net.DialTimeout("tcp", addr, time.Second)
		if err != nil {
			t.Fatalf("dial third: %v", err)
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(3 * time.Second))
	}
	if _, err := c.Write([]byte("GET /a HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write third GET: %v", err)
	}
	r := bufio.NewReader(c)
	if st, _, err := readHTTPResponse(r); err != nil || !strings.Contains(st, "200") {
		t.Fatalf("third GET response = %q err=%v, want 200", st, err)
	}
	fmt.Fprintf(c, "POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n%s", bodyLimit, strings.Repeat("z", bodyLimit))
	st, _, err := readHTTPResponse(r)
	if err != nil {
		t.Fatalf("third POST: expected 503, got error %v", err)
	}
	if !strings.Contains(st, "503") {
		t.Fatalf("third POST status = %q, want 503 (takeover budget not enforced)", st)
	}
}
