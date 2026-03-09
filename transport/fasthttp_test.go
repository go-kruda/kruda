//go:build !windows

package transport

import (
	"context"
	"testing"
	"time"

	"github.com/valyala/fasthttp"
)

func TestFastHTTPTransport_Basic(t *testing.T) {
	cfg := FastHTTPConfig{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	transport := NewFastHTTP(cfg)

	// Test that we can create the transport and it has the right config
	if transport == nil {
		t.Fatal("NewFastHTTP returned nil")
	}
	if transport.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", transport.config.ReadTimeout)
	}

	// Test shutdown without starting (should not panic)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := transport.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

// fasthttpRequest tests
func TestFasthttpRequest_Method(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetMethod("POST")
	r := &fasthttpRequest{ctx: ctx}
	if r.Method() != "POST" {
		t.Errorf("expected POST, got %s", r.Method())
	}
}

func TestFasthttpRequest_Path(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/users/123")
	r := &fasthttpRequest{ctx: ctx}
	if r.Path() != "/users/123" {
		t.Errorf("expected /users/123, got %s", r.Path())
	}
}

func TestFasthttpRequest_Path_Empty(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx}
	if r.Path() != "/" {
		t.Errorf("expected /, got %s", r.Path())
	}
}

func TestFasthttpRequest_Header(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("Authorization", "Bearer token")
	r := &fasthttpRequest{ctx: ctx}
	if r.Header("Authorization") != "Bearer token" {
		t.Errorf("expected Bearer token, got %s", r.Header("Authorization"))
	}
	if r.Header("Missing") != "" {
		t.Errorf("expected empty, got %s", r.Header("Missing"))
	}
}

