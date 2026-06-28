package kruda

import (
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// nonFlushingHeaderMap is a stub transport.HeaderMap that does NOT implement http.Flusher.
type nonFlushingHeaderMap struct {
	headers map[string]string
}

func (h *nonFlushingHeaderMap) Set(key, value string) {
	if h.headers == nil {
		h.headers = make(map[string]string)
	}
	h.headers[key] = value
}

func (h *nonFlushingHeaderMap) Add(key, value string) {
	if h.headers == nil {
		h.headers = make(map[string]string)
	}
	h.headers[key] = value
}

func (h *nonFlushingHeaderMap) Get(key string) string {
	return h.headers[key]
}

func (h *nonFlushingHeaderMap) Del(key string) {
	delete(h.headers, key)
}

// nonFlushingWriter is a stub transport.ResponseWriter that does NOT implement http.Flusher.
// Used to simulate fasthttp or other transports that don't support streaming.
type nonFlushingWriter struct {
	statusCode int
	headers    transport.HeaderMap
}

func (w *nonFlushingWriter) Header() transport.HeaderMap {
	if w.headers == nil {
		w.headers = &nonFlushingHeaderMap{headers: make(map[string]string)}
	}
	return w.headers
}

func (w *nonFlushingWriter) Write(b []byte) (int, error) {
	return 0, nil
}

func (w *nonFlushingWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// TestSSE_NoFlushSupport verifies that c.SSE() returns an actionable error
// when the transport (like fasthttp) does not support the Flusher interface.
func TestSSE_NoFlushSupport(t *testing.T) {
	app := New()
	c := newCtx(app)

	// Replace the writer with a non-flushing stub
	c.writer = &nonFlushingWriter{}

	// Call SSE and expect an error
	err := c.SSE(func(stream *SSEStream) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error when flusher not supported")
	}

	// Cast to *KrudaError to check the message
	kErr, ok := err.(*KrudaError)
	if !ok {
		t.Fatalf("expected *KrudaError, got %T", err)
	}

	msg := kErr.Message
	// The message should be actionable and mention both remedies:
	// 1. kruda.Stream preset (for Wing on Linux)
	// 2. kruda.NetHTTP() (for fasthttp on macOS/dev)
	if !strings.Contains(msg, "kruda.Stream") {
		t.Errorf("error message missing 'kruda.Stream': %q", msg)
	}
	if !strings.Contains(msg, "NetHTTP") {
		t.Errorf("error message missing 'NetHTTP': %q", msg)
	}
	if !strings.Contains(msg, "fasthttp") {
		t.Errorf("error message missing 'fasthttp': %q", msg)
	}
}
