// Cross-platform (no build tag): references only streamConn and wing_http.go
// types that compile everywhere via wing_types_shared.go and wing_http.go.
// wing_http.go itself carries the linux||darwin tag so the types it defines
// (wingHeaders, statusLines, appendStatusAndHeaders) are only available there;
// this file carries the same tag to keep the build consistent.

//go:build linux || darwin

package kruda

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// streamConn is the minimal I/O surface the stream writer needs from a connection.
// *os.File satisfies it; tests use fakeStreamConn.
type streamConn interface {
	Write(p []byte) (int, error)
	SetWriteDeadline(t time.Time) error
}

// wingStreamWriter implements transport.ResponseWriter and http.Flusher for
// streaming responses (SSE, chunked). It writes the HTTP preamble (status line
// + headers) on the first Flush or Write (whichever comes first), then streams
// subsequent writes straight to the connection without buffering. SSE flushes
// on connect so the client receives the headers before the first event.
type wingStreamWriter struct {
	conn         streamConn
	headers      wingHeaders
	status       int
	headersSent  bool
	head         bool // HEAD: send the headers, never the body
	broken       bool // part of the preamble may be on the wire
	copying      bool // c.Stream is copying the body, or panicked doing so
	copyFailed   bool // the c.Stream copy returned an error
	writeTimeout time.Duration

	// StreamKeepAlive: the connection is reused after a response with one
	// valid Content-Length whose c.Stream copy wrote exactly that.
	tryKeepAlive bool
	framed       bool   // the preamble promised a reusable length
	copied       bool   // the copy succeeded
	failed       bool   // a framed write failed or overran the length
	remaining    int64  // framed bytes still owed
	onUnframed   func() // called once the preamble commits to closing
}

// Compile-time interface assertions. The http.Flusher assertion matters:
// c.SSE()/c.Stream() type-assert the writer to http.Flusher, so dropping
// Flush() must fail the build, not silently degrade at runtime.
var (
	_ transport.ResponseWriter = (*wingStreamWriter)(nil)
	_ http.Flusher             = (*wingStreamWriter)(nil)
)

func newWingStreamWriter(conn streamConn, writeTimeout time.Duration) *wingStreamWriter {
	return &wingStreamWriter{conn: conn, status: 200, writeTimeout: writeTimeout}
}

// WriteHeader records the status code. After the preamble has been written it
// is a no-op, matching net/http's contract (the status line is already on the wire).
func (w *wingStreamWriter) WriteHeader(code int) {
	if w.headersSent {
		return
	}
	w.status = code
}

func (w *wingStreamWriter) Header() transport.HeaderMap { return &w.headers }

// Flush emits the response preamble (status line + headers) if it has not been
// sent yet, then returns. Body writes already hit the socket directly, so Flush
// has no buffered data to push — but SSE flushes on connect (before the first
// event) to deliver the text/event-stream headers promptly, which is the one
// case where Flush must actually write the preamble.
func (w *wingStreamWriter) Flush() { _ = w.flushHeaders() }

// flushHeaders writes the status line + headers + the blank line that
// terminates the header section, exactly once. Subsequent calls are no-ops.
func (w *wingStreamWriter) flushHeaders() error {
	if w.headersSent {
		return nil
	}
	if w.broken {
		return errStreamPreamble // a retry would follow the part already sent
	}
	if w.tryKeepAlive && w.failed {
		return io.ErrClosedPipe
	}
	// Reuse needs one unambiguous response boundary and a c.Stream copy whose
	// result Wing sees. The preamble announces the decision. Otherwise the
	// stream path closes the connection, and the length is the handler's to state.
	if w.tryKeepAlive && w.copying {
		w.remaining, w.framed = streamContentLength(w.status, &w.headers)
	}
	if w.framed {
		w.headers.Set("Connection", "keep-alive") // HTTP/1.0 needs the token
	} else {
		w.headers.Set("Connection", "close")
	}
	// Raw fields are always checked here: this runs once per stream.
	hdr, _ := appendStatusAndHeaders(nil, w.status, &w.headers, true, true)
	hdr = append(hdr, "\r\n"...) // blank line terminates the header section
	if w.writeTimeout > 0 {
		if err := w.conn.SetWriteDeadline(time.Now().Add(w.writeTimeout)); err != nil && w.tryKeepAlive {
			w.failed = true
			return err
		}
	}
	if n, err := w.conn.Write(hdr); err != nil || n != len(hdr) {
		w.broken, w.failed = true, true
		if err == nil {
			err = io.ErrShortWrite
		}
		return err
	}
	w.headersSent = true
	if !w.framed && w.onUnframed != nil {
		w.onUnframed()
	}
	return nil
}

