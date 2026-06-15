//go:build linux || darwin

package kruda

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
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
	st, need := classifyIncomplete(raw, lim)
	if st != parseNeedBody || need != 500 {
		t.Fatalf("got (%v,%d) want (NeedBody,500)", st, need)
	}

	// content-length over bodyLimit -> BodyTooLarge
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\nContent-Length: 99999\r\n\r\n")
	if st, _ := classifyIncomplete(raw, lim); st != parseBodyTooLarge {
		t.Fatalf("got %v want BodyTooLarge", st)
	}

	// chunked body -> Chunked
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\nTransfer-Encoding: chunked\r\n\r\n")
	if st, _ := classifyIncomplete(raw, lim); st != parseChunked {
		t.Fatalf("got %v want Chunked", st)
	}

	// no end-of-headers yet, buffer not full -> NeedHeaderMore
	raw = []byte("POST /u HTTP/1.1\r\nHost: h\r\n")
	if st, _ := classifyIncomplete(raw, lim); st != parseNeedHeaderMore {
		t.Fatalf("got %v want NeedHeaderMore", st)
	}
}
