package kruda

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// --- Ctx.JSON: net/http path / stream encoder / custom encoder / error paths ---

func TestCtx_JSON_NetHTTPPath(t *testing.T) {
	// Use net/http transport to exercise the non-fasthttp JSON path
	app := New(NetHTTP())
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(Map{"key": "value"})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "value") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestCtx_JSON_WithStreamEncoder(t *testing.T) {
	// Default config has JSONStreamEncoder set
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(Map{"hello": "world"})
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
	if !strings.Contains(string(resp.body), "hello") {
		t.Errorf("body = %q", resp.body)
	}
}

func TestCtx_JSON_WithCustomEncoder(t *testing.T) {
	// Custom encoder disables stream path
	app := New(WithJSONEncoder(func(v any) ([]byte, error) {
		return []byte(`{"custom":true}`), nil
	}))
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(Map{"ignored": true})
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "custom") {
		t.Errorf("body = %q", resp.body)
	}
}

func TestCtx_JSON_EncoderError(t *testing.T) {
	app := New(WithJSONEncoder(func(v any) ([]byte, error) {
		return nil, errors.New("encode failed")
	}))
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(Map{"key": "value"})
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	// Should return error status
	if resp.statusCode == 200 {
		t.Error("should not return 200 on encoder error")
	}
}

func TestCtx_JSON_StreamEncoderError(t *testing.T) {
	app := New()
	// Override stream encoder to fail
	app.config.JSONStreamEncoder = func(buf *bytes.Buffer, v any) error {
		return errors.New("stream encode failed")
	}
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(Map{"key": "value"})
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode == 200 {
		t.Error("should not return 200 on stream encoder error")
	}
}

func TestCtx_JSON_AlreadyResponded(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		_ = c.Text("first")
		err := c.JSON(Map{"key": "value"})
		if err != ErrAlreadyResponded {
			return BadRequest("expected ErrAlreadyResponded")
		}
		return nil
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

// --- Ctx.JSON with various data types ---

func TestCtx_JSON_Slice(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.JSON([]string{"a", "b", "c"})
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), `"a"`) {
		t.Errorf("body = %q", resp.body)
	}
}

func TestCtx_JSON_Number(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(42)
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "42") {
		t.Errorf("body = %q", resp.body)
	}
}

func TestCtx_JSON_Null(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.JSON(nil)
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "null") {
		t.Errorf("body = %q", resp.body)
	}
}

// --- isCookieSeparator ---

func TestIsCookieSeparator(t *testing.T) {
	separators := []byte{'(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}', ' ', '\t'}
	for _, c := range separators {
		if !isCookieSeparator(c) {
			t.Errorf("isCookieSeparator(%q) = false, want true", string(c))
		}
	}
	nonSeparators := []byte{'a', 'z', '0', '9', '-', '_', '.', '!', '~'}
	for _, c := range nonSeparators {
		if isCookieSeparator(c) {
			t.Errorf("isCookieSeparator(%q) = true, want false", string(c))
		}
	}
}
