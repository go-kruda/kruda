//go:build linux || darwin

package kruda

import (
	"bytes"
	"testing"
)

func TestWingDateHeaderDoesNotEmitServerHeader(t *testing.T) {
	if bytes.Contains(dateHdr(), []byte("\r\nServer:")) {
		t.Fatal("wing date header chunk should not emit Server header")
	}
}

func TestWingAppTextWithSecureHeadersDoesNotEmitServerAndKeepsSecurityHeaders(t *testing.T) {
	app := New(Wing(), WithSecureHeaders())
	app.Get("/text", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	resp := serveWingResponse(t, app, "GET", "/text")

	if got := resp.headers.Get("Server"); got != "" {
		t.Fatalf("Server header should not be present, got %q", got)
	}
	if got := resp.headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatal("expected X-Content-Type-Options=nosniff")
	}
	if !bytes.Contains(resp.body, []byte("ok")) {
		t.Fatal("expected text response body")
	}
}

func TestWingAppJSONWithSecureHeadersDoesNotEmitServerAndKeepsSecurityHeaders(t *testing.T) {
	app := New(Wing(), WithSecureHeaders())
	app.Get("/json", func(c *Ctx) error { return c.JSON(Map{"ok": true}) })
	app.Compile()

	resp := serveWingResponse(t, app, "GET", "/json")

	if got := resp.headers.Get("Server"); got != "" {
		t.Fatalf("Server header should not be present, got %q", got)
	}
	if got := resp.headers.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatal("expected X-Content-Type-Options=nosniff")
	}
	if !bytes.Contains(resp.body, []byte(`"ok":true`)) {
		t.Fatal("expected JSON response body")
	}
}

func serveWingResponse(t *testing.T, app *App, method, path string) *wingResponse {
	t.Helper()

	resp := acquireResponse()
	t.Cleanup(func() { releaseResponse(resp) })

	app.ServeKruda(resp, &wingRequest{method: method, path: path})
	return resp
}
