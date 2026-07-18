//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func TestWingHostValidationParserMatrix(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantParsed     bool
		wantClassified parseStatus
	}{
		{
			name:       "http11 valid host",
			raw:        "GET / HTTP/1.1\r\nHost: example.test\r\n\r\n",
			wantParsed: true,
		},
		{
			name:       "http11 explicit empty host",
			raw:        "GET / HTTP/1.1\r\nHost:\r\n\r\n",
			wantParsed: true,
		},
		{
			name:           "http11 missing host",
			raw:            "GET / HTTP/1.1\r\n\r\n",
			wantClassified: parseBadRequest,
		},
		{
			name:           "http11 invalid host",
			raw:            "GET / HTTP/1.1\r\nHost: bad host\r\n\r\n",
			wantClassified: parseBadRequest,
		},
		{
			name:           "http11 duplicate host",
			raw:            "GET / HTTP/1.1\r\nHost: first.test\r\nHost: second.test\r\n\r\n",
			wantClassified: parseBadRequest,
		},
		{
			name:       "http10 missing host",
			raw:        "GET / HTTP/1.0\r\n\r\n",
			wantParsed: true,
		},
		{
			name:           "http10 invalid present host",
			raw:            "GET / HTTP/1.0\r\nHost: bad/host\r\n\r\n",
			wantClassified: parseBadRequest,
		},
		{
			name:           "later version missing host",
			raw:            "GET / HTTP/2.0\r\n\r\n",
			wantClassified: parseBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _, parsed := parseHTTPRequest([]byte(tt.raw), noLimits)
			if req != nil {
				releaseRequest(req)
			}
			if parsed != tt.wantParsed {
				t.Fatalf("parseHTTPRequest() parsed = %v, want %v", parsed, tt.wantParsed)
			}
			if tt.wantParsed {
				return
			}
			status, _, _ := classifyIncomplete([]byte(tt.raw), noLimits)
			if status != tt.wantClassified {
				t.Fatalf("classifyIncomplete() = %v, want %v", status, tt.wantClassified)
			}
		})
	}
}

func TestWingHostValidationPrecedesExpectContinue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing host",
			raw:  "POST /upload HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 4\r\n\r\n",
		},
		{
			name: "invalid host",
			raw:  "POST /upload HTTP/1.1\r\nHost: bad host\r\nExpect: 100-continue\r\nContent-Length: 4\r\n\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, need, expectContinue := classifyIncomplete([]byte(tt.raw), noLimits)
			if status != parseBadRequest || need != 0 || expectContinue {
				t.Fatalf("classifyIncomplete() = (%v, %d, %v), want (%v, 0, false)", status, need, expectContinue, parseBadRequest)
			}
		})
	}
}

func TestWingHostViolationsReturn400BeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServer(t, handler)
	t.Cleanup(stop)

	tests := []struct {
		name string
		raw  string
	}{
		{name: "missing host", raw: "GET / HTTP/1.1\r\n\r\n"},
		{name: "invalid host", raw: "GET / HTTP/1.1\r\nHost: bad host\r\n\r\n"},
		{name: "duplicate host", raw: "GET / HTTP/1.1\r\nHost: a\r\nHost: b\r\n\r\n"},
		{
			name: "expect continue without host",
			raw:  "POST / HTTP/1.1\r\nExpect: 100-continue\r\nContent-Length: 4\r\n\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, headers := readWingHostResponse(t, addr, tt.raw)
			if !strings.Contains(status, "400 Bad Request") {
				t.Fatalf("status = %q, want 400 Bad Request", status)
			}
			if got := headers["connection"]; !strings.EqualFold(got, "close") {
				t.Fatalf("Connection = %q, want close", got)
			}
		})
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("handler calls = %d, want 0", got)
	}
}