func TestFasthttpRequest_Body(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetBody([]byte(`{"key":"value"}`))
	r := &fasthttpRequest{ctx: ctx}
	body, err := r.Body()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"key":"value"}` {
		t.Errorf("expected {\"key\":\"value\"}, got %s", body)
	}
}

func TestFasthttpRequest_QueryParam(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/path?a=1&b=2")
	r := &fasthttpRequest{ctx: ctx}
	if r.QueryParam("a") != "1" {
		t.Errorf("expected 1, got %s", r.QueryParam("a"))
	}
	if r.QueryParam("b") != "2" {
		t.Errorf("expected 2, got %s", r.QueryParam("b"))
	}
	if r.QueryParam("missing") != "" {
		t.Errorf("expected empty, got %s", r.QueryParam("missing"))
	}
}

func TestFasthttpRequest_RemoteAddr_TrustProxy_XForwardedFor(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "1.2.3.4, 10.0.0.1")
	r := &fasthttpRequest{ctx: ctx, trustProxy: true}
	if r.RemoteAddr() != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4, got %s", r.RemoteAddr())
	}
}

func TestFasthttpRequest_RemoteAddr_TrustProxy_XRealIP(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Real-Ip", "5.6.7.8")
	r := &fasthttpRequest{ctx: ctx, trustProxy: true}
	if r.RemoteAddr() != "5.6.7.8" {
		t.Errorf("expected 5.6.7.8, got %s", r.RemoteAddr())
	}
}

func TestFasthttpRequest_RemoteAddr_NoTrustProxy(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", "1.2.3.4")
	r := &fasthttpRequest{ctx: ctx, trustProxy: false}
	addr := r.RemoteAddr()
	// Should not use proxy headers when trustProxy is false
	if addr == "1.2.3.4" {
		t.Errorf("should not use proxy header when trustProxy=false, got %s", addr)
	}
}

func TestFasthttpRequest_Cookie(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.SetCookie("session", "abc123")
	r := &fasthttpRequest{ctx: ctx}
	if r.Cookie("session") != "abc123" {
		t.Errorf("expected abc123, got %s", r.Cookie("session"))
	}
	if r.Cookie("missing") != "" {
		t.Errorf("expected empty, got %s", r.Cookie("missing"))
	}
}

func TestFasthttpRequest_RawRequest(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx}
	if r.RawRequest() != ctx {
		t.Error("RawRequest should return the underlying RequestCtx")
	}
}

func TestFasthttpRequest_MultipartForm(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx}
	_, err := r.MultipartForm(1024)
	// Should return error for non-multipart request
	if err == nil {
		t.Error("expected error for non-multipart request")
	}
}

func TestFasthttpRequest_Context(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx}
	if r.Context() != ctx {
		t.Error("Context should return the RequestCtx")
	}
}

func TestFasthttpRequest_AllHeaders(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Test", "value1")
	ctx.Request.Header.Set("X-Custom", "value2")
	r := &fasthttpRequest{ctx: ctx}
	headers := r.AllHeaders()
	if headers["X-Test"] != "value1" {
		t.Errorf("expected value1, got %s", headers["X-Test"])
	}
	if headers["X-Custom"] != "value2" {
		t.Errorf("expected value2, got %s", headers["X-Custom"])
	}
}

func TestFasthttpRequest_AllQuery(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/path?a=1&b=2&c=3")
	r := &fasthttpRequest{ctx: ctx}
	query := r.AllQuery()
	if query["a"] != "1" {
		t.Errorf("expected 1, got %s", query["a"])
	}
	if query["b"] != "2" {
		t.Errorf("expected 2, got %s", query["b"])
	}
	if query["c"] != "3" {
		t.Errorf("expected 3, got %s", query["c"])
	}
}

// fasthttpResponseWriter tests
func TestFasthttpResponseWriter_WriteHeader(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	w := &fasthttpResponseWriter{ctx: ctx}
	w.WriteHeader(201)
	if ctx.Response.StatusCode() != 201 {
		t.Errorf("expected 201, got %d", ctx.Response.StatusCode())
	}
}

func TestFasthttpResponseWriter_Write(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	w := &fasthttpResponseWriter{ctx: ctx}
	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5, got %d", n)
	}
	if string(ctx.Response.Body()) != "hello" {
		t.Errorf("expected hello, got %s", ctx.Response.Body())
	}
}

func TestFasthttpResponseWriter_Header(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	w := &fasthttpResponseWriter{ctx: ctx}
	h := w.Header()
	if h == nil {
		t.Fatal("Header() returned nil")
	}
}

// fasthttpHeaderMap tests
func TestFasthttpHeaderMap_Set(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	h := &fasthttpHeaderMap{ctx: ctx}
	h.Set("X-Test", "value")
	if string(ctx.Response.Header.Peek("X-Test")) != "value" {
		t.Errorf("expected value, got %s", ctx.Response.Header.Peek("X-Test"))
	}
}

func TestFasthttpHeaderMap_Add(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	h := &fasthttpHeaderMap{ctx: ctx}
	h.Add("X-Multi", "a")
	h.Add("X-Multi", "b")
	// fasthttp Add behavior - check that header exists
	if string(ctx.Response.Header.Peek("X-Multi")) == "" {
		t.Error("expected X-Multi header to be set")
	}
}

func TestFasthttpHeaderMap_Get(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Response.Header.Set("X-Test", "value")
	h := &fasthttpHeaderMap{ctx: ctx}
	if h.Get("X-Test") != "value" {
		t.Errorf("expected value, got %s", h.Get("X-Test"))
	}
	if h.Get("Missing") != "" {
		t.Errorf("expected empty, got %s", h.Get("Missing"))
	}
}

func TestFasthttpHeaderMap_Del(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Response.Header.Set("X-Test", "value")
	h := &fasthttpHeaderMap{ctx: ctx}
	h.Del("X-Test")
	if h.Get("X-Test") != "" {
		t.Errorf("expected empty after Del, got %s", h.Get("X-Test"))
	}
}

// Additional edge case tests
func TestFasthttpRequest_RemoteAddr_TrustProxy_SingleIP(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.Header.Set("X-Forwarded-For", " 1.2.3.4 ")
	r := &fasthttpRequest{ctx: ctx, trustProxy: true}
	if r.RemoteAddr() != "1.2.3.4" {
		t.Errorf("expected 1.2.3.4 (trimmed), got %s", r.RemoteAddr())
	}
}

func TestFasthttpRequest_RemoteAddr_TrustProxy_NoHeaders(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx, trustProxy: true}
	// Should fallback to RemoteAddr when no proxy headers
	addr := r.RemoteAddr()
	if addr == "" {
		t.Error("RemoteAddr should not be empty")
	}
}

func TestFastHTTPTransport_AllConfig(t *testing.T) {
	cfg := FastHTTPConfig{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
		MaxBodySize:  1024,
		Concurrency:  100,
		TrustProxy:   true,
	}
	transport := NewFastHTTP(cfg)

	if transport.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", transport.config.ReadTimeout)
	}
	if transport.config.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", transport.config.WriteTimeout)
	}
	if transport.config.IdleTimeout != 15*time.Second {
		t.Errorf("IdleTimeout = %v, want 15s", transport.config.IdleTimeout)
	}
	if transport.config.MaxBodySize != 1024 {
		t.Errorf("MaxBodySize = %d, want 1024", transport.config.MaxBodySize)
	}
	if transport.config.Concurrency != 100 {
		t.Errorf("Concurrency = %d, want 100", transport.config.Concurrency)
	}
	if !transport.config.TrustProxy {
		t.Error("TrustProxy should be true")
	}
}

func TestFasthttpRequest_QueryParam_Empty(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/path")
	r := &fasthttpRequest{ctx: ctx}
	if r.QueryParam("any") != "" {
		t.Errorf("expected empty, got %s", r.QueryParam("any"))
	}
}

func TestFasthttpRequest_AllQuery_Empty(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	ctx.Request.SetRequestURI("/path")
	r := &fasthttpRequest{ctx: ctx}
	query := r.AllQuery()
	if len(query) != 0 {
		t.Errorf("expected empty map, got %v", query)
	}
}

func TestFasthttpRequest_AllHeaders_Empty(t *testing.T) {
	ctx := &fasthttp.RequestCtx{}
	r := &fasthttpRequest{ctx: ctx}
	headers := r.AllHeaders()
	// Should have at least some default headers
	if headers == nil {
		t.Error("AllHeaders should not return nil")
	}
}

func TestFastHTTPTransport_ZeroConfig(t *testing.T) {
	cfg := FastHTTPConfig{} // All zero values
	transport := NewFastHTTP(cfg)

	if transport.config.ReadTimeout != 0 {
		t.Errorf("ReadTimeout = %v, want 0", transport.config.ReadTimeout)
	}
	if transport.config.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0", transport.config.WriteTimeout)
	}
	if transport.config.IdleTimeout != 0 {
		t.Errorf("IdleTimeout = %v, want 0", transport.config.IdleTimeout)
	}
	if transport.config.MaxBodySize != 0 {
		t.Errorf("MaxBodySize = %d, want 0", transport.config.MaxBodySize)
	}
	if transport.config.Concurrency != 0 {
		t.Errorf("Concurrency = %d, want 0", transport.config.Concurrency)
	}
	if transport.config.TrustProxy {
		t.Error("TrustProxy should be false by default")
	}
}

func TestFastHTTPIntegration(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedHeaders map[string]string
	var capturedQuery map[string]string

	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		capturedMethod = r.Method()
		capturedPath = r.Path()
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
}
