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
	"sync"
	"sync/atomic"
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
// not the thread).
//
// Close honors the standard http.Hijacker ownership contract: after Hijack the
// caller owns the connection and closes it when done — possibly from a goroutine
// that outlives the route handler. Close half-closes the socket (to unblock the
// peer / concurrent I/O) and signals the owning takeover goroutine via done; it
// does NOT close the fd itself. hijackTakeover holds the fd open until done fires
// (or the server shuts down) and performs the single physical close then — so a
// late Close from an app goroutine can never Shutdown a recycled fd.
type wingHijackConn struct {
	f      *os.File
	fd     int32 // raw fd for syscall.Shutdown (avoids os.File.Fd()'s blocking side effect)
	remote string
	ctx    context.Context
	gate   *takeoverGate // its mu orders the app's reads and read deadlines against Shutdown and the drain
	closed atomic.Bool
	reads  sync.WaitGroup // app reads in progress; the drain starts once they have returned
	done   chan struct{}  // closed once by Close() — tells hijackTakeover the app is done with the fd
}

func newWingHijackConn(f *os.File, fd int32, remote string, ctx context.Context, g *takeoverGate) *wingHijackConn {
	if g == nil {
		g = &takeoverGate{}
	}
	return &wingHijackConn{f: f, fd: fd, remote: remote, ctx: ctx, gate: g, done: make(chan struct{})}
}

// Reads end for good once the app closes the adapter or Shutdown begins. Both
// stop a blocked read with a deadline and keep the read side open, so Wing can
// drain behind what was written; a later deadline from the app's read loop
// must not revive it. Read deadlines change only under the gate, so a setter
// cannot slip in between the end of reads and the deadline that enforces it.
func (c *wingHijackConn) readsEnded() error {
	if c.closed.Load() {
		return net.ErrClosed
	}
	if c.gate.stopped() {
		return io.EOF
	}
	return nil
}

func (c *wingHijackConn) Read(b []byte) (int, error) {
	if err := c.admitRead(); err != nil {
		return 0, err
	}
	n, err := c.f.Read(b)
	c.reads.Done()
	if err != nil {
		if ended := c.readsEnded(); ended != nil {
			err = ended
		}
	}
	return n, err
}

func (c *wingHijackConn) admitRead() error {
	c.gate.mu.Lock()
	defer c.gate.mu.Unlock()
	if err := c.readsEnded(); err != nil {
		return err
	}
	c.reads.Add(1)
	return nil
}

func (c *wingHijackConn) Write(b []byte) (int, error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	return c.f.Write(b)
}

func (c *wingHijackConn) LocalAddr() net.Addr  { return wingAddr{"wing"} }
func (c *wingHijackConn) RemoteAddr() net.Addr { return wingAddr{c.remote} }

func (c *wingHijackConn) SetDeadline(t time.Time) error {
	c.gate.mu.Lock()
	defer c.gate.mu.Unlock()
	if c.readsEnded() != nil {
		return c.f.SetWriteDeadline(t)
	}
	return c.f.SetDeadline(t)
}

func (c *wingHijackConn) SetReadDeadline(t time.Time) error {
	c.gate.mu.Lock()
	defer c.gate.mu.Unlock()
	if c.readsEnded() != nil {
		return nil
	}
	return c.f.SetReadDeadline(t)
}

func (c *wingHijackConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// Close ends the write side and any blocked read, and signals the owning
// takeover goroutine that the fd may be reclaimed. The read side stays open:
// the owner drains input behind what was written before it closes, as a reset
// over unread input could discard a response the peer has not read yet. It does
// NOT close the fd itself (single-close invariant): hijackTakeover, which is
// still holding the fd open, performs the one physical close after done fires.
//
// Idempotent AND synchronizing: the first caller does the raw-fd Shutdown and
// closes done; any concurrent/later caller BLOCKS on done before returning. That
// guarantee is load-bearing — hijackTakeover reclaims (and the kernel may
// recycle) the fd only after a Close returns, so every caller must observe the
// raw-fd Shutdown as complete before returning, or a losing closer could run
// Shutdown on an already-recycled fd.
func (c *wingHijackConn) Close() error {
	c.gate.mu.Lock()
	if c.closed.Swap(true) {
		c.gate.mu.Unlock()
		<-c.done // a concurrent closer won; wait until its Shutdown has completed
		return nil
	}
	// Use the raw fd, not c.f.Fd(): os.File.Fd() can move the fd off the runtime
	// poller (blocking mode), which would break concurrent parked I/O. Safe here
	// because hijackTakeover has not closed the fd yet — it waits for done below.
	c.gate.shutWriteLocked(c.fd)
	_ = c.f.SetReadDeadline(time.Now())
	c.gate.mu.Unlock()
	close(c.done)
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
	resp         *wingResponse
	f            *os.File
	fd           int32
	leftover     []byte
	remote       string
	ctx          context.Context
	hijacked     bool
	hijackedConn *wingHijackConn // set by Hijack(); hijackTakeover waits on its done
	gate         *takeoverGate
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
	nc := newWingHijackConn(hw.f, hw.fd, hw.remote, hw.ctx, hw.gate)
	hw.hijackedConn = nc
	var r io.Reader = nc
	if len(hw.leftover) > 0 {
		r = io.MultiReader(bytes.NewReader(hw.leftover), nc)
	}
	brw := bufio.NewReadWriter(bufio.NewReader(r), bufio.NewWriter(nc))
	return nc, brw, nil
}

// hijackTakeover runs a Hijack-preset route over the takeover fd f. It gives the
// handler a wingHijackWriter (an http.Hijacker); the handler (e.g. contrib/ws
// Upgrade) either hijacks the raw connection and runs its own read/write loop, or
// returns a normal buffered response (a rejected upgrade) which we serialize
// here. Unlike streamTakeover it seeds leftover (a pipelined first frame) into
// the hijacked reader and runs NO read-watcher — the handler owns reads on f.
//
// http.Hijacker ownership: once hijacked, the connection belongs to the caller,
// which may keep using it from a goroutine that outlives the route handler. So
// after the handler returns we hold the fd open until the app closes the adapter
// (its done channel) OR the server shuts down (ctx cancel), and only then perform
// the single physical close via the shared takeover-done path (doneMsg.file).
// This is what makes a late Conn.Close() from an app goroutine safe — the fd is
// never reclaimed (and thus never recycled) while the app still references it.
func (w *worker) hijackTakeover(first *wingRequest, fd int32, f *os.File, g *takeoverGate, leftover []byte, afterResponse bool) {
	// Cancellable conn context so shutdown (Task 6) can fire Conn.Done().
	ctx, cancel := context.WithCancel(first.ctx)
	first.ctx = ctx

	remote := ""
	if first.remoteAddrRef != nil {
		remote = *first.remoteAddrRef
	}

	resp := acquireResponseFor(first)
	hw := newWingHijackWriter(resp, f, fd, leftover, remote, ctx)
	hw.gate = g

	func() {
		defer func() {
			if r := recover(); r != nil && !hw.hijacked {
				// Only meaningful if the connection was not hijacked; a panic
				// inside a hijacked WS loop cannot produce an HTTP response.
				resp.WriteHeader(500)
				resp.Write([]byte("Internal Server Error\n"))
			}
		}()
		w.handler.ServeKruda(hw, first)
	}()

	// Phase 3 state machine: not hijacked → serialize the buffered response;
	// hijacked → the handler already wrote everything to the raw connection.
	rejected := false
	if !hw.hijacked {
		// The fd is closed right after this write, so advertise Connection: close
		// rather than letting an HTTP/1.1 client treat the socket as reusable.
		resp.Header().Set("Connection", "close")
		data := resp.buildZeroCopy()
		if w.writeTimeout > 0 {
			_ = f.SetWriteDeadline(time.Now().Add(time.Duration(w.writeTimeout)))
		}
		_, err := f.Write(data)
		rejected = err == nil
	}
	releaseResponse(resp)
	releaseRequest(first) // handler returned; the adapter holds ctx/remote independently
	if !hw.hijacked && (rejected || afterResponse) {
		// After the release: the wait can take 500 ms. A rejection whose write
		// failed still drains behind an earlier response on this connection.
		lingerTakeoverConn(f, fd, g)
	}

	// Hijacked: the app owns the connection. Wait until it closes the adapter
	// (done) or the server shuts down (ctx) before reclaiming the fd, so an app
	// goroutine that outlives the handler never Shutdowns a recycled fd.
	if hw.hijacked && hw.hijackedConn != nil {
		select {
		case <-hw.hijackedConn.done:
			// App closed the connection: the adapter is already marked closed and
			// has done its raw-fd Shutdown while the fd was still valid.
		case <-ctx.Done():
			// Server shutdown won the race before the app closed. Retire the
			// adapter ourselves (idempotent atomic guard) BEFORE we reclaim the
			// fd below, so a late app Close() finds it already closed and is a
			// no-op — it can never Shutdown the (soon-to-be-recycled) raw fd. A
			// concurrent app Close races harmlessly: exactly one wins.
			_ = hw.hijackedConn.Close()
		}
		// The app is done with the socket: once its reads have returned, which
		// the expired deadline makes prompt, drain behind what was written on it.
		hw.hijackedConn.reads.Wait()
		lingerTakeoverConn(f, fd, hw.hijackedConn.gate)
	}

	cancel()
	// Close the fd exactly once via the shared takeover-done path (handleDone
	// removes bookkeeping then closes through the File). Do NOT close here.
	w.doneCh <- doneMsg{fd: fd, keepAlive: false, file: f}
	w.wake()
}
