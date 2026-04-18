package kruda

import (
	"strings"
	"testing"
)

// --- Compile: security headers ---

func TestCompile_SecurityHeaders_AllVariants(t *testing.T) {
	app := New(WithLegacySecurityHeaders())
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	if len(app.secHeaders) == 0 {
		t.Error("secHeaders should be populated after Compile")
	}
}

func TestCompile_SecurityHeaders_HSTS(t *testing.T) {
	app := &App{
		config:   defaultConfig(),
		router:   newRouter(),
		errorMap: defaultErrorMap(),
	}
	app.config.SecurityHeaders = true
	app.config.Security.HSTSMaxAge = 31536000
	app.config.Security.ContentSecurityPolicy = "default-src 'self'"
	app.config.Security.ReferrerPolicy = "no-referrer"
	app.transport, app.transportType = selectTransport(app.config, app.config.Logger)
	app.ctxPool.New = func() any { return newCtx(app) }

	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	found := false
	for _, kv := range app.secHeaders {
		if strings.Contains(kv[0], "Strict-Transport-Security") {
			found = true
		}
	}
	if !found {
		t.Error("HSTS header not found in secHeaders")
	}
}

func TestCompile_AllSecurityHeaders(t *testing.T) {
	app := New(func(a *App) {
		a.config.SecurityHeaders = true
		a.config.Security.XSSProtection = "1; mode=block"
		a.config.Security.ContentTypeNosniff = "nosniff"
		a.config.Security.XFrameOptions = "DENY"
		a.config.Security.ReferrerPolicy = "strict-origin"
		a.config.Security.ContentSecurityPolicy = "default-src 'self'"
		a.config.Security.HSTSMaxAge = 31536000
	})
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	if len(app.secHeaders) != 6 {
		t.Errorf("expected 6 security headers, got %d", len(app.secHeaders))
	}
}

// --- Compile: hasLifecycle flag ---

func TestCompile_HasLifecycle_False(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if app.hasLifecycle {
		t.Error("hasLifecycle should be false when no hooks registered")
	}
}

func TestCompile_HasLifecycle_True(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error { return nil })
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	if !app.hasLifecycle {
		t.Error("hasLifecycle should be true when hooks registered")
	}
}

func TestCompile_HasLifecycleFlag(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error { return nil })
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	if !app.hasLifecycle {
		t.Error("hasLifecycle should be true when OnRequest is registered")
	}
}

func TestCompile_NoLifecycle(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	if app.hasLifecycle {
		t.Error("hasLifecycle should be false when no hooks registered")
	}
}

// --- buildChain ---

func TestBuildChain(t *testing.T) {
	handler := func(c *Ctx) error { return nil }
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }

	chain := buildChain([]HandlerFunc{mw1}, []HandlerFunc{mw2}, handler)
	if len(chain) != 3 {
		t.Errorf("chain len = %d, want 3", len(chain))
	}
}

func TestBuildChain_NoMiddleware(t *testing.T) {
	handler := func(c *Ctx) error { return nil }
	chain := buildChain(nil, nil, handler)
	if len(chain) != 1 {
		t.Errorf("chain len = %d, want 1", len(chain))
	}
}

// --- containsDotPercent ---

func TestContainsDotPercent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/api/users", false},
		{"/api/v1.0/users", true},
		{"/api/%2e%2e/secret", true},
		{"", false},
		{".", true},
		{"%", true},
	}
	for _, tt := range tests {
		got := containsDotPercent(tt.input)
		if got != tt.want {
			t.Errorf("containsDotPercent(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestContainsDotPercent_Boost3(t *testing.T) {
	// Additional edge cases beyond the basic table above.
	tests := []struct {
		input string
		want  bool
	}{
		{"/users/42", false},
		{"/users/../admin", true},
		{"/files/test%2F", true},
		{"/.hidden", true},
		{"/a/b/c/d/e", false},
		{"/path.with.dots", true},
	}
	for _, tt := range tests {
		if got := containsDotPercent(tt.input); got != tt.want {
			t.Errorf("containsDotPercent(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
