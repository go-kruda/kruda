//go:build linux || darwin

package kruda

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func framingGet(path string) string { return "GET " + path + " HTTP/1.1\r\nHost: x\r\n\r\n" }

func framingDial(t *testing.T, addr string) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return conn, bufio.NewReader(conn)
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("reader failed") }

// TestWingStreamCompletesBodylessResponse ends Stream-preset handlers without
// a body write. The preamble must still reach the client.
func TestWingStreamCompletesBodylessResponse(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		status       int
		handler      HandlerFunc
	}{
		{"empty Stream", "GET", 200, func(c *Ctx) error { return c.Stream(strings.NewReader("")) }},
		{"empty Stream with status", "GET", 206, func(c *Ctx) error { return c.Status(206).Stream(strings.NewReader("")) }},
		{"empty Stream HEAD", "HEAD", 200, func(c *Ctx) error { return c.Stream(strings.NewReader("")) }},
		{"NoContent", "GET", 204, func(c *Ctx) error { return c.NoContent() }},
		{"no response written", "GET", 200, func(c *Ctx) error { return nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/x", tc.handler, Stream)
			app.Head("/x", tc.handler, Stream)
			addr, stop := startWingApp(t, app)
			defer stop()
			conn, br := framingDial(t, addr)
			if _, err := io.WriteString(conn, tc.method+" /x HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
				t.Fatal(err)
			}
			res, err := http.ReadResponse(br, &http.Request{Method: tc.method})
			if err != nil {
				t.Fatalf("no response: %v", err)
			}
			body, err := io.ReadAll(res.Body)
			if err != nil || len(body) != 0 || res.StatusCode != tc.status || !res.Close {
				t.Fatalf("status %d close %v body %q err %v", res.StatusCode, res.Close, body, err)
			}
		})
	}
}

// TestWingStreamFailedCopyIsNotAnEmptySuccess fails c.Stream before its first
// byte. Completing that as an empty 200 would let a cache store it.
func TestWingStreamFailedCopyIsNotAnEmptySuccess(t *testing.T) {
	for name, reader := range map[string]io.Reader{
		"error": failingReader{io.ErrUnexpectedEOF},
		"panic": panicReader{},
	} {
		t.Run(name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/x", func(c *Ctx) error { return c.Stream(reader) }, Stream)
			addr, stop := startWingApp(t, app)
			defer stop()
			conn, br := framingDial(t, addr)
			if _, err := io.WriteString(conn, framingGet("/x")); err != nil {
				t.Fatal(err)
			}
			if res, err := http.ReadResponse(br, nil); err == nil && res.StatusCode < 500 {
				t.Fatalf("failed copy answered %d", res.StatusCode)
			}
		})
	}
}

type failFirstWriteConn struct{ writes int }

func (c *failFirstWriteConn) Write([]byte) (int, error) {
	c.writes++
	return 1, io.ErrShortWrite
}

func (c *failFirstWriteConn) SetWriteDeadline(time.Time) error { return nil }

// TestWingStreamDoesNotResendFailedPreamble fails the preamble part-way.
// Ending the stream must not append a second one to the partial first.
func TestWingStreamDoesNotResendFailedPreamble(t *testing.T) {
	conn := &failFirstWriteConn{}
	sw := newWingStreamWriter(conn, 0)
	sw.Flush()
	// An empty c.Stream afterwards succeeds without writing.
	sw.setCopyUnfinished(true)
	sw.setCopyUnfinished(false)
	sw.finish()
	if conn.writes != 1 {
		t.Fatalf("preamble written %d times", conn.writes)
	}
}
