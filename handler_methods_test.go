package kruda

import (
	"testing"
)

// --- Handler variants (X = no error return) ---

func TestHandlerPutX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Updated bool `json:"updated"`
	}

	app := New()
	PutX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Updated: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Put("/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestHandlerDeleteX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Deleted bool `json:"deleted"`
	}

	app := New()
	DeleteX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Deleted: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Delete("/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestHandlerPatchX(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Patched bool `json:"patched"`
	}

	app := New()
	PatchX[Input, Output](app, "/items/:id", func(c *C[Input]) *Output {
		return &Output{Patched: true}
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Patch("/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}