// streamCopyStart and streamCopyResult bracket c.Stream's copy, so a copy
// that failed or panicked is not completed as an empty success, and a reused
// connection knows its body was written whole.
func (w *wingStreamWriter) streamCopyStart() { w.copying = true }

func (w *wingStreamWriter) streamCopyResult(err error) {
	w.copying, w.copyFailed, w.copied = false, err != nil, err == nil
	// Only a framed body is spoiled by a failed copy. An unframed one is a
	// close-delimited stream the handler may still write to.
	w.failed = w.failed || w.framed && err != nil
}

var errStreamPreamble = errors.New("kruda: stream preamble was not fully written")

// finish sends the preamble of a response whose handler wrote no body. Not when
// that response declared a body: the client would wait for bytes that never
// come, so it gets the connection closed instead, as for a failed copy.
func (w *wingStreamWriter) finish() {
	if w.headersSent || w.broken || w.copying || w.copyFailed {
		return
	}
	if !w.head && !statusHasNoBody(w.status) && strings.TrimLeft(strings.Trim(w.headers.contentLength(), " \t"), "0") != "" {
		return
	}
	_ = w.flushHeaders()
}

func (w *wingStreamWriter) Write(p []byte) (int, error) {
	if len(p) == 0 && w.copying {
		return 0, nil // commits nothing: the copy may still fail
	}
	if err := w.flushHeaders(); err != nil {
		return 0, err
	}
	if w.head || statusHasNoBody(w.status) {
		return len(p), nil
	}
	if w.writeTimeout > 0 {
		if err := w.conn.SetWriteDeadline(time.Now().Add(w.writeTimeout)); err != nil && w.tryKeepAlive {
			w.failed = true
			return 0, err
		}
	}
	if !w.framed {
		return w.conn.Write(p)
	}
	if w.failed || int64(len(p)) > w.remaining {
		w.failed = true
		return 0, http.ErrContentLength
	}
	n, err := w.conn.Write(p)
	w.remaining -= int64(n)
	if n != len(p) && err == nil {
		err = io.ErrShortWrite
	}
	w.failed = w.failed || err != nil
	return n, err
}

// streamContentLength returns the length a reusable stream response states:
// GET 200 or 206, one valid positive Content-Length, and no field that changes
// framing or the connection.
func streamContentLength(status int, headers *wingHeaders) (int64, bool) {
	if status != http.StatusOK && status != http.StatusPartialContent {
		return 0, false
	}
	var length int64
	check := func(key, value string) bool {
		if !isValidTokenName([]byte(key)) || hasInvalidHeaderValueByte([]byte(value)) {
			return false
		}
		switch {
		case headerIs(key, "Connection"), headerIs(key, "Transfer-Encoding"),
			headerIs(key, "Trailer"), headerIs(key, "Upgrade"):
			return false
		case headerIs(key, "Content-Length"):
			if length != 0 || len(value) == 0 || value[0] < '0' || value[0] > '9' {
				return false
			}
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil || n <= 0 {
				return false
			}
			length = n
		}
		return true
	}
	for i := 0; i < headers.count; i++ {
		if !check(headers.keys[i], headers.vals[i]) {
			return 0, false
		}
	}
	for _, h := range headers.extra {
		if !check(h.key, h.val) {
			return 0, false
		}
	}
	return length, length > 0
}

