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

// exerciseExpectContinue sends request headers and body in separate writes. The
// second parsed response must be the final response, which also guards against
// Wing emitting the interim response more than once.
func exerciseExpectContinue(t *testing.T, conn net.Conn, reader *bufio.Reader, path, body string) {
	t.Helper()

	headers := fmt.Sprintf("POST %s HTTP/1.1\r\nHost: h\r\nExpect: 100-continue\r\nContent-Length: %d\r\n\r\n", path, len(body))
	if _, err := conn.Write([]byte(headers)); err != nil {
		t.Fatalf("write Expect headers: %v", err)
	}

	status, interimBody, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("read interim response: %v", err)
	}
	if status != "HTTP/1.1 100 Continue" || interimBody != "" {
		t.Fatalf("interim response = (%q, %q), want exactly one 100 Continue with no body", status, interimBody)
	}

	if _, err := conn.Write([]byte(body)); err != nil {
		t.Fatalf("write body after 100 Continue: %v", err)
	}

	status, responseBody, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("read final response: %v", err)
	}
	wantBody := fmt.Sprintf("POST %s len=%d", path, len(body))
	if status != "HTTP/1.1 200 OK" || responseBody != wantBody {
		t.Fatalf("final response = (%q, %q), want (HTTP/1.1 200 OK, %q)", status, responseBody, wantBody)
	}
}

func TestWing_EventLoop_Expect100Continue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	body := strings.Repeat("e", 16*1024)
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          8192,
		BodyLimit:            64 * 1024,
		MaxInflightBodyBytes: len(body),
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	exerciseExpectContinue(t, conn, bufio.NewReader(conn), "/event-loop", body)
}

func TestWing_Takeover_Expect100Continue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	body := strings.Repeat("t", 16*1024)
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          8192,
		BodyLimit:            64 * 1024,
		MaxInflightBodyBytes: len(body),
		DefaultPreset:        DB,
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, reader := takeoverEstablish(t, addr)
	defer conn.Close()

	exerciseExpectContinue(t, conn, reader, "/takeover", body)
}

// TestWing_EventLoop_Expect100_EagerBodyDoesNotLeakContinue verifies that an
// Expect request completed synchronously inside beginBodyAccum does not reuse
// its stale Expect decision after reentrant parsing starts the next request's
// non-Expect body accumulation. The 512-byte read buffer forces request 1 into
// parseNeedBody; the same client write queues its remaining body plus request 2
// headers and a partial body for the synchronous drain/reentrant parse.
func TestWing_EventLoop_Expect100_EagerBodyDoesNotLeakContinue(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const (
		bodyLen       = 1024
		secondBodyNow = 64
	)
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          512,
		BodyLimit:            4 * 1024,
		MaxInflightBodyBytes: 4 * 1024,
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	firstBody := strings.Repeat("a", bodyLen)
	secondBody := strings.Repeat("b", bodyLen)
	queued := fmt.Sprintf("POST /first HTTP/1.1\r\nHost: h\r\nExpect: 100-continue\r\nContent-Length: %d\r\n\r\n", len(firstBody)) + firstBody +
		fmt.Sprintf("POST /second HTTP/1.1\r\nHost: h\r\nConnection: close\r\nContent-Length: %d\r\n\r\n", len(secondBody)) + secondBody[:secondBodyNow]
	if _, err := conn.Write([]byte(queued)); err != nil {
		t.Fatalf("write eager Expect body and pipelined partial body: %v", err)
	}

	reader := bufio.NewReader(conn)
	status, responseBody, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("read first interim response: %v", err)
	}
	if status != "HTTP/1.1 100 Continue" || responseBody != "" {
		t.Fatalf("first interim response = (%q, %q), want exactly one 100 Continue", status, responseBody)
	}

	status, responseBody, err = readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("read first final response: %v", err)
	}
	if status != "HTTP/1.1 200 OK" || responseBody != "POST /first len=1024" {
		t.Fatalf("first final response = (%q, %q), want 200 for /first after its single 100", status, responseBody)
	}

	if _, err := conn.Write([]byte(secondBody[secondBodyNow:])); err != nil {
		t.Fatalf("finish pipelined non-Expect body: %v", err)
	}
	status, responseBody, err = readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("read second final response: %v", err)
	}
	if status != "HTTP/1.1 200 OK" || responseBody != "POST /second len=1024" {
		t.Fatalf("second response = (%q, %q), want final 200 for /second with no leaked 100", status, responseBody)
	}
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

