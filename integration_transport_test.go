//go:build linux || darwin

package kruda

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestTransportSmoke_NetHTTP exercises the net/http transport end-to-end.
// Two paths reach net/http inside Kruda:
//
//  1. App.ServeHTTP — used when the App is mounted as an http.Handler (TLS,
//     Windows fallback, custom servers). httptest.Server takes the App
//     directly, which exercises this path.
//  2. transport.NetHTTPTransport.Serve — used when Listen() runs and selects
//     "nethttp". This adapter calls App.ServeKruda.
//
// Both paths share the same Ctx pool, but they own different response
// writers, so each sub-test uses a fresh App to keep state isolated.
func TestTransportSmoke_NetHTTP(t *testing.T) {
	t.Run("ServeHTTP via httptest", func(t *testing.T) {
		app := New(NetHTTP())
		app.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
		app.Compile()
		srv := httptest.NewServer(app)
		defer srv.Close()
		requireSmokeGet(t, srv.URL+"/ping", "pong")
	})

	t.Run("Transport.Serve on TCP listener", func(t *testing.T) {
		app := New(NetHTTP())
		app.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
		app.Compile()
		addr, shutdown := startSmokeApp(t, app, "/ping")
		defer shutdown()
		requireSmokeGet(t, "http://"+addr+"/ping", "pong")
	})
}

func TestTransportSecurityHeadersAndCookies_NetHTTP(t *testing.T) {
	newApp := func() *App {
		app := New(NetHTTP(), WithSecureHeaders())
		app.Get("/headers/json", func(c *Ctx) error {
			c.SetCookie(&Cookie{Name: "sid", Value: "abc", Path: "/", HTTPOnly: true})
			return c.JSON(Map{"ok": true})
		})
		app.Get("/headers/text", func(c *Ctx) error {
			c.SetCookie(&Cookie{Name: "sid", Value: "abc", Path: "/", HTTPOnly: true})
			return c.Text("ok")
		})
		app.Compile()
		return app
	}

	t.Run("ServeHTTP via httptest", func(t *testing.T) {
		srv := httptest.NewServer(newApp())
		defer srv.Close()
		requireSecurityCookieGet(t, srv.URL+"/headers/json")
		requireSecurityCookieGet(t, srv.URL+"/headers/text")
	})

	t.Run("Transport.Serve on TCP listener", func(t *testing.T) {
		app := newApp()
		addr, shutdown := startSmokeApp(t, app, "/headers/json")
		defer shutdown()
		requireSecurityCookieGet(t, "http://"+addr+"/headers/json")
		requireSecurityCookieGet(t, "http://"+addr+"/headers/text")
	})
}

// TestTransportSmoke_FastHTTP verifies the fasthttp transport can serve a
// basic request through the App's normal route registration path. The
// fasthttp transport requires a real TCP listener (no httptest equivalent),
// so we bind to 127.0.0.1:0 and shut down via the transport's Shutdown.
func TestTransportSmoke_FastHTTP(t *testing.T) {
	app := New(FastHTTP())
	app.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
	app.Compile()

	if app.transportType != "fasthttp" {
		t.Skipf("fasthttp transport not selected on this platform (got %q)", app.transportType)
	}

	addr, shutdown := startSmokeApp(t, app, "/ping")
	defer shutdown()
	requireSmokeGet(t, "http://"+addr+"/ping", "pong")
}

// TestTransportSmoke_Wing verifies the Wing transport (epoll on Linux,
// kqueue on macOS) can serve a basic request through the App. Like
// fasthttp, Wing needs a real listener.
//
// The handler responds with c.Text on purpose: it rides the Wing string
// fast lane (transport.StringResponder), which serializes a fresh response
// per request — no shared static cache, no background Date patching, so it
// must stay race-free under -race.
func TestTransportSmoke_Wing(t *testing.T) {
	app := New(Wing())
	app.Get("/ping", func(c *Ctx) error { return c.Text("pong") })
	app.Compile()

	if app.transportType != "wing" {
		t.Skipf("wing transport not selected on this platform (got %q)", app.transportType)
	}

	addr, shutdown := startSmokeApp(t, app, "/ping")
	defer shutdown()
	requireSmokeGet(t, "http://"+addr+"/ping", "pong")
}

// TestTransportSecurityHeadersAndCookies_Wing proves the string/JSON fast
// lanes step aside when the response needs headers the lane cannot carry:
// with WithSecureHeaders and a cookie set, canBypassHeaderWrite returns
// false, c.JSON/c.Text/c.HTML take the generic path, and the secure headers
// and Set-Cookie must appear on the wire.
func TestTransportSecurityHeadersAndCookies_Wing(t *testing.T) {
	app := New(Wing(), WithSecureHeaders())
	app.Get("/headers/json", func(c *Ctx) error {
		c.SetCookie(&Cookie{Name: "sid", Value: "abc", Path: "/", HTTPOnly: true})
		return c.JSON(Map{"ok": true})
	})
	app.Get("/headers/text", func(c *Ctx) error {
		c.SetCookie(&Cookie{Name: "sid", Value: "abc", Path: "/", HTTPOnly: true})
		return c.Text("ok")
	})
	app.Get("/headers/html", func(c *Ctx) error {
		c.SetCookie(&Cookie{Name: "sid", Value: "abc", Path: "/", HTTPOnly: true})
		return c.HTML("<b>ok</b>")
	})
	app.Compile()

	if app.transportType != "wing" {
		t.Skipf("wing transport not selected on this platform (got %q)", app.transportType)
	}

	addr, shutdown := startSmokeApp(t, app, "/headers/json")
	defer shutdown()
	requireSecurityCookieGet(t, "http://"+addr+"/headers/json")
	requireSecurityCookieGet(t, "http://"+addr+"/headers/text")
	requireSecurityCookieGet(t, "http://"+addr+"/headers/html")
}

