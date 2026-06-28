// Cross-platform (no build tag): references only streamConn and wing_http.go
// types that compile everywhere via wing_types_shared.go and wing_http.go.
// wing_http.go itself carries the linux||darwin tag so the types it defines
// (wingHeaders, statusLines, appendStatusAndHeaders) are only available there;
// this file carries the same tag to keep the build consistent.

//go:build linux || darwin

package kruda

import (
	"context"
	"net/http"
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
// + headers) on the first Write call, then streams subsequent writes straight
// to the connection without buffering.
type wingStreamWriter struct {
	conn         streamConn
	headers      wingHeaders
	status       int
	headersSent  bool
	writeTimeout time.Duration
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
func (w *wingStreamWriter) Flush()                      {} // writes already hit the socket directly

func (w *wingStreamWriter) Write(p []byte) (int, error) {
	if !w.headersSent {
		hdr, _ := appendStatusAndHeaders(nil, w.status, &w.headers)
		hdr = append(hdr, "\r\n"...) // blank line terminates the header section
		if w.writeTimeout > 0 {
			_ = w.conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))
		}
		if _, err := w.conn.Write(hdr); err != nil {
			return 0, err
		}
		w.headersSent = true
	}
	if w.writeTimeout > 0 {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	}
	return w.conn.Write(p)
}

// streamReader is the minimal read surface the disconnect watcher needs.
// *os.File (the takeover fd) satisfies it.
type streamReader interface {
	Read(p []byte) (int, error)
}

// watchStreamDisconnect cancels the stream context when the client closes its
// half of the connection. A well-behaved SSE/streaming client sends nothing
// after the request, so any Read result — EOF, error, or stray bytes — means
// the client is gone or misbehaving; either way the handler should stop.
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
