package kruda

import (
	"context"
	"errors"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"

	"github.com/go-kruda/kruda/transport"
)

// ErrBodyTooLarge is returned when the request body exceeds the configured limit.
var ErrBodyTooLarge = errors.New("kruda: request body too large")

type fastNetHTTPRequest struct {
	r         *http.Request
	body      []byte
	bodyRead  bool
	queryVals url.Values
	queryDone bool
	maxBody   int // 0 = no limit (benchmark mode)
}

func (r *fastNetHTTPRequest) Method() string { return r.r.Method }

func (r *fastNetHTTPRequest) Path() string {
	path := r.r.URL.Path
	if path == "" {
		return "/"
	}
	return path
}

func (r *fastNetHTTPRequest) Header(key string) string {
	return r.r.Header.Get(key)
}

func (r *fastNetHTTPRequest) Body() ([]byte, error) {
	if r.bodyRead {
		return r.body, nil
	}
	r.bodyRead = true
	if r.r.Body == nil {
		return nil, nil
	}
	defer r.r.Body.Close()

	if r.r.ContentLength == 0 {
		return nil, nil
	}

	// O(1) reject for known Content-Length
	if r.maxBody > 0 && r.r.ContentLength > int64(r.maxBody) {
		return nil, ErrBodyTooLarge
	}

	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	total := 0
	for {
		n, err := r.r.Body.Read(tmp)
		if n > 0 {
			total += n
			// Streaming check for chunked transfers (ContentLength == -1)
			if r.maxBody > 0 && total > r.maxBody {
				return nil, ErrBodyTooLarge
			}
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	r.body = buf
	return buf, nil
}

func (r *fastNetHTTPRequest) QueryParam(key string) string {
	// Fast path: if no query string, return empty
	if r.r.URL.RawQuery == "" {
		return ""
	}
	if !r.queryDone {
		r.queryVals = r.r.URL.Query()
		r.queryDone = true
	}
	return r.queryVals.Get(key)
}

func (r *fastNetHTTPRequest) RemoteAddr() string {
	if host, _, err := net.SplitHostPort(r.r.RemoteAddr); err == nil {
		return host
	}
	return r.r.RemoteAddr
}

func (r *fastNetHTTPRequest) Cookie(name string) string {
	c, err := r.r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func (r *fastNetHTTPRequest) RawRequest() any { return r.r }

func (r *fastNetHTTPRequest) Context() context.Context { return r.r.Context() }

func (r *fastNetHTTPRequest) MultipartForm(maxBytes int64) (*multipart.Form, error) {
	if err := r.r.ParseMultipartForm(maxBytes); err != nil {
		return nil, err
	}
	return r.r.MultipartForm, nil
}

type fastNetHTTPResponseWriter struct {
	w          http.ResponseWriter
	statusCode int
	written    bool
	headerMap  *fastNetHTTPHeaderMap
	// Pre-allocated slices for common headers
	contentTypeSlice   []string
	contentLengthSlice []string
}

func (w *fastNetHTTPResponseWriter) WriteHeader(statusCode int) {
	if w.written {
		return
	}
	w.statusCode = statusCode
	w.w.WriteHeader(statusCode)
	w.written = true
}

func (w *fastNetHTTPResponseWriter) Header() transport.HeaderMap {
	if w.headerMap == nil {
		w.headerMap = &fastNetHTTPHeaderMap{h: w.w.Header()}
	}
	return w.headerMap
}

func (w *fastNetHTTPResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.w.WriteHeader(w.statusCode)
		w.written = true
	}
	return w.w.Write(data)
}

func (w *fastNetHTTPResponseWriter) DirectHeader() http.Header {
	return w.w.Header()
}

func (w *fastNetHTTPResponseWriter) SetContentType(contentType string) {
	if w.contentTypeSlice == nil {
		w.contentTypeSlice = make([]string, 1)
	}
	w.contentTypeSlice[0] = contentType
	w.w.Header()["Content-Type"] = w.contentTypeSlice
}

func (w *fastNetHTTPResponseWriter) SetContentLength(length string) {
	if w.contentLengthSlice == nil {
		w.contentLengthSlice = make([]string, 1)
	}
	w.contentLengthSlice[0] = length
	w.w.Header()["Content-Length"] = w.contentLengthSlice
}

type fastNetHTTPHeaderMap struct {
	h http.Header
}

func (m *fastNetHTTPHeaderMap) Set(key, value string)     { m.h.Set(key, value) }
func (m *fastNetHTTPHeaderMap) Get(key string) string     { return m.h.Get(key) }
func (m *fastNetHTTPHeaderMap) Del(key string)            { m.h.Del(key) }
func (m *fastNetHTTPHeaderMap) Add(key, value string)     { m.h.Add(key, value) }
func (m *fastNetHTTPHeaderMap) DirectHeader() http.Header { return m.h }
