//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

func staticStreamBackends() []struct {
	name   string
	option Option
} {
	return []struct {
		name   string
		option Option
	}{
		{"nethttp", NetHTTP()},
		{"fasthttp", FastHTTP()},
		{"fasthttp-generic", WithTransport(staticGenericTransport{transport.NewFastHTTP(transport.FastHTTPConfig{})})},
		{"wing-inline", WithTransport(NewWingTransport(WingConfig{Workers: 1}))},
		{"wing-pool", WithTransport(NewWingTransport(WingConfig{Workers: 1, HandlerPoolSize: 1, DefaultPreset: Arrow}))},
		{"wing-spawn", WithTransport(NewWingTransport(WingConfig{Workers: 1, DefaultPreset: Preset{Dispatch: Spawn}}))},
		{"wing-takeover", WithTransport(NewWingTransport(WingConfig{Workers: 1, DefaultPreset: DB}))},
	}
}

func startStaticStreamApp(t *testing.T, app *App) string {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Error(ctx.Err())
		}
	})
	// Wing compiles its route table after entering Serve.
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	deadline := time.Now().Add(3 * time.Second)
	for {
		r, e := client.Get("http://" + ln.Addr().String() + "/ready")
		if e == nil {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			if r.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("startup: %v", e)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return ln.Addr().String()
}

type staticPatternWriter struct{ offset int64 }

func (w *staticPatternWriter) Write(p []byte) (int, error) {
	for i, b := range p {
		if b != byte((w.offset+int64(i))%251) {
			return i, fmt.Errorf("wrong file byte at %d", w.offset+int64(i))
		}
	}
	w.offset += int64(len(p))
	return len(p), nil
}

func waitStaticCloses(t *testing.T, files *boundedStaticFS, count int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for files.closes.Load() < count && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := files.closes.Load(); got != count {
		t.Fatalf("file closes=%d, want %d", got, count)
	}
}

func TestStaticStreamAcrossBackends(t *testing.T) {
	for _, backend := range staticStreamBackends() {
		t.Run(backend.name, func(t *testing.T) {
			files := &boundedStaticFS{size: 16 << 20}
			app := New(backend.option)
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			app.Get("/next", func(c *Ctx) error { return c.Text("next") })
			app.StaticFS("/assets", files, WithMaxAge(60))
			addr := startStaticStreamApp(t, app)
			conn, err := net.Dial("tcp4", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(15 * time.Second))
			io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\n")
			rd := bufio.NewReader(conn)
			first, err := http.ReadResponse(rd, nil)
			if err != nil {
				t.Fatal(err)
			}
			// New read readiness must not let these responses overtake the first body.
			io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\nGET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
			for i := 0; i < 2; i++ {
				resp := first
				if i > 0 {
					resp, err = http.ReadResponse(rd, nil)
					if err != nil {
						t.Fatal(err)
					}
				}
				if resp.StatusCode != 200 || resp.ContentLength != files.size || resp.Header.Get("Content-Type") != "application/octet-stream" || resp.Header.Get("Cache-Control") != "public, max-age=60" {
					t.Fatalf("response %d: status=%d length=%d headers=%v", i, resp.StatusCode, resp.ContentLength, resp.Header)
				}
				if n, e := io.Copy(&staticPatternWriter{}, resp.Body); e != nil || n != files.size {
					t.Fatalf("body %d: bytes=%d error=%v", i, n, e)
				}
				resp.Body.Close()
			}
			next, err := http.ReadResponse(rd, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(next.Body)
			next.Body.Close()
			if err != nil || string(body) != "next" {
				t.Fatalf("next body=%q error=%v", body, err)
			}
			waitStaticCloses(t, files, 2)
			if got := files.maxRead.Load(); got > 64*1024 {
				t.Fatalf("max read=%d", got)
			}
		})
	}
}

func TestStaticStreamDisconnectClosesFile(t *testing.T) {
	for _, backend := range staticStreamBackends() {
		t.Run(backend.name, func(t *testing.T) {
			files := &boundedStaticFS{size: 1 << 30}
			app := New(backend.option)
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			app.StaticFS("/assets", files)
			addr := startStaticStreamApp(t, app)
			conn, err := net.Dial("tcp4", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\n")
			resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != 200 {
				t.Fatalf("status=%d", resp.StatusCode)
			}
			conn.Close()
			waitStaticCloses(t, files, 1)
		})
	}
}

func TestStaticStreamTruncationClosesConnection(t *testing.T) {
	for _, backend := range staticStreamBackends() {
		t.Run(backend.name, func(t *testing.T) {
			files := &boundedStaticFS{size: 4 << 20, failAfter: 64 << 10}
			app := New(backend.option)
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			app.Get("/next", func(c *Ctx) error { return c.Text("NEXT-RESPONSE") })
			app.StaticFS("/assets", files)
			addr := startStaticStreamApp(t, app)
			conn, err := net.Dial("tcp4", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			io.WriteString(conn, "GET /assets/asset.bin HTTP/1.1\r\nHost: test\r\n\r\nGET /next HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n")
			rd := bufio.NewReader(conn)
			resp, err := http.ReadResponse(rd, nil)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				t.Fatal("truncated file returned a complete response")
			}
			if e, ok := err.(net.Error); ok && e.Timeout() {
				t.Fatalf("truncated file stalled: %v", err)
			}
			if strings.Contains(string(body), "NEXT-RESPONSE") {
				t.Fatal("next response contaminated truncated body")
			}
			waitStaticCloses(t, files, 1)
		})
	}
}

// Force the generic transport adapter instead of the app's fasthttp fast path.
type staticGenericHandler struct{ handler transport.Handler }

func (h staticGenericHandler) ServeKruda(w transport.ResponseWriter, r transport.Request) {
	h.handler.ServeKruda(w, r)
}

type staticGenericTransport struct{ transport.Transport }

func (tr staticGenericTransport) Serve(ln net.Listener, handler transport.Handler) error {
	return tr.Transport.Serve(ln, staticGenericHandler{handler})
}
