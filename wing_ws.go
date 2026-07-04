//go:build linux || darwin

package kruda

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// wingAddr is a minimal net.Addr synthesized from Wing's per-connection remote
// address string (os.File has no LocalAddr/RemoteAddr). WebSocket handlers
// rarely need it; it exists to satisfy the net.Conn contract contrib/ws expects.
type wingAddr struct{ s string }

func (a wingAddr) Network() string { return "tcp" }
func (a wingAddr) String() string  { return a.s }

// wingHijackConn adapts a taken-over *os.File to net.Conn so contrib/ws (which
// is entirely net.Conn-based) works over Wing unchanged. Read/Write/deadlines
// forward to the *os.File (pollable on Linux/Darwin, so they park the goroutine,
// not the thread). Close is coordinated: it half-closes the socket to unblock
// the peer but does NOT close the fd — hijackTakeover owns the single physical
// close via doneMsg.file after the handler returns.
type wingHijackConn struct {
	f      *os.File
	fd     int32 // raw fd for syscall.Shutdown (avoids os.File.Fd()'s blocking side effect)
	remote string
	ctx    context.Context
	closed atomic.Bool
}

func newWingHijackConn(f *os.File, fd int32, remote string, ctx context.Context) *wingHijackConn {
	return &wingHijackConn{f: f, fd: fd, remote: remote, ctx: ctx}
}

func (c *wingHijackConn) Read(b []byte) (int, error)         { return c.f.Read(b) }
func (c *wingHijackConn) Write(b []byte) (int, error)        { return c.f.Write(b) }
func (c *wingHijackConn) LocalAddr() net.Addr                { return wingAddr{"wing"} }
func (c *wingHijackConn) RemoteAddr() net.Addr               { return wingAddr{c.remote} }
func (c *wingHijackConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *wingHijackConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *wingHijackConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// Close half-closes both directions to unblock the peer promptly, then returns.
// It does NOT close the fd (single-close invariant): hijackTakeover closes it
// once via doneMsg.file after the handler returns. Idempotent.
func (c *wingHijackConn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	// Use the raw fd, not c.f.Fd(): os.File.Fd() can move the fd off the runtime
	// poller (blocking mode), which would break concurrent parked I/O.
	_ = syscall.Shutdown(int(c.fd), syscall.SHUT_RDWR)
	return nil
}

// Done reports when the owning Wing connection is being torn down (server
// shutdown cancels the conn context). contrib/ws.Conn surfaces this so a handler
// blocked in app logic (not in a socket call) can select on it and exit.
func (c *wingHijackConn) Done() <-chan struct{} {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Done()
}

// wingHijackWriter is the c.ResponseWriter() for a Hijack-preset route. It
// buffers a normal handler response through a pooled wingResponse (so a rejected
// upgrade returns a correct 4xx via Wing's own serialization) and implements
// http.Hijacker so contrib/ws can take over the raw connection. It deliberately
// does NOT implement the responder fast-lane interfaces, so c.JSON/c.Text on the
// not-hijacked path fall through to plain buffering.
type wingHijackWriter struct {
	resp     *wingResponse
	f        *os.File
	fd       int32
	leftover []byte
	remote   string
	ctx      context.Context
	hijacked bool
}

func newWingHijackWriter(resp *wingResponse, f *os.File, fd int32, leftover []byte, remote string, ctx context.Context) *wingHijackWriter {
	return &wingHijackWriter{resp: resp, f: f, fd: fd, leftover: leftover, remote: remote, ctx: ctx}
}

// Compile-time interface assertions.
var (
	_ transport.ResponseWriter = (*wingHijackWriter)(nil)
	_ http.Hijacker            = (*wingHijackWriter)(nil)
)

func (hw *wingHijackWriter) WriteHeader(code int) {
	if hw.hijacked {
		return
	}
	hw.resp.WriteHeader(code)
}

func (hw *wingHijackWriter) Header() transport.HeaderMap { return hw.resp.Header() }

func (hw *wingHijackWriter) Write(p []byte) (int, error) {
	if hw.hijacked {
		return 0, http.ErrHijacked
	}
	return hw.resp.Write(p)
}

// Hijack detaches the connection: it returns the wingHijackConn adapter and a
// *bufio.ReadWriter whose reader is seeded with any bytes the client already
// sent past the handshake (leftover), so a pipelined first WS frame is not lost.
func (hw *wingHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hw.hijacked {
		return nil, nil, http.ErrHijacked
	}
	hw.hijacked = true
	nc := newWingHijackConn(hw.f, hw.fd, hw.remote, hw.ctx)
	var r io.Reader = nc
	if len(hw.leftover) > 0 {
		r = io.MultiReader(bytes.NewReader(hw.leftover), nc)
	}
	brw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(nc))
	return nc, brw, nil
}
