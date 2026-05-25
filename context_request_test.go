package kruda

import (
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

type contextValueKey string

type countingContextRequest struct {
	ctx   context.Context
	calls int
}

func (r *countingContextRequest) Method() string { return "GET" }
func (r *countingContextRequest) Path() string   { return "/" }
func (r *countingContextRequest) Header(string) string {
	return ""
}
func (r *countingContextRequest) Body() ([]byte, error) { return nil, nil }
func (r *countingContextRequest) QueryParam(string) string {
	return ""
}
func (r *countingContextRequest) RemoteAddr() string { return "" }
func (r *countingContextRequest) Cookie(string) string {
	return ""
}
func (r *countingContextRequest) RawRequest() any { return nil }
func (r *countingContextRequest) Context() context.Context {
	r.calls++
	return r.ctx
}
func (r *countingContextRequest) MultipartForm(int64) (*multipart.Form, error) {
	return nil, nil
}

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

func TestCtxResetDefersTransportRequestContextLookup(t *testing.T) {
	app := New()
	c := newCtx(app)
	req := &countingContextRequest{ctx: context.WithValue(context.Background(), contextValueKey("lazy"), "ok")}
	resp := newMockResponse()

	c.reset(resp, req)

	if req.calls != 0 {
		t.Fatalf("reset called request Context %d times, want 0", req.calls)
	}
	if got := c.Context().Value(contextValueKey("lazy")); got != "ok" {
		t.Fatalf("Context value = %v, want lazy request context value", got)
	}
	if req.calls != 1 {
		t.Fatalf("Context calls = %d, want 1", req.calls)
	}
}

func TestCtxResetWingUsesLazyRequestContext(t *testing.T) {
	app := New()
	c := newCtx(app)
	req := &wingRequest{ctx: context.WithValue(context.Background(), contextValueKey("wing-lazy"), "ok")}
	resp := acquireResponse()
	defer releaseResponse(resp)

	c.resetWing(resp, req)

	if got := c.Context().Value(contextValueKey("wing-lazy")); got != "ok" {
		t.Fatalf("Context value = %v, want lazy Wing request context value", got)
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