// writeKeepAliveStream runs a StreamKeepAlive route and reports whether the
// connection may carry the next request. While the handler runs, a watcher
// borrows the read poller to notice a disconnect without consuming the next
// request; reuse starts only after it has returned. A response that will close
// switches to a watcher that drains input as soon as its preamble is sent, as
// the ordinary Stream path does, so input cannot pile up behind a long stream.
// When the connection is not reused, the caller's final drain closes it.
func (w *worker) writeKeepAliveStream(req *wingRequest, f *os.File) (reuse bool) {
	raw, err := f.SyscallConn()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithCancel(req.ctx)
	defer cancel()
	req.ctx = ctx
	stopPeek := make(chan struct{})
	peeked := make(chan bool, 1)
	go func() {
		var probe [1]byte
		var pending bool
		err := raw.Read(func(fd uintptr) bool {
			n, _, err := syscall.Recvfrom(int(fd), probe[:], syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
			pending = err == nil && n > 0
			return err != syscall.EAGAIN && err != syscall.EWOULDBLOCK && err != syscall.EINTR
		})
		var ok bool
		if pending {
			ok = watchQueuedStreamPeer(raw, ctx, stopPeek)
		} else if errors.Is(err, os.ErrDeadlineExceeded) {
			select {
			case <-stopPeek:
				ok = true
			default:
			}
		}
		if !ok {
			cancel() // the peer is gone
		}
		peeked <- ok
	}()
	peeking := true
	// endPeek stops the peeking watcher and reports whether the peer was
	// still there. Afterwards this goroutine is the only reader.
	endPeek := func() bool {
		peeking = false
		close(stopPeek)
		if f.SetReadDeadline(time.Now()) != nil {
			return false
		}
		ok := <-peeked
		return f.SetReadDeadline(time.Time{}) == nil && ok
	}
	var draining chan struct{}
	sw := newWingStreamWriter(f, time.Duration(w.writeTimeout))
	sw.tryKeepAlive = true
	sw.onUnframed = func() {
		if endPeek() {
			draining = make(chan struct{})
			go func() {
				watchStreamDisconnect(f, cancel)
				close(draining)
			}()
		}
	}
	defer func() {
		if recover() != nil {
			reuse = false
			if !sw.headersSent {
				sw.WriteHeader(http.StatusInternalServerError)
				_, _ = sw.Write([]byte("Internal Server Error\n"))
			}
		}
		if reuse {
			return
		}
		if peeking {
			endPeek()
		}
		if draining != nil {
			_ = f.SetReadDeadline(time.Now())
			<-draining
			_ = f.SetReadDeadline(time.Time{})
		}
	}()
	w.handler.ServeKruda(sw, req)
	sw.finish()
	if !sw.framed || !sw.copied || sw.failed || sw.remaining != 0 || ctx.Err() != nil {
		return false
	}
	if !endPeek() || ctx.Err() != nil {
		return false
	}
	return f.SetWriteDeadline(time.Time{}) == nil
}

// streamReader is the minimal read surface the disconnect watcher needs.
// *os.File (the takeover fd) satisfies it.
type streamReader interface {
	Read(p []byte) (int, error)
}

// watchStreamDisconnect cancels the stream context when the client closes its
// half of the connection. It cancels on a Read error or EOF (the client is gone
// or the socket failed); stray bytes from a misbehaving-but-connected client are
// ignored and watching continues, since a well-behaved SSE client is silent.
//
// It is run as a goroutine alongside the handler. The takeover fd is owned by a
// single *os.File, and a concurrent Read here while the handler Writes through
// the stream writer is safe (the runtime poller serializes per-direction).
func watchStreamDisconnect(r streamReader, cancel context.CancelFunc) {
	var b [256]byte
	for {
		if _, err := r.Read(b[:]); err != nil {
			cancel()
			return
		}
	}
}
