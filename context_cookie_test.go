package kruda

import (
	"strings"
	"testing"
)

// cookieMockRequest is a mockRequest variant that returns cookies.
type cookieMockRequest struct {
	mockRequest
	cookies map[string]string
}

func (r *cookieMockRequest) Cookie(name string) string {
	if r.cookies != nil {
		return r.cookies[name]
	}
	return ""
}

// --- Ctx.Cookie (TestClient flow) ---

func TestCtx_Cookie(t *testing.T) {
	app := New()
	app.Get("/test-cookie", func(c *Ctx) error {
		v := c.Cookie("session")
		return c.Text(v)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Request("GET", "/test-cookie").Cookie("session", "abc123").Send()
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Ctx.Cookie via mockRequest with explicit values / missing / nil request ---

func TestCtx_Cookie_WithValue(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.Text(c.Cookie("session"))
	})
	app.Compile()

	req := &cookieMockRequest{
		mockRequest: mockRequest{method: "GET", path: "/test"},
		cookies:     map[string]string{"session": "abc123"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "abc123") {
		t.Errorf("Cookie not returned, body = %q", resp.body)
	}
}

func TestCtx_Cookie_Missing(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		val := c.Cookie("nonexistent")
		if val != "" {
			return BadRequest("should be empty")
		}
		return c.Text("ok")
	})
	app.Compile()

	req := &cookieMockRequest{
		mockRequest: mockRequest{method: "GET", path: "/test"},
		cookies:     map[string]string{},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestCtx_Cookie_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	// c.request is nil
	val := c.Cookie("anything")
	if val != "" {
		t.Errorf("Cookie with nil request should return empty, got %q", val)
	}
}
