package bench

import (
	"net/http"
	"net/url"

	"github.com/go-kruda/kruda/transport"
)

// ---------------------------------------------------------------------------
// Transport adapters for benchmarking
// ---------------------------------------------------------------------------
// These wrap standard net/http types to implement transport.Request and
// transport.ResponseWriter, allowing us to call App.ServeKruda directly
// without network I/O overhead.

// testResponseWriter wraps http.ResponseWriter to implement transport.ResponseWriter.
type testResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
	written    bool
}

func newTestResponseWriter(w http.ResponseWriter) *testResponseWriter {
	return &testResponseWriter{w: w, statusCode: 200}
}

func (t *testResponseWriter) WriteHeader(code int) {
	if t.written {
		return
	}
	t.statusCode = code
	t.w.WriteHeader(code)
	t.written = true
}

func (t *testResponseWriter) Header() transport.HeaderMap {
	return &testHeaderMap{h: t.w.Header()}
}

func (t *testResponseWriter) Write(data []byte) (int, error) {
	if !t.written {
		t.w.WriteHeader(t.statusCode)
		t.written = true
	}
	return t.w.Write(data)
}

// testHeaderMap wraps http.Header to implement transport.HeaderMap.
type testHeaderMap struct {
	h http.Header
}

func (m *testHeaderMap) Set(key, value string) { m.h.Set(key, value) }
func (m *testHeaderMap) Get(key string) string { return m.h.Get(key) }
func (m *testHeaderMap) Del(key string)        { m.h.Del(key) }
func (m *testHeaderMap) Add(key, value string) { m.h.Add(key, value) }

// testRequest wraps *http.Request to implement transport.Request.
type testRequest struct {
	r         *http.Request
	body      []byte
	bodyRead  bool
	queryVals url.Values
	queryDone bool
}

func newTestRequest(r *http.Request) *testRequest {
	return &testRequest{r: r}
}

func (r *testRequest) Method() string { return r.r.Method }

func (r *testRequest) Path() string {
	p := r.r.URL.Path
	if p == "" {
		return "/"
	}
	return p
}

func (r *testRequest) Header(key string) string {
	return r.r.Header.Get(key)
}

func (r *testRequest) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, nil
	}
	r.bodyRead = true
	if r.r.Body == nil {
		return nil, nil
	}
	defer r.r.Body.Close()
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, err := r.r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	r.body = buf
	return buf, nil
}

func (r *testRequest) QueryParam(key string) string {
	if !r.queryDone {
		r.queryVals = r.r.URL.Query()
		r.queryDone = true
	}
	return r.queryVals.Get(key)
}

func (r *testRequest) RemoteAddr() string { return "127.0.0.1" }
func (r *testRequest) Cookie(name string) string {
	c, err := r.r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
func (r *testRequest) RawRequest() any { return r.r }
