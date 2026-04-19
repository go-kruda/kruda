package kruda

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// --- Ctx.Bind / Bind_EmptyBody ---

func TestCtx_Bind(t *testing.T) {
	app := New()
	app.Post("/bind-test", func(c *Ctx) error {
		var data struct {
			Name string `json:"name"`
		}
		if err := c.Bind(&data); err != nil {
			return err
		}
		return c.JSON(Map{"got": data.Name})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/bind-test", strings.NewReader(`{"name":"test"}`))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestCtx_Bind_EmptyBody(t *testing.T) {
	app := New()
	app.Post("/bind-empty", func(c *Ctx) error {
		var data struct{ Name string }
		if err := c.Bind(&data); err != nil {
			return c.Status(400).JSON(Map{"error": err.Error()})
		}
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/bind-empty", nil)
	if resp.StatusCode() != 400 {
		t.Errorf("status = %d, want 400 for empty body", resp.StatusCode())
	}
}

// --- Ctx.BodyBytes / BodyString ---

func TestCtx_BodyBytes(t *testing.T) {
	app := New()
	app.Post("/echo-body", func(c *Ctx) error {
		body, err := c.BodyBytes()
		if err != nil {
			return err
		}
		return c.Text(string(body))
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/echo-body", strings.NewReader("hello body"))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestCtx_BodyString(t *testing.T) {
	app := New()
	app.Post("/body-str", func(c *Ctx) error {
		bs := c.BodyString()
		return c.Text("got: " + bs)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/body-str", []byte("hello"))
	if !strings.Contains(resp.BodyString(), "got: hello") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// --- isBodyTooLarge classifier ---

func TestIsBodyTooLarge_True(t *testing.T) {
	err := &transport.BodyTooLargeError{}
	if !isBodyTooLarge(err) {
		t.Error("isBodyTooLarge should return true for BodyTooLargeError")
	}
}

func TestIsBodyTooLarge_False(t *testing.T) {
	if isBodyTooLarge(errors.New("some other error")) {
		t.Error("isBodyTooLarge should return false for non-BodyTooLargeError")
	}
}

func TestIsBodyTooLarge_Nil(t *testing.T) {
	if isBodyTooLarge(nil) {
		t.Error("isBodyTooLarge should return false for nil")
	}
}
