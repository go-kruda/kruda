//go:build linux || darwin

package kruda

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
		addr, shutdown := startSmokeApp(t, app)
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
		addr, shutdown := startSmokeApp(t, app)
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

	addr, shutdown := startSmokeApp(t, app)
	defer shutdown()
	requireSmokeGet(t, "http://"+addr+"/ping", "pong")
}

// TestTransportSmoke_Wing verifies the Wing transport (epoll on Linux,
// kqueue on macOS) can serve a basic request through the App. Like
// fasthttp, Wing needs a real listener.
//
// The handler responds with c.JSON rather than c.Text. On Wing, c.Text
// goes through transport.GetStaticResponseString, which caches response
// bytes globally and patches the Date header from a background goroutine
// every second. Under -race that background writer races with worker
// reads in tryParse (see transport/transport.go:144). c.JSON takes the
// JSONResponder fast path which writes a per-request body and avoids
// the shared cache. Once the static cache race is fixed this handler
// can switch back to c.Text.
func TestTransportSmoke_Wing(t *testing.T) {
	app := New(Wing())
	app.Get("/ping", func(c *Ctx) error { return c.JSON(Map{"msg": "pong"}) })
	app.Compile()

	if app.transportType != "wing" {
		t.Skipf("wing transport not selected on this platform (got %q)", app.transportType)
	}

	addr, shutdown := startSmokeApp(t, app)
	defer shutdown()
	requireSmokeGet(t, "http://"+addr+"/ping", `"msg":"pong"`)
}

// startSmokeApp binds the App's transport to a random TCP port, starts
// Serve in a goroutine, and waits until the port accepts connections.
// Returns the bound addr and a shutdown closure that the caller must defer.
func startSmokeApp(t *testing.T, app *App) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = app.transport.Serve(ln, app) // returns ErrServerClosed on Shutdown
	}()

	// Wait for the server to accept.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, dErr := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if dErr == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = app.transport.Shutdown(ctx)
		wg.Wait()
	}
	return addr, shutdown
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
