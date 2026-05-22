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

func TestWingPreflightWithSecureHeadersKeepsAllHeaders(t *testing.T) {
	app := New(Wing(), WithSecureHeaders())
	app.Use(func(c *Ctx) error {
		if c.Method() == "OPTIONS" {
			c.AddHeader("Vary", "Origin")
			c.SetHeader("Access-Control-Allow-Origin", "https://example.com")
			c.SetHeader("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			c.SetHeader("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.SetHeader("Access-Control-Max-Age", "86400")
			c.SetHeader("Access-Control-Allow-Credentials", "true")
			return c.NoContent()
		}
		return c.Next()
	})
	app.Options("/resource", func(c *Ctx) error { return c.NoContent() })
	app.Get("/resource", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	resp := serveWingResponse(t, app, "OPTIONS", "/resource")

	expected := map[string]string{
		"Vary":                             "Origin",
		"Access-Control-Allow-Origin":      "https://example.com",
		"Access-Control-Allow-Methods":     "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers":     "Origin, Content-Type, Authorization",
		"Access-Control-Max-Age":           "86400",
		"Access-Control-Allow-Credentials": "true",
		"X-Content-Type-Options":           "nosniff",
		"X-Frame-Options":                  "DENY",
		"X-Xss-Protection":                 "0",
		"Referrer-Policy":                  "strict-origin-when-cross-origin",
	}
	for key, want := range expected {
		if got := resp.headers.Get(key); got != want {
			t.Fatalf("%s = %q, want %q\nresponse:\n%s", key, got, want, string(resp.build()))
		}
	}
}

func serveWingResponse(t *testing.T, app *App, method, path string) *wingResponse {
	t.Helper()

	resp := acquireResponse()
	t.Cleanup(func() { releaseResponse(resp) })

	app.ServeKruda(resp, &wingRequest{method: method, path: path})
	return resp
}
