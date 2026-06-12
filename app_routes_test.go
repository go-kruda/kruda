package kruda

import (
	"testing"
)

// --- App route methods (Options, Head, All) ---

func TestAppOptions(t *testing.T) {
	app := New()
	app.Options("/test", func(c *Ctx) error {
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Options("/test")
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestAppHead(t *testing.T) {
	app := New()
	app.Head("/test", func(c *Ctx) error {
		c.Set("X-Custom", "yes")
		return c.NoContent()
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Head("/test")
	if resp.StatusCode() != 204 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestApp_All_AllMethods(t *testing.T) {
	app := New()
	app.All("/everything", func(c *Ctx) error {
		return c.Text("method: " + c.Method())
	})
	app.Compile()

	tc := NewTestClient(app)
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, m := range methods {
		resp, _ := tc.Request(m, "/everything").Send()
		if resp.StatusCode() != 200 {
			t.Errorf("%s status = %d, want 200", m, resp.StatusCode())
		}
	}
}

// --- addRoute with Preset route option ---

func TestApp_AddRoute_WithWingOption(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.Text("ok")
	}, Plaintext)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/health"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestGroup_AddRoute_WithWingOption(t *testing.T) {
	app := New()
	g := app.Group("/api")
	g.Get("/data", func(c *Ctx) error {
		return c.JSON(Map{"ok": true})
	}, JSON)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/api/data"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestApp_AddRoute_WingStaticOptionNormalHandlerFallback(t *testing.T) {
	app := New(NetHTTP())
	app.Use(func(c *Ctx) error {
		c.SetHeader("X-Middleware-Ran", "yes")
		return c.Next()
	})
	app.Get("/healthz", func(c *Ctx) error {
		return c.Text("handler")
	}, StaticText(200, "text/plain; charset=utf-8", "static"))
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/healthz")
	if resp.StatusCode() != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode())
	}
	if resp.Header("X-Middleware-Ran") != "yes" {
		t.Fatalf("middleware header = %q, want yes", resp.Header("X-Middleware-Ran"))
	}
	if resp.BodyString() != "handler" {
		t.Fatalf("body = %q, want handler", resp.BodyString())
	}
}

// --- App.Use middleware chaining ---

func TestApp_Use_Chaining(t *testing.T) {
	app := New()
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }
	result := app.Use(mw1, mw2)
	if result != app {
		t.Error("Use should return app for chaining")
	}
	if len(app.middleware) != 2 {
		t.Errorf("middleware count = %d, want 2", len(app.middleware))
	}
}

// --- Ctx.Next ---

func TestCtx_Next(t *testing.T) {
	app := New()
	app.Use(func(c *Ctx) error {
		c.SetHeader("X-Before", "yes")
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}
