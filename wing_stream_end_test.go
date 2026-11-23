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
		{"empty Stream with padded zero length", "GET", 200, func(c *Ctx) error {
			c.SetHeader("Content-Length", "0 ")
			return c.Stream(strings.NewReader(""))
		}},
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
			expectNoSuccess(t, br)
		})
	}
}

// zeroWriteThenFail commits nothing but an empty write before it fails, as an
// io.WriterTo may.
type zeroWriteThenFail struct{}

func (zeroWriteThenFail) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func (zeroWriteThenFail) WriteTo(w io.Writer) (int64, error) {
	_, _ = w.Write(nil)
	return 0, io.ErrUnexpectedEOF
}

// expectNoSuccess reads what a failed response left: EOF before any byte, or a
// whole 5xx.
func expectNoSuccess(t *testing.T, br *bufio.Reader) {
	t.Helper()
	if _, err := br.Peek(1); err == io.EOF {
		return
	}
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("incomplete response: %v", err)
	}
	if body, err := io.ReadAll(res.Body); err != nil || res.StatusCode < 500 {
		t.Fatalf("failed copy answered %d with %q, %v", res.StatusCode, body, err)
	}
}

// TestWingStreamUnkeptPromiseIsNotSent ends Stream handlers without the body
// they declared. A preamble would promise bytes that never come.
func TestWingStreamUnkeptPromiseIsNotSent(t *testing.T) {
	for name, handler := range map[string]HandlerFunc{
		"declared length": func(c *Ctx) error {
			c.SetHeader("Content-Length", "5")
			return c.Stream(strings.NewReader(""))
		},
		"empty write before failure": func(c *Ctx) error { return c.Stream(zeroWriteThenFail{}) },
	} {
		t.Run(name, func(t *testing.T) {
			app := New(Wing())
			app.Get("/x", handler, Stream)
			addr, stop := startWingApp(t, app)
			defer stop()
			conn, br := framingDial(t, addr)
			if _, err := io.WriteString(conn, framingGet("/x")); err != nil {
				t.Fatal(err)
			}
			expectNoSuccess(t, br)
		})
	}
}

// TestWingStreamHeadKeepsDeclaredLength answers HEAD for a Stream route that
// declares its length. HEAD sends that length and no body.
func TestWingStreamHeadKeepsDeclaredLength(t *testing.T) {
	app := New(Wing())
	app.Head("/x", func(c *Ctx) error {
		c.SetHeader("Content-Length", "5")
		return c.Stream(strings.NewReader(""))
	}, Stream)
	addr, stop := startWingApp(t, app)
	defer stop()
	conn, br := framingDial(t, addr)
	if _, err := io.WriteString(conn, "HEAD /x HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	res, err := http.ReadResponse(br, &http.Request{Method: "HEAD"})
	if err != nil || res.StatusCode != 200 || res.ContentLength != 5 {
		t.Fatalf("HEAD: %v %+v", err, res)
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
	sw.streamCopyStart()
	sw.streamCopyResult(nil)
	sw.finish()
	sw.Flush()
	_, _ = sw.Write([]byte("x"))
	if conn.writes != 1 {
		t.Fatalf("preamble written %d times", conn.writes)
	}
}
