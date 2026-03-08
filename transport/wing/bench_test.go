//go:build linux || darwin

package wing

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// ============================================================================
// Level 1 — CPU-only benchmarks
//
// These measure parse + build in memory. No TCP, no epoll, no concurrency.
// Useful for micro-optimizing the parser and response builder, but do NOT
// represent real throughput. For actual req/s, see Level 2 (Loopback TCP).
// ============================================================================

var rawGET = []byte("GET /users/42?page=1 HTTP/1.1\r\nHost: localhost\r\nAccept: text/plain\r\nConnection: keep-alive\r\n\r\n")

var rawPOST = []byte("POST /users HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: 42\r\nConnection: keep-alive\r\n\r\n{\"name\":\"John\",\"email\":\"john@example.com\"}")

func BenchmarkCPUParseGET(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		req, _, _ := parseHTTPRequest(rawGET, noLimits)
		if req != nil {
			releaseRequest(req)
		}
	}
}

func BenchmarkCPUParsePOST(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		req, _, _ := parseHTTPRequest(rawPOST, noLimits)
		if req != nil {
			releaseRequest(req)
		}
	}
}

func BenchmarkCPUResponseBuild(b *testing.B) {
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

func BenchmarkCPUResponseJSON(b *testing.B) {
	body := []byte(`{"message":"Hello, World!"}`)
	b.ReportAllocs()
	for b.Loop() {
		r := acquireResponse()
		r.SetJSON(200, body)
		_ = r.buildZeroCopy()
		r.buf = nil
		releaseResponse(r)
	}
}

func BenchmarkCPUFullCycle(b *testing.B) {
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
		releaseRequest(req)
	}
}

func BenchmarkCPUHandlerInline(b *testing.B) {
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
		releaseRequest(req)
	}
}

// ============================================================================
// Level 2 — Loopback TCP benchmarks
//
// These start a real Wing transport, send requests over TCP, and measure
// actual req/s through the full stack: accept → epoll → read → parse →
// handler → build → write. Use raw TCP (not http.Client) to avoid Go
// HTTP client overhead skewing results.
//
// Skip with -short flag since these involve real I/O.
// ============================================================================

// startBenchTransport starts a Wing transport with the given handler and
// returns the address and a cleanup function. Single worker for determinism.
func startBenchTransport(b *testing.B, handler transport.Handler, cfg ...Config) (string, func()) {
	b.Helper()

	eng := newEngine()
	if err := eng.Init(engineConfig{RingSize: 4}); err != nil {
		b.Skipf("engine not available: %v", err)
	}
	eng.Close()

	// Find free port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	c := Config{Workers: 1, RingSize: 64, ReadBufSize: 4096}
	if len(cfg) > 0 {
		c = cfg[0]
		c.defaults()
	}

	tr := New(c)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tr.ListenAndServe(addr, handler)
	}()

	// Wait for server ready.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr.Shutdown(ctx)
		wg.Wait()
	}
	return addr, cleanup
}

// BenchmarkLoopbackPlaintext measures real req/s for plaintext "Hello, World!" over loopback TCP.
// Uses raw net.Conn (not http.Client) to avoid Go HTTP client overhead.
func BenchmarkLoopbackPlaintext(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping loopback benchmark in short mode")
	}

	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World!"))
	})
	addr, cleanup := startBenchTransport(b, handler)
	defer cleanup()

	req := []byte("GET /plaintext HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	// Warm up.
	conn.Write(req)
	conn.Read(buf)

	b.SetBytes(int64(len(req)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		conn.Write(req)
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			b.Fatalf("read: n=%d err=%v", n, err)
		}
	}
}

