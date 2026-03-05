//go:build linux || darwin

package wing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// skipIfNoEngine skips the test if the platform engine can't initialize.
func skipIfNoEngine(t *testing.T) {
	t.Helper()
	eng := newEngine()
	if err := eng.Init(engineConfig{RingSize: 4}); err != nil {
		t.Skipf("engine not available: %v", err)
	}
	eng.Close()
}

// echoHandler is a simple test handler that echoes the body back.
type echoHandler struct{}

func (h *echoHandler) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	body, _ := r.Body()
	w.Header().Set("Content-Type", "text/plain")
	if len(body) > 0 {
		w.Write(body)
	} else {
		w.Write([]byte("hello"))
	}
}

// jsonHandler returns a static JSON response for benchmarking.
type jsonHandler struct{}

func (h *jsonHandler) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"message":"Hello, World!"}`))
}

// getFreePort finds an available TCP port.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// startTransport starts the epoll/kqueue transport in background and returns
// the address and a cleanup function.
func startTransport(t *testing.T, handler transport.Handler) (string, func()) {
	t.Helper()

	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	tr := New(Config{
		Workers:     1, // single worker for test determinism
		RingSize:    64,
		ReadBufSize: 4096,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tr.ListenAndServe(addr, handler); err != nil {
			// Expected after shutdown.
		}
	}()

	// Wait for server to be ready.
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

// TestTransportBasicGET verifies a simple GET request through the full stack.
func TestTransportBasicGET(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})
	defer cleanup()

	resp, err := http.Get("http://" + addr + "/test")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want hello", body)
	}
}

// TestTransportPOSTWithBody verifies POST with body echo.
func TestTransportPOSTWithBody(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})
	defer cleanup()

	resp, err := http.Post("http://"+addr+"/echo", "text/plain",
		bytes.NewReader(bytes.Repeat([]byte("A"), 100)))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if len(body) != 100 {
		t.Errorf("body len = %d, want 100", len(body))
	}
}

// TestTransportConcurrent verifies the transport handles concurrent requests.
func TestTransportConcurrent(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})
	defer cleanup()

	const concurrency = 50
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := http.Get("http://" + addr + "/concurrent")
			if err != nil {
				t.Errorf("GET failed: %v", err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				t.Errorf("status = %d, want 200", resp.StatusCode)
			}
		}()
	}

	wg.Wait()
}

// TestTransportKeepAlive verifies HTTP keep-alive (multiple requests on one connection).
func TestTransportKeepAlive(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})
	defer cleanup()

	// Use a single HTTP client with keep-alive.
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 1,
			DisableKeepAlives:   false,
		},
	}

	for i := 0; i < 5; i++ {
		resp, err := client.Get("http://" + addr + "/keepalive")
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("request %d: status = %d", i, resp.StatusCode)
		}
	}
}

// TestTransportConnectionClose verifies Connection: close handling.
func TestTransportConnectionClose(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})
	defer cleanup()

	// Raw TCP — send Connection: close.
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	conn.Write([]byte("GET / HTTP/1.1\r\nConnection: close\r\n\r\n"))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	resp := string(buf[:n])

	if len(resp) == 0 {
		t.Fatal("got empty response")
	}

	// Server should close the connection after response.
	// A second read should return EOF or error.
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	n2, err := conn.Read(buf)
	if n2 > 0 && err == nil {
		t.Error("expected connection close after Connection: close, but got more data")
	}
}

// TestCreateListenFd verifies socket creation with SO_REUSEPORT.
func TestCreateListenFd(t *testing.T) {
	port := getFreePort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	fd1, err := createListenFd(addr)
	if err != nil {
		t.Fatalf("createListenFd failed: %v", err)
	}
	defer closeFd(fd1)

	// SO_REUSEPORT should allow a second bind on the same address.
	fd2, err := createListenFd(addr)
	if err != nil {
		t.Fatalf("second createListenFd failed (SO_REUSEPORT not working): %v", err)
	}
	defer closeFd(fd2)
}

// TestShutdownGraceful verifies clean shutdown.
func TestShutdownGraceful(t *testing.T) {
	skipIfNoEngine(t)

	addr, cleanup := startTransport(t, &echoHandler{})

	// Make a request to verify it's running.
	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	resp.Body.Close()

	// Shutdown should complete within 2 seconds.
	done := make(chan struct{})
	go func() {
		cleanup()
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

// end of tests