func TestWingAcceptsEmptyHostAndHTTP10WithoutHost(t *testing.T) {
	var calls atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServer(t, handler)
	t.Cleanup(stop)

	for _, raw := range []string{
		"GET /empty HTTP/1.1\r\nHost:\r\nConnection: close\r\n\r\n",
		"GET /legacy HTTP/1.0\r\nConnection: close\r\n\r\n",
	} {
		status, _ := readWingHostResponse(t, addr, raw)
		if !strings.Contains(status, "200 OK") {
			t.Fatalf("status = %q, want 200 OK for %q", status, raw)
		}
	}

	if got := calls.Load(); got != 2 {
		t.Fatalf("handler calls = %d, want 2", got)
	}
}

func TestWingHostViolationInPipelineReturns400(t *testing.T) {
	var calls atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	addr, stop := startWingServer(t, handler)
	t.Cleanup(stop)

	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	pipeline := "GET /ok HTTP/1.1\r\nHost: example.test\r\n\r\n" +
		"GET /rejected HTTP/1.1\r\n\r\n"
	if _, err := conn.Write([]byte(pipeline)); err != nil {
		t.Fatalf("write pipeline: %v", err)
	}

	r := bufio.NewReader(conn)
	status, body, err := readHTTPResponse(r)
	if err != nil || !strings.Contains(status, "200 OK") || body != "ok" {
		t.Fatalf("first response = (%q, %q, %v), want 200 with ok", status, body, err)
	}
	status, _, err = readHTTPResponse(r)
	if err != nil || !strings.Contains(status, "400 Bad Request") {
		t.Fatalf("second response = (%q, %v), want 400 Bad Request", status, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("handler calls = %d, want 1", got)
	}
}

func TestWingHostViolationOnTakeoverPathReturns400(t *testing.T) {
	var calls atomic.Int32
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, _ transport.Request) {
		calls.Add(1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	})
	cfg := WingConfig{
		Workers:       1,
		RingSize:      256,
		ReadBufSize:   8192,
		DefaultPreset: DB,
	}
	addr, stop := startWingServerWithConfig(t, cfg, handler)
	t.Cleanup(stop)

	conn, r := takeoverEstablish(t, addr)
	defer conn.Close()
	if _, err := conn.Write([]byte("GET /rejected HTTP/1.1\r\n\r\n")); err != nil {
		t.Fatalf("write rejected request: %v", err)
	}
	status, _, err := readHTTPResponse(r)
	if err != nil || !strings.Contains(status, "400 Bad Request") {
		t.Fatalf("takeover response = (%q, %v), want 400 Bad Request", status, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("handler calls = %d, want 1", got)
	}
}

func TestWingHostRejectionPreservesPartialResponseOrdering(t *testing.T) {
	writer, reader := newSocketpairFiles(t)
	defer writer.Close()
	defer reader.Close()
	readerConn, err := net.FileConn(reader)
	if err != nil {
		t.Fatalf("wrap reader socket: %v", err)
	}
	defer readerConn.Close()
	if err := readerConn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	prefix := []byte("HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\n")
	remainder := []byte("body")
	terminal := wingStatusClose(400)
	if _, err := writer.Write(prefix); err != nil {
		t.Fatalf("write transmitted prefix: %v", err)
	}

	w, _ := newTestWorker(0)
	c := newTestConn(int32(writer.Fd()))
	c.keepAlive = true
	c.sendBuf = append(c.sendBuf, prefix...)
	c.sendBuf = append(c.sendBuf, remainder...)
	c.sendN = len(prefix) // force the state left by a partial non-blocking write
	w.conns[c.fd] = c
	w.writeAndClose(c, terminal)

	want := append(append(append([]byte(nil), prefix...), remainder...), terminal...)
	got := make([]byte, len(want))
	if _, err := io.ReadFull(readerConn, got); err != nil {
		t.Fatalf("read ordered responses: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("response stream duplicated or reordered\ngot:  %q\nwant: %q", got, want)
	}
}

func readWingHostResponse(t *testing.T, addr, raw string) (string, map[string]string) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}
	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}

	r := bufio.NewReader(conn)
	status, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	headers := make(map[string]string)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("read header: %v", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if ok {
			headers[strings.ToLower(strings.TrimSpace(name))] = strings.TrimSpace(value)
		}
	}
	return strings.TrimSpace(status), headers
}
