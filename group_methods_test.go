package kruda

import (
	"strings"
	"testing"
)

// --- Group typed handlers ---

func TestGroupGet(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Value string `json:"value"`
	}

	app := New()
	g := app.Group("/api")
	GroupGet[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Value: "ok"}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/api/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPost(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}
	type Output struct {
		ID int `json:"id"`
	}

	app := New()
	g := app.Group("/api")
	GroupPost[Input, Output](g, "/items", func(c *C[Input]) (*Output, error) {
		return &Output{ID: 1}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Post("/api/items", strings.NewReader(`{"name":"test"}`))
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPut(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Updated bool `json:"updated"`
	}

	app := New()
	g := app.Group("/api")
	GroupPut[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Updated: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Put("/api/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupDelete(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Deleted bool `json:"deleted"`
	}

	app := New()
	g := app.Group("/api")
	GroupDelete[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Deleted: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Delete("/api/items/1")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestGroupPatch(t *testing.T) {
	type Input struct {
		ID int `param:"id"`
	}
	type Output struct {
		Patched bool `json:"patched"`
	}

	app := New()
	g := app.Group("/api")
	GroupPatch[Input, Output](g, "/items/:id", func(c *C[Input]) (*Output, error) {
		return &Output{Patched: true}, nil
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Patch("/api/items/1", nil)
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

// --- Group catch-all and method-specific routes ---

func TestGroup_All_Methods(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.All("/catch", func(c *Ctx) error {
		return c.Text("caught " + c.Method())
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/api/catch")
	if !strings.Contains(resp.BodyString(), "caught GET") {
		t.Errorf("GET body = %q", resp.BodyString())
	}
	resp, _ = tc.Post("/api/catch", nil)
	if !strings.Contains(resp.BodyString(), "caught POST") {
		t.Errorf("POST body = %q", resp.BodyString())
	}
}

func TestGroup_HeadRoute(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.Head("/ping", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Head("/api/ping")
	if resp.StatusCode() != 204 {
		t.Errorf("HEAD /api/ping status = %d, want 204", resp.StatusCode())
	}
}

func TestGroup_OptionsRoute(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.Options("/cors", func(c *Ctx) error {
		c.SetHeader("Allow", "GET, POST")
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Options("/api/cors")
	if resp.StatusCode() != 204 {
		t.Errorf("OPTIONS /api/cors status = %d", resp.StatusCode())
	}
}

func TestGroup_AllRoute(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.All("/any", func(c *Ctx) error {
		return c.Text(c.Method())
	})
	app.Compile()

	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		req := &mockRequest{method: method, path: "/api/any"}
		resp := newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 200 {
			t.Errorf("%s /api/any status = %d", method, resp.statusCode)
		}
	}
}
