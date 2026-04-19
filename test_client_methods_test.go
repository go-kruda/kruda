package kruda

import (
	"testing"
)

// --- TestClient: verb helpers (Patch/Head/Options) ---

func TestTestClient_Patch(t *testing.T) {
	app := New()
	app.Patch("/items/:id", func(c *Ctx) error {
		return c.JSON(Map{"patched": true})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Patch("/items/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestTestClient_Head(t *testing.T) {
	app := New()
	app.Head("/ping", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Head("/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestTestClient_Options(t *testing.T) {
	app := New()
	app.Options("/ping", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Options("/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- TestResponse.Body ([]byte) ---

func TestTestResponse_Body(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.Text("hello")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	body := resp.Body()
	if len(body) == 0 {
		t.Error("Body() returned empty")
	}
}
