package kruda

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

type contextValueKey string

func TestCtxContextUsesTransportRequestContext(t *testing.T) {
	const key contextValueKey = "request-value"

	app := New()
	var got any
	app.Get("/", func(c *Ctx) error {
		got = c.Context().Value(key)
		return c.Text("ok")
	})
	app.Compile()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "from-request"))
	resp := newMockResponse()

	app.ServeKruda(resp, transport.NewNetHTTPRequestWithLimit(req, app.config.BodyLimit))

	if got != "from-request" {
		t.Fatalf("Ctx.Context value = %v, want request context value", got)
	}
}

func TestServeHTTPContextUsesHTTPRequestContext(t *testing.T) {
	const key contextValueKey = "http-request-value"

	app := New()
	var got any
	app.Get("/", func(c *Ctx) error {
		got = c.Context().Value(key)
		return c.Text("ok")
	})
	app.Compile()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), key, "from-http-request"))
	resp := httptest.NewRecorder()

	app.ServeHTTP(resp, req)

	if got != "from-http-request" {
		t.Fatalf("Ctx.Context value = %v, want http request context value", got)
	}
}