// TestTransportTakeoverKeepAlive_Wing drives the Takeover dispatch loop end
// to end on one raw TCP connection: the first request enters takeoverLoop
// from the event loop, the second is read inside the loop itself, two
// pipelined requests arrive in a single segment (exercising the parse-
// without-read path), and a final Connection: close request must end with
// the server closing the fd — which the worker does through the *os.File
// handed back from the takeover goroutine.
func TestTransportTakeoverKeepAlive_Wing(t *testing.T) {
	app := New(Wing())
	app.Get("/take", func(c *Ctx) error { return c.Text("spear") }, DB)
	app.Compile()

	if app.transportType != "wing" {
		t.Skipf("wing transport not selected on this platform (got %q)", app.transportType)
	}

	addr, shutdown := startSmokeApp(t, app, "/take")
	defer shutdown()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReader(conn)

	keepAliveReq := "GET /take HTTP/1.1\r\nHost: test\r\n\r\n"
	readResp := func(step string) {
		t.Helper()
		resp, err := http.ReadResponse(br, nil)
		if err != nil {
			t.Fatalf("%s: read response: %v", step, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatalf("%s: read body: %v", step, err)
		}
		if resp.StatusCode != http.StatusOK || string(body) != "spear" {
			t.Fatalf("%s: got status %d body %q, want 200 %q", step, resp.StatusCode, body, "spear")
		}
	}

	if _, err := conn.Write([]byte(keepAliveReq)); err != nil {
		t.Fatalf("write first request: %v", err)
	}
	readResp("first request (event loop -> takeover)")

	if _, err := conn.Write([]byte(keepAliveReq)); err != nil {
		t.Fatalf("write second request: %v", err)
	}
	readResp("second request (read inside takeoverLoop)")

	if _, err := conn.Write([]byte(keepAliveReq + keepAliveReq)); err != nil {
		t.Fatalf("write pipelined requests: %v", err)
	}
	readResp("pipelined request 1")
	readResp("pipelined request 2")

	closeReq := "GET /take HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"
	if _, err := conn.Write([]byte(closeReq)); err != nil {
		t.Fatalf("write close request: %v", err)
	}
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("close request: read response: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "spear" {
		t.Fatalf("close request: got status %d body %q", resp.StatusCode, body)
	}
	if _, err := br.ReadByte(); err != io.EOF {
		t.Fatalf("after Connection: close, want EOF from server, got %v", err)
	}
}

// startSmokeApp binds the App's transport to a random TCP port, starts
// Serve in a goroutine, and waits until the app serves an HTTP request.
// Returns the bound addr and a shutdown closure that the caller must defer.
func startSmokeApp(t *testing.T, app *App, readyPath string) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.transport.Serve(ln, app) // returns ErrServerClosed on Shutdown
	}()

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = app.transport.Shutdown(ctx)
		select {
		case <-done:
		case <-ctx.Done():
			t.Errorf("transport shutdown timed out: %v", ctx.Err())
		}
	}

	// Wait until the app can serve an actual HTTP request. A bare TCP dial can
	// succeed before an async transport has registered its read loop.
	// DisableKeepAlives so the probe never holds a connection slot — otherwise a
	// lingering idle keep-alive conn counts against a tight WithMaxConns cap.
	rt := &http.Transport{DisableKeepAlives: true}
	client := &http.Client{Timeout: 250 * time.Millisecond, Transport: rt}
	readyURL := "http://" + addr + readyPath
	var lastErr error
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, getErr := client.Get(readyURL)
		if getErr == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				rt.CloseIdleConnections()
				return addr, shutdown
			}
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = getErr
		}
		time.Sleep(25 * time.Millisecond)
	}

	shutdown()
	t.Fatalf("server did not become ready at %s: %v", readyURL, lastErr)
	return "", func() {}
}

// requireSmokeGet performs a GET against url and fails the test unless the
// response body contains want.
func requireSmokeGet(t *testing.T, url, want string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s status=%d want=200", url, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), want) {
		t.Errorf("GET %s body=%q want contains %q", url, body, want)
	}
}

func requireSecurityCookieGet(t *testing.T, url string) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET %s status=%d want=200", url, resp.StatusCode)
	}
	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("GET %s X-Content-Type-Options=%q", url, got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("GET %s X-Frame-Options=%q", url, got)
	}
	if got := resp.Header.Get("Set-Cookie"); !strings.Contains(got, "sid=abc") || !strings.Contains(got, "HttpOnly") {
		t.Fatalf("GET %s Set-Cookie=%q", url, got)
	}
	if got := resp.Header.Get("Server"); got != "" {
		t.Fatalf("GET %s Server header should not be set, got %q", url, got)
	}
}
