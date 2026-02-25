package transport

import (
	"testing"
	
	"github.com/valyala/fasthttp"
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
	
	// Create a mock request and response
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

func TestFastHTTPIntegration(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedHeaders map[string]string
	var capturedQuery map[string]string
	
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedMethod = r.Method()
		capturedPath = r.Path()
		
		// Test provider interfaces
		if mp, ok := r.(MultipartProvider); ok {
			_, _ = mp.MultipartForm(1024) // Test interface
		}
		if cp, ok := r.(ContextProvider); ok {
			_ = cp.Context() // Test interface
		}
		if ahp, ok := r.(AllHeadersProvider); ok {
			capturedHeaders = ahp.AllHeaders()
		}
		if aqp, ok := r.(AllQueryProvider); ok {
			capturedQuery = aqp.AllQuery()
		}
		
		w.Header().Set("X-Test", "response")
		w.WriteHeader(201)
		w.Write([]byte("created"))
	})
	
	// Create fasthttp context
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	ctx.Request.SetRequestURI("/api/test?param=value")
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.SetBody([]byte(`{"data":"test"}`))
	
	req := &fasthttpRequest{ctx: ctx, trustProxy: false}
	resp := &fasthttpResponseWriter{ctx: ctx}
	
	handler.ServeKruda(resp, req)
	
	if capturedMethod != "POST" {
		t.Errorf("expected POST, got %s", capturedMethod)
	}
	if capturedPath != "/api/test" {
		t.Errorf("expected /api/test, got %s", capturedPath)
	}
	if capturedHeaders["Content-Type"] != "application/json" {
		t.Errorf("expected application/json, got %s", capturedHeaders["Content-Type"])
	}
	if capturedQuery["param"] != "value" {
		t.Errorf("expected value, got %s", capturedQuery["param"])
	}
	if ctx.Response.StatusCode() != 201 {
		t.Errorf("expected 201, got %d", ctx.Response.StatusCode())
	}
	if string(ctx.Response.Body()) != "created" {
		t.Errorf("expected created, got %s", ctx.Response.Body())
	}
	if string(ctx.Response.Header.Peek("X-Test")) != "response" {
		t.Errorf("expected response, got %s", ctx.Response.Header.Peek("X-Test"))
	}
}

// Mock implementations for testing
type mockRequest struct {
	method string
	path   string
}

func (r *mockRequest) Method() string                    { return r.method }
func (r *mockRequest) Path() string                     { return r.path }
func (r *mockRequest) Header(key string) string         { return "" }
func (r *mockRequest) Body() ([]byte, error)            { return nil, nil }
func (r *mockRequest) QueryParam(key string) string     { return "" }
func (r *mockRequest) RemoteAddr() string               { return "127.0.0.1" }
func (r *mockRequest) Cookie(name string) string        { return "" }
func (r *mockRequest) RawRequest() any                  { return nil }

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