//go:build linux || darwin

package wing

import (
	"testing"

	"github.com/go-kruda/kruda/transport"
)

var rawGET = []byte("GET /users/42?page=1 HTTP/1.1\r\nHost: localhost\r\nAccept: text/plain\r\nConnection: keep-alive\r\n\r\n")

var rawPOST = []byte("POST /users HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 42\r\nConnection: keep-alive\r\n\r\n{\"name\":\"John\",\"email\":\"john@example.com\"}")

func BenchmarkParseGET(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = parseHTTPRequest(rawGET, noLimits)
	}
}

func BenchmarkParsePOST(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = parseHTTPRequest(rawPOST, noLimits)
	}
}

func BenchmarkResponseBuild(b *testing.B) {
	body := []byte(`{"message":"Hello, World!"}`)
	b.ReportAllocs()
	for b.Loop() {
		r := acquireResponse()
		r.Header().Set("Content-Type", "application/json")
		r.Write(body)
		_ = r.buildZeroCopy()
		r.buf = nil
		releaseResponse(r)
	}
}

func BenchmarkResponseBuildCopy(b *testing.B) {
	body := []byte(`{"message":"Hello, World!"}`)
	b.ReportAllocs()
	for b.Loop() {
		r := acquireResponse()
		r.Header().Set("Content-Type", "application/json")
		r.Write(body)
		_ = r.build()
		releaseResponse(r)
	}
}

func BenchmarkFullCycle(b *testing.B) {
	body := []byte("Hello, World!")
	b.ReportAllocs()
	for b.Loop() {
		req, _, _ := parseHTTPRequest(rawGET, noLimits)
		_ = req.Path()
		r := acquireResponse()
		r.Header().Set("Content-Type", "text/plain")
		r.Write(body)
		_ = r.buildZeroCopy()
		r.buf = nil
		releaseResponse(r)
	}
}

// BenchmarkHandlerInline simulates the inline handler path (no goroutine).
func BenchmarkHandlerInline(b *testing.B) {
	respBody := []byte("Hello, World!")
	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(respBody)
	})
	b.ReportAllocs()
	for b.Loop() {
		req, _, _ := parseHTTPRequest(rawGET, noLimits)
		resp := acquireResponse()
		handler.ServeKruda(resp, req)
		_ = resp.buildZeroCopy()
		resp.buf = nil
		releaseResponse(resp)
	}
}