// BenchmarkLoopbackJSON measures real req/s for JSON responses over loopback TCP.
func BenchmarkLoopbackJSON(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping loopback benchmark in short mode")
	}

	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"Hello, World!"}`))
	})
	addr, cleanup := startBenchTransport(b, handler)
	defer cleanup()

	req := []byte("GET /json HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	// Warm up.
	conn.Write(req)
	conn.Read(buf)

	b.SetBytes(int64(len(req)))
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		conn.Write(req)
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			b.Fatalf("read: n=%d err=%v", n, err)
		}
	}
}

// BenchmarkLoopbackConcurrent measures throughput with N concurrent keep-alive connections.
func BenchmarkLoopbackConcurrent(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping loopback benchmark in short mode")
	}

	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World!"))
	})
	addr, cleanup := startBenchTransport(b, handler, Config{
		Workers:     2,
		RingSize:    256,
		ReadBufSize: 4096,
	})
	defer cleanup()

	const numConns = 8
	req := []byte("GET /plaintext HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n")

	conns := make([]net.Conn, numConns)
	for i := range conns {
		c, err := net.Dial("tcp", addr)
		if err != nil {
			b.Fatalf("dial[%d]: %v", i, err)
		}
		conns[i] = c
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	// Warm up all connections.
	buf := make([]byte, 4096)
	for _, c := range conns {
		c.Write(req)
		c.Read(buf)
	}

	b.SetBytes(int64(len(req)) * numConns)
	b.ReportAllocs()
	b.ResetTimer()

	var wg sync.WaitGroup
	for b.Loop() {
		wg.Add(numConns)
		for _, c := range conns {
			go func(c net.Conn) {
				defer wg.Done()
				c.Write(req)
				buf := make([]byte, 4096)
				c.Read(buf)
			}(c)
		}
		wg.Wait()
	}
}

// ============================================================================
// Level 3 — Regression guard
//
// Fails the test if throughput drops below a minimum threshold.
// Runs a fixed number of requests and checks elapsed time.
// Skip with -short flag.
// ============================================================================

// startGuardTransport starts a Wing transport for use in *testing.T tests.
// Mirrors startBenchTransport but accepts *testing.T.
func startGuardTransport(t *testing.T, handler transport.Handler) (string, func()) {
	t.Helper()

	eng := newEngine()
	if err := eng.Init(engineConfig{RingSize: 4}); err != nil {
		t.Skipf("engine not available: %v", err)
	}
	eng.Close()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	tr := New(Config{Workers: 1, RingSize: 64, ReadBufSize: 4096})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tr.ListenAndServe(addr, handler)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr.Shutdown(ctx)
		wg.Wait()
	}
	return addr, cleanup
}

// TestPlaintextPerformanceGuard ensures plaintext throughput stays above
// a conservative floor (50k req/s on loopback). This catches accidental
// regressions like adding a mutex on the hot path or a syscall per request.
func TestPlaintextPerformanceGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance guard in short mode")
	}
	if runtime.GOOS != "linux" {
		t.Skip("performance guard only meaningful on Linux (Wing uses epoll)")
	}
	if os.Getenv("CI") != "" {
		t.Skip("skipping performance guard in CI (use dedicated hardware for perf testing)")
	}

	handler := transport.HandlerFunc(func(w transport.ResponseWriter, r transport.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello, World!"))
	})
	addr, cleanup := startGuardTransport(t, handler)
	defer cleanup()

	req := []byte("GET /plaintext HTTP/1.1\r\nHost: localhost\r\nConnection: keep-alive\r\n\r\n")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 4096)
	// Warm up.
	conn.Write(req)
	conn.Read(buf)

	const iterations = 10_000
	const minReqPerSec = 50_000 // conservative floor — any modern machine should hit this

	start := time.Now()
	for range iterations {
		conn.Write(req)
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			t.Fatalf("read failed: n=%d err=%v", n, err)
		}
	}
	elapsed := time.Since(start)

	reqPerSec := float64(iterations) / elapsed.Seconds()
	if reqPerSec < minReqPerSec {
		t.Errorf("throughput regression: got %.0f req/s, want >= %d req/s (elapsed %v)",
			reqPerSec, minReqPerSec, elapsed)
	}
	t.Logf("plaintext throughput: %.0f req/s (%d reqs in %v)", reqPerSec, iterations, elapsed)
}
