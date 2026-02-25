package transport

import (
	"testing"
)

func TestHandlerFunc_ServeKruda(t *testing.T) {
	var capturedMethod string
	var capturedPath string
	
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedMethod = r.Method()
		capturedPath = r.Path()
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})
	
	mockReq := &mockRequest{method: "GET", path: "/test"}
	mockResp := &mockResponseWriter{}
	
	handler.ServeKruda(mockResp, mockReq)
	
	if capturedMethod != "GET" {
		t.Errorf("expected GET, got %s", capturedMethod)
	}
	if capturedPath != "/test" {
		t.Errorf("expected /test, got %s", capturedPath)
	}
	if mockResp.statusCode != 200 {
		t.Errorf("expected 200, got %d", mockResp.statusCode)
	}
	if string(mockResp.data) != "ok" {
		t.Errorf("expected ok, got %s", mockResp.data)
	}
}

// Mock implementations for testing
type mockRequest struct {
	method string
	path   string
}

func (r *mockRequest) Method() string                { return r.method }
func (r *mockRequest) Path() string                  { return r.path }
func (r *mockRequest) Header(key string) string      { return "" }
func (r *mockRequest) Body() ([]byte, error)         { return nil, nil }
func (r *mockRequest) QueryParam(key string) string  { return "" }
func (r *mockRequest) RemoteAddr() string            { return "127.0.0.1" }
func (r *mockRequest) Cookie(name string) string     { return "" }
func (r *mockRequest) RawRequest() any               { return nil }

type mockResponseWriter struct {
	statusCode int
	data       []byte
	headers    map[string]string
}

func (w *mockResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *mockResponseWriter) Header() HeaderMap {
	if w.headers == nil {
		w.headers = make(map[string]string)
	}
	return &mockHeaderMap{headers: w.headers}
}

func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.data = append(w.data, data...)
	return len(data), nil
}

type mockHeaderMap struct {
	headers map[string]string
}

func (h *mockHeaderMap) Set(key, value string) { h.headers[key] = value }
func (h *mockHeaderMap) Add(key, value string) { h.headers[key] = value }
func (h *mockHeaderMap) Get(key string) string { return h.headers[key] }
func (h *mockHeaderMap) Del(key string)        { delete(h.headers, key) }
