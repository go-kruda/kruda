//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWingInlineThenTakeoverPreservesResponseOrder(t *testing.T) {
	firstBody := strings.Repeat("first-response-", 4096)
	app := New(Wing())
	app.Get("/inline", func(c *Ctx) error { return c.Text(firstBody) }, Bolt)
	app.Get("/takeover", func(c *Ctx) error { return c.Text("takeover-response") }, DB)
	addr, stop := startWingApp(t, app)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	pipeline := "GET /inline HTTP/1.1\r\nHost: test\r\n\r\n" +
		"GET /takeover HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, pipeline); err != nil {
		t.Fatal(err)
	}

	reader := bufio.NewReader(conn)
	status, body, err := readHTTPResponse(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") || body != firstBody {
		t.Fatalf("first response = (%q, %d bytes), want 200 and %d-byte inline body", status, len(body), len(firstBody))
	}
	status, body, err = readHTTPResponse(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(status, "200") || body != "takeover-response" {
		t.Fatalf("second response = (%q, %q), want 200 takeover response", status, body)
	}
}

func TestWingInlinePartialWriteThenTakeoverPreservesResponseOrder(t *testing.T) {
	firstBody := bytes.Repeat([]byte("x"), 8<<20)
	app := New(Wing())
	app.Get("/inline", func(c *Ctx) error { return c.SendBytes(firstBody) }, Bolt)
	app.Get("/takeover", func(c *Ctx) error { return c.Text("takeover-response") }, DB)
	addr, stop := startWingApp(t, app)
	defer stop()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if err := tcpConn.SetReadBuffer(64 << 10); err != nil {
			t.Fatal(err)
		}
	}
	if err := conn.SetDeadline(time.Now().Add(20 * time.Second)); err != nil {
		t.Fatal(err)
	}
	pipeline := "GET /inline HTTP/1.1\r\nHost: test\r\n\r\n" +
		"GET /takeover HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
	if _, err := io.WriteString(conn, pipeline); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, firstBody) {
		t.Fatalf("first response body length = %d, want %d", len(got), len(firstBody))
	}
	assertWingPipelineResponse(t, reader, "takeover-response")
}
