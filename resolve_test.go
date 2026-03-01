package kruda

import (
	"context"
	"sync/atomic"
	"testing"
)

// testDIService is a simple service for DI integration tests.
type testDIService struct {
	Name string
}

// TestWithContainerOption verifies that WithContainer attaches the container to the app.
func TestWithContainerOption(t *testing.T) {
	c := NewContainer()
	app := New(WithContainer(c))

	if app.container == nil {
		t.Fatal("expected container to be set on app")
	}
	if app.container != c {
		t.Fatal("expected same container instance")
	}
}

// TestWithContainerNil verifies that app works without container (Phase 1-3 behavior).
func TestWithContainerNil(t *testing.T) {
	app := New()

	if app.container != nil {
		t.Fatal("expected container to be nil by default")
	}
}

// TestResolveFromAppContainer verifies Resolve[T] works via app-level container
// (WithContainer fallback, no InjectMiddleware needed).
func TestResolveFromAppContainer(t *testing.T) {
	c := NewContainer()
	svc := &testDIService{Name: "hello"}
	if err := c.Give(svc); err != nil {
		t.Fatal(err)
	}

	app := New(WithContainer(c))

	var resolved *testDIService
	app.Get("/test", func(ctx *Ctx) error {
		var err error
		resolved, err = Resolve[*testDIService](ctx)
		if err != nil {
			return err
		}
		return ctx.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	if resolved != svc {
		t.Fatal("Resolve should return the same instance registered via Give")
	}
}

// TestResolveFromInjectMiddleware verifies Resolve[T] works via InjectMiddleware
// (context locals path).
func TestResolveFromInjectMiddleware(t *testing.T) {
	c := NewContainer()
	svc := &testDIService{Name: "injected"}
	if err := c.Give(svc); err != nil {
		t.Fatal(err)
	}

	app := New()
	app.Use(c.InjectMiddleware())

	var resolved *testDIService
	app.Get("/test", func(ctx *Ctx) error {
		var err error
		resolved, err = Resolve[*testDIService](ctx)
		if err != nil {
			return err
		}
		return ctx.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	if resolved != svc {
		t.Fatal("Resolve should return the same instance from InjectMiddleware")
	}
}

// TestResolveNoContainer verifies Resolve returns error when no container is configured.
func TestResolveNoContainer(t *testing.T) {
	app := New() // no container

	var resolveErr error
	app.Get("/test", func(ctx *Ctx) error {
		_, resolveErr = Resolve[*testDIService](ctx)
		return ctx.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resolveErr == nil {
		t.Fatal("expected error from Resolve when no container configured")
	}
	if resolveErr.Error() != "kruda: no container configured" {
		t.Fatalf("unexpected error: %v", resolveErr)
	}
}

// TestInjectMiddlewareSetsContainer verifies InjectMiddleware sets the container
// in context locals.
func TestInjectMiddlewareSetsContainer(t *testing.T) {
	c := NewContainer()

	app := New()
	app.Use(c.InjectMiddleware())

	var got any
	app.Get("/test", func(ctx *Ctx) error {
		got = ctx.Get("container")
		return ctx.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if got == nil {
		t.Fatal("expected container in context locals")
	}
	container, ok := got.(*Container)
	if !ok {
		t.Fatal("expected *Container type in locals")
	}
	if container != c {
		t.Fatal("expected same container instance in locals")
	}
}

// testShutdownService tracks OnShutdown calls.
type testShutdownService struct {
	shutdownCalled atomic.Bool
}

func (s *testShutdownService) OnShutdown(_ context.Context) error {
	s.shutdownCalled.Store(true)
	return nil
}

// TestShutdownWithContainer verifies container.Shutdown is called during app shutdown.
func TestShutdownWithContainer(t *testing.T) {
	c := NewContainer()
	svc := &testShutdownService{}
	if err := c.Give(svc); err != nil {
		t.Fatal(err)
	}

	app := New(WithContainer(c))

	ctx := context.Background()
	_ = app.Shutdown(ctx)

	if !svc.shutdownCalled.Load() {
		t.Fatal("expected container.Shutdown to call OnShutdown on registered services")
	}
}

// TestResolveInjectMiddlewarePriority verifies that InjectMiddleware container
// takes priority over app-level container in Resolve.
func TestResolveInjectMiddlewarePriority(t *testing.T) {
	appContainer := NewContainer()
	appSvc := &testDIService{Name: "app-level"}
	if err := appContainer.Give(appSvc); err != nil {
		t.Fatal(err)
	}

	mwContainer := NewContainer()
	mwSvc := &testDIService{Name: "middleware-level"}
	if err := mwContainer.Give(mwSvc); err != nil {
		t.Fatal(err)
	}

	app := New(WithContainer(appContainer))
	app.Use(mwContainer.InjectMiddleware())

	var resolved *testDIService
	app.Get("/test", func(ctx *Ctx) error {
		var err error
		resolved, err = Resolve[*testDIService](ctx)
		if err != nil {
			return err
		}
		return ctx.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	// InjectMiddleware container should take priority
	if resolved != mwSvc {
		t.Fatalf("expected middleware container service (Name=%q), got Name=%q", mwSvc.Name, resolved.Name)
	}
}
