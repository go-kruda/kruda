package kruda

import (
	"context"
	"testing"
)

// --- App lifecycle hooks: OnRequest / OnResponse / Before / After / OnError / OnParse ---

func TestApp_OnRequest(t *testing.T) {
	called := false
	app := New()
	app.OnRequest(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("OnRequest hook not called")
	}
}

func TestApp_OnResponse(t *testing.T) {
	called := false
	app := New()
	app.OnResponse(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("OnResponse hook not called")
	}
}

// TestApp_OnResponse_FiresOnPathTraversalError guards the documented "always
// runs — even on 404 and OnRequest errors" guarantee for OnResponse. Path
// traversal rejections used to short-circuit with an early return, skipping
// OnResponse — broken metrics/logging hooks for any rejected request.
func TestApp_OnResponse_FiresOnPathTraversalError(t *testing.T) {
	var (
		responseCalled bool
		errorCalled    bool
	)
	app := New(WithPathTraversal())
	app.OnResponse(func(c *Ctx) error {
		responseCalled = true
		return nil
	})
	app.OnError(func(c *Ctx, err error) {
		errorCalled = true
	})
	app.Get("/files/*path", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/%2e%2e/etc/passwd") // url-encoded "/../etc/passwd" — escapes root
	if err != nil {
		t.Fatalf("test client Get: %v", err)
	}
	if resp.StatusCode() != 400 {
		t.Errorf("path traversal: status = %d, want 400", resp.StatusCode())
	}
	if !errorCalled {
		t.Error("OnError hook not called on path traversal rejection")
	}
	if !responseCalled {
		t.Error("OnResponse hook not called on path traversal rejection")
	}
}

func TestApp_BeforeHandle(t *testing.T) {
	called := false
	app := New()
	app.BeforeHandle(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("BeforeHandle hook not called")
	}
}

func TestApp_AfterHandle(t *testing.T) {
	called := false
	app := New()
	app.AfterHandle(func(c *Ctx) error {
		called = true
		return nil
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/test")
	if !called {
		t.Error("AfterHandle hook not called")
	}
}

func TestApp_OnError(t *testing.T) {
	called := false
	app := New()
	app.OnError(func(c *Ctx, err error) {
		called = true
	})
	app.Get("/fail", func(c *Ctx) error {
		return InternalError("boom")
	})
	app.Compile()

	tc := NewTestClient(app)
	tc.Get("/fail")
	if !called {
		t.Error("OnError hook not called")
	}
}

func TestApp_OnParse(t *testing.T) {
	called := false
	app := New()
	app.OnParse(func(c *Ctx, input any) error {
		called = true
		return nil
	})
	if len(app.hooks.OnParse) != 1 {
		t.Errorf("OnParse hooks = %d, want 1", len(app.hooks.OnParse))
	}
	app.hooks.OnParse[0](nil, nil) // just invoke to test registration
	if !called {
		t.Error("OnParse hook not called")
	}
}

// --- App.OnShutdown order + panic recovery ---

func TestApp_OnShutdown_LIFO(t *testing.T) {
	var order []int
	app := New()
	app.OnShutdown(func() { order = append(order, 1) })
	app.OnShutdown(func() { order = append(order, 2) })
	app.OnShutdown(func() { order = append(order, 3) })

	app.runShutdownHooks()

	if len(order) != 3 {
		t.Fatalf("hook count = %d, want 3", len(order))
	}
	// LIFO order: 3, 2, 1
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("order = %v, want [3 2 1]", order)
	}
}

func TestApp_OnShutdown_PanicRecovery(t *testing.T) {
	var reached bool
	app := New()
	app.OnShutdown(func() { reached = true })
	app.OnShutdown(func() { panic("boom") }) // this runs first (LIFO)

	// Should not panic
	app.runShutdownHooks()

	if !reached {
		t.Error("second hook should still run after first panics")
	}
}

func TestOnShutdown_EmptyHooks(t *testing.T) {
	app := New()
	app.Compile()
	// Should not panic with empty hooks
	app.runShutdownHooks()
}

// --- App.Shutdown with/without container ---

func TestApp_Shutdown_WithContainer(t *testing.T) {
	c := NewContainer()
	// Register a service that implements Shutdowner interface
	_ = c.Give("test-value")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error { return ctx.Text("ok") })
	app.Compile()

	err := app.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}

func TestApp_Shutdown_WithoutContainer(t *testing.T) {
	app := New()
	app.Get("/test", func(ctx *Ctx) error { return ctx.Text("ok") })
	app.Compile()

	// Shutdown should work fine without a container
	err := app.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Shutdown error: %v", err)
	}
}
