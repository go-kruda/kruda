//go:build !windows

package transport

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// startTransport starts a transport on a random port and returns the address and a shutdown function.
func startTransport(t *testing.T, tr Transport, handler Handler) (addr string, shutdown func()) {
	t.Helper()

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr = ln.Addr().String()
	ln.Close() // release so the transport can bind

	errCh := make(chan error, 1)
	go func() {
		errCh <- tr.ListenAndServe(addr, handler)
	}()

	// Wait for server to accept connections.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	shutdown = func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		tr.Shutdown(ctx)
	}
	return addr, shutdown
}

// httpClient returns an *http.Client with a short timeout for integration tests.
func httpClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Second}
}

// transportFactories returns factory functions for each transport implementation.
func transportFactories() []struct {
	name    string
	factory func() Transport
} {
	return []struct {
		name    string
		factory func() Transport
	}{
		{"nethttp", func() Transport {
			return NewNetHTTP(NetHTTPConfig{
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				MaxBodySize:  1024,
			})
		}},
		{"netpoll", func() Transport {
			tr, _ := NewNetpoll(NetpollConfig{
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 5 * time.Second,
				MaxBodySize:  1024,
			})
			return tr
		}},
	}
}

func TestTransportEquivalence(t *testing.T) {
	for _, tf := range transportFactories() {
		tf := tf
		t.Run(tf.name, func(t *testing.T) {
			t.Run("StaticRoute", func(t *testing.T) {
				testStaticRoute(t, tf.factory)
			})
			t.Run("Headers", func(t *testing.T) {
				testHeaders(t, tf.factory)
			})
			t.Run("QueryParams", func(t *testing.T) {
				testQueryParams(t, tf.factory)
			})
			t.Run("JSONBody", func(t *testing.T) {
				testJSONBody(t, tf.factory)
			})
			t.Run("StatusCodes", func(t *testing.T) {
				testStatusCodes(t, tf.factory)
			})
			t.Run("BodySizeLimit", func(t *testing.T) {
				testBodySizeLimit(t, tf.factory)
			})
			t.Run("EmptyBody", func(t *testing.T) {
				testEmptyBody(t, tf.factory)
			})
			t.Run("MultipleHeaders", func(t *testing.T) {
				testMultipleHeaders(t, tf.factory)
			})
		})
	}
}

// testStaticRoute verifies a handler returning a static "hello" response.
func testStaticRoute(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("hello"))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	resp, err := httpClient().Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", string(body), "hello")
	}
}

// testHeaders verifies request header reading and response header writing.
func testHeaders(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		val := r.Header("X-Test")
		w.Header().Set("X-Echo", val)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
	req.Header.Set("X-Test", "kruda-value")

	resp, err := httpClient().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	if got := resp.Header.Get("X-Echo"); got != "kruda-value" {
		t.Errorf("X-Echo = %q, want %q", got, "kruda-value")
	}
}

// testQueryParams verifies query parameter reading.
func testQueryParams(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		name := r.QueryParam("name")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte(name))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	resp, err := httpClient().Get("http://" + addr + "/greet?name=kruda")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "kruda" {
		t.Errorf("body = %q, want %q", string(body), "kruda")
	}
}

// testJSONBody verifies reading a POST body and echoing it back as JSON.
func testJSONBody(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		data, err := r.Body()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(data)
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	jsonBody := `{"framework":"kruda"}`
	resp, err := httpClient().Post("http://"+addr+"/echo", "application/json", strings.NewReader(jsonBody))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != jsonBody {
		t.Errorf("body = %q, want %q", string(body), jsonBody)
	}
}

// testStatusCodes verifies that various HTTP status codes are returned correctly.
func testStatusCodes(t *testing.T, factory func() Transport) {
	codes := []int{201, 404, 500}

	for _, code := range codes {
		code := code
		t.Run(fmt.Sprintf("Status%d", code), func(t *testing.T) {
			handler := HandlerFunc(func(w ResponseWriter, r Request) {
				w.WriteHeader(code)
				w.Write([]byte(fmt.Sprintf("status:%d", code)))
			})

			addr, shutdown := startTransport(t, factory(), handler)
			defer shutdown()

			resp, err := httpClient().Get("http://" + addr + "/")
			if err != nil {
				t.Fatalf("GET failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != code {
				t.Errorf("status = %d, want %d", resp.StatusCode, code)
			}

			body, _ := io.ReadAll(resp.Body)
			want := fmt.Sprintf("status:%d", code)
			if string(body) != want {
				t.Errorf("body = %q, want %q", string(body), want)
			}
		})
	}
}

// testBodySizeLimit verifies that both transports reject bodies exceeding MaxBodySize.
func testBodySizeLimit(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		_, err := r.Body()
		if err != nil {
			w.WriteHeader(413)
			w.Write([]byte("body too large"))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	// MaxBodySize is 1024 — send 2048 bytes.
	bigBody := strings.Repeat("x", 2048)
	resp, err := httpClient().Post("http://"+addr+"/upload", "application/octet-stream", strings.NewReader(bigBody))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	// Both transports should reject — handler returns 413.
	if resp.StatusCode != 413 {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 413; body = %q", resp.StatusCode, string(body))
	}
}

// testEmptyBody verifies GET requests with no body are handled correctly.
func testEmptyBody(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		data, err := r.Body()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("len:%d", len(data))))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	resp, err := httpClient().Get("http://" + addr + "/empty")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "len:0" {
		t.Errorf("body = %q, want %q", string(body), "len:0")
	}
}

// testMultipleHeaders verifies that multiple response headers are all present.
func testMultipleHeaders(t *testing.T, factory func() Transport) {
	handler := HandlerFunc(func(w ResponseWriter, r Request) {
		w.Header().Set("X-One", "1")
		w.Header().Set("X-Two", "2")
		w.Header().Set("X-Three", "3")
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("headers"))
	})

	addr, shutdown := startTransport(t, factory(), handler)
	defer shutdown()

	resp, err := httpClient().Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	checks := map[string]string{
		"X-One":   "1",
		"X-Two":   "2",
		"X-Three": "3",
	}
	for key, want := range checks {
		if got := resp.Header.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}