// TestWing_Takeover_BodyAccumulation covers the takeover accumulation path end to
// end on a single server (to limit takeover server churn): a request pipelined
// after an accumulated body is preserved (#4), and the in-flight body budget is
// charged and then refunded on completion (#3) so sequential uploads succeed.
//
// A first GET on a DB-preset (Takeover dispatch) route hands the connection to
// takeoverLoop; the following POST (16KB body, > read buffer) accumulates there.
// The budget has room for exactly one upload, so the second upload only succeeds
// if the first's charge was refunded.
func TestWing_Takeover_BodyAccumulation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	const bodyLimit = 64 * 1024
	cfg := WingConfig{
		Workers:              1,
		ReadBufSize:          8192,
		BodyLimit:            bodyLimit,
		MaxInflightBodyBytes: 16 * 1024, // room for exactly one 16KB upload
		DefaultPreset:        DB,        // Takeover dispatch
	}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, r := takeoverEstablish(t, addr)
	defer conn.Close()

	// POST (accumulated body) immediately followed by a pipelined GET: the GET
	// must not be dropped (#4 surplus preservation).
	body := strings.Repeat("y", 16*1024)
	pipeline := "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body +
		"GET /done HTTP/1.1\r\nHost: h\r\n\r\n"
	if _, err := conn.Write([]byte(pipeline)); err != nil {
		t.Fatalf("write pipeline: %v", err)
	}
	for _, want := range []string{"POST /upload len=16384", "GET /done len=0"} {
		_, b, err := readHTTPResponse(r)
		if err != nil {
			t.Fatalf("response %q: %v", want, err)
		}
		if b != want {
			t.Fatalf("response body = %q, want %q", b, want)
		}
	}

	// A second accumulated upload must succeed — proving the first upload's budget
	// charge was refunded on completion (#3). A leak would reject this with 503.
	fmt.Fprintf(conn, "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	st, b, err := readHTTPResponse(r)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if !strings.Contains(st, "200") || b != "POST /upload len=16384" {
		t.Fatalf("second upload st=%q body=%q, want 200 / len=16384 (budget leaked?)", st, b)
	}

	// An upload whose charge exceeds the budget (32KB > 16KB) is rejected with 503
	// (#3 enforcement). This is the last request on the connection — the reject
	// closes it. The 32KB body exceeds the read buffer so it takes the takeover
	// accumulation path where the budget is charged.
	big := strings.Repeat("z", 32*1024)
	fmt.Fprintf(conn, "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: %d\r\n\r\n%s", len(big), big)
	st, _, err = readHTTPResponse(r)
	if err != nil {
		t.Fatalf("over-budget upload: expected 503, got error %v", err)
	}
	if !strings.Contains(st, "503") {
		t.Fatalf("over-budget upload status = %q, want 503 (takeover budget not enforced)", st)
	}
}

// TestWing_EventLoop_PartialPipelinedAfterAccumulatedBody verifies that a
// request whose header is only partially present behind an accumulated body is
// preserved (not dropped) and is completed when the remaining bytes arrive on a
// later read. This is the edge-triggered-epoll case: the partial surplus is held
// in readBuf and the next socket edge finishes it.
func TestWing_EventLoop_PartialPipelinedAfterAccumulatedBody(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Wing integration test in short mode")
	}
	cfg := WingConfig{Workers: 1, ReadBufSize: 8192, BodyLimit: 64 * 1024}
	addr, stop := startWingServerWithConfig(t, cfg, echoLenHandler())
	defer stop()

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	body := strings.Repeat("x", 16*1024)
	// Body plus only a partial next request-line — the rest is withheld.
	first := "POST /upload HTTP/1.1\r\nHost: h\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body +
		"GET /done HT"
	if _, err := conn.Write([]byte(first)); err != nil {
		t.Fatalf("write first: %v", err)
	}

	reader := bufio.NewReader(conn)
	_, body1, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 1 (POST): %v", err)
	}
	if body1 != "POST /upload len=16384" {
		t.Fatalf("response 1 body = %q, want POST /upload len=16384", body1)
	}

	// Send the rest of the partial request on a later write (a new socket edge).
	if _, err := conn.Write([]byte("TP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write rest: %v", err)
	}
	_, body2, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatalf("response 2 (partial pipelined request lost): %v", err)
	}
	if body2 != "GET /done len=0" {
		t.Fatalf("response 2 body = %q, want GET /done len=0", body2)
	}
}

// takeoverEstablish dials a connection and drives a first GET through to the
// Takeover loop, returning the live connection and its response reader. A
// subsequent body POST on the same connection then exercises the takeover
// parseNeedBody accumulation path (the first request can never accumulate — it
// is dispatched complete or inline). Caller closes the connection.
func takeoverEstablish(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte("GET /a HTTP/1.1\r\nHost: h\r\n\r\n")); err != nil {
		t.Fatalf("write GET: %v", err)
	}
	r := bufio.NewReader(c)
	st, _, err := readHTTPResponse(r)
	if err != nil || !strings.Contains(st, "200") {
		t.Fatalf("GET /a response = %q err=%v, want 200", st, err)
	}
	return c, r
}
