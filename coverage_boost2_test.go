package kruda

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// =============================================================================
// config.go — WithTransport, Wing, WithHTTP3, selectTransport
// =============================================================================

func TestWithTransport_SetsTransport(t *testing.T) {
	custom := &mockTransport{}
	app := &App{config: defaultConfig()}
	WithTransport(custom)(app)
	if app.config.Transport != custom {
		t.Error("WithTransport did not set Transport")
	}
}

func TestWingOption_SetsTransportName(t *testing.T) {
	app := &App{config: defaultConfig()}
	Wing()(app)
	if app.config.TransportName != "wing" {
		t.Errorf("Wing() set TransportName = %q, want %q", app.config.TransportName, "wing")
	}
}

func TestWithHTTP3_SetsConfig(t *testing.T) {
	app := &App{config: defaultConfig()}
	WithHTTP3("cert.pem", "key.pem")(app)
	if app.config.TLSCertFile != "cert.pem" {
		t.Errorf("TLSCertFile = %q", app.config.TLSCertFile)
	}
	if app.config.TLSKeyFile != "key.pem" {
		t.Errorf("TLSKeyFile = %q", app.config.TLSKeyFile)
	}
	if !app.config.HTTP3 {
		t.Error("HTTP3 should be true")
	}
}

func TestSelectTransport_FastHTTPWithTLS(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "fasthttp"
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	if name != "nethttp" {
		t.Errorf("fasthttp+TLS should fallback to nethttp, got %q", name)
	}
}

func TestSelectTransport_WingWithTLS(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "wing"
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	if name != "nethttp" {
		t.Errorf("wing+TLS should fallback to nethttp, got %q", name)
	}
}

func TestSelectTransport_ExplicitTransportNameReturned(t *testing.T) {
	custom := &mockTransport{}
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.Transport = custom
	cfg.TransportName = "custom-name"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr != custom {
		t.Error("expected explicit transport")
	}
	if name != "custom-name" {
		t.Errorf("expected TransportName %q, got %q", "custom-name", name)
	}
}

// =============================================================================
// container.go — validateFactory, GiveTransient, GiveLazy, MustUse, MustUseNamed,
// ResolveNamed, MustResolveNamed
// =============================================================================

func TestValidateFactory_NilInput(t *testing.T) {
	_, err := validateFactory(nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
}

func TestValidateFactory_NonFunction(t *testing.T) {
	_, err := validateFactory("not a func")
	if err == nil {
		t.Error("expected error for non-function")
	}
}

func TestValidateFactory_FunctionWithArgs(t *testing.T) {
	_, err := validateFactory(func(x int) int { return x })
	if err == nil {
		t.Error("expected error for function with args")
	}
}

func TestValidateFactory_FunctionNoReturn(t *testing.T) {
	_, err := validateFactory(func() {})
	if err == nil {
		t.Error("expected error for function with no return")
	}
}

func TestValidateFactory_SecondReturnNotError(t *testing.T) {
	_, err := validateFactory(func() (int, string) { return 0, "" })
	if err == nil {
		t.Error("expected error when second return is not error")
	}
}

func TestValidateFactory_ValidSingleReturn(t *testing.T) {
	rt, err := validateFactory(func() int { return 42 })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.String() != "int" {
		t.Errorf("return type = %v, want int", rt)
	}
}

func TestValidateFactory_ValidWithError(t *testing.T) {
	rt, err := validateFactory(func() (string, error) { return "", nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt.String() != "string" {
		t.Errorf("return type = %v, want string", rt)
	}
}

func TestGiveTransient_InvalidFactory(t *testing.T) {
	c := NewContainer()
	err := c.GiveTransient("not a func")
	if err == nil {
		t.Error("expected error for invalid factory")
	}
}

func TestGiveTransient_Duplicate(t *testing.T) {
	c := NewContainer()
	err := c.GiveTransient(func() int { return 1 })
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	err = c.GiveTransient(func() int { return 2 })
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

func TestGiveLazy_InvalidFactory(t *testing.T) {
	c := NewContainer()
	err := c.GiveLazy("not a func")
	if err == nil {
		t.Error("expected error for invalid factory")
	}
}

func TestGiveLazy_Duplicate(t *testing.T) {
	c := NewContainer()
	err := c.GiveLazy(func() int { return 1 })
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}
	err = c.GiveLazy(func() int { return 2 })
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

func TestMustUse_Success(t *testing.T) {
	c := NewContainer()
	_ = c.Give("hello")
	got := MustUse[string](c)
	if got != "hello" {
		t.Errorf("MustUse = %q", got)
	}
}

func TestMustUseNamed_Success(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("greeting", "hi")
	got := MustUseNamed[string](c, "greeting")
	if got != "hi" {
		t.Errorf("MustUseNamed = %q", got)
	}
}

func TestResolveNamed_ViaHandler(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("db", "postgres://localhost")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val, err := ResolveNamed[string](ctx, "db")
		if err != nil {
			return BadRequest(err.Error())
		}
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "postgres://localhost") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestResolveNamed_NoContainer(t *testing.T) {
	app := New()
	app.Get("/test", func(ctx *Ctx) error {
		_, err := ResolveNamed[string](ctx, "db")
		if err == nil {
			return BadRequest("expected error")
		}
		return ctx.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestMustResolveNamed_ViaHandler(t *testing.T) {
	c := NewContainer()
	_ = c.GiveNamed("msg", "hello world")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val := MustResolveNamed[string](ctx, "msg")
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 200 {
		t.Errorf("status = %d", resp.StatusCode())
	}
	if !strings.Contains(resp.BodyString(), "hello world") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

func TestMustResolveNamed_Panics(t *testing.T) {
	c := NewContainer()
	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		defer func() {
			recover() // catch panic
		}()
		MustResolveNamed[string](ctx, "nonexistent")
		return ctx.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	// The panic should be caught internally by defer/recover
	tc.Get("/test")
}

// =============================================================================
// context.go — Cookie, IP, Route, Latency, SetHeaderBytes, AddHeader, JSON, isCookieSeparator
// =============================================================================

// cookieMockRequest is a mockRequest that returns cookies.
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

func TestCtx_IP_ReturnsRemoteAddr(t *testing.T) {
	app := New()
	app.Get("/ip", func(c *Ctx) error {
		return c.Text(c.IP())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/ip"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "127.0.0.1") {
		t.Errorf("IP = %q", resp.body)
	}
}

func TestCtx_IP_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	if c.IP() != "" {
		t.Errorf("IP with nil request should return empty, got %q", c.IP())
	}
}

func TestCtx_Route_WithPattern(t *testing.T) {
	app := New()
	app.Get("/users/:id", func(c *Ctx) error {
		return c.Text(c.Route())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/42"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "/users/:id") {
		t.Errorf("Route() = %q, want /users/:id", resp.body)
	}
}

func TestCtx_Route_StaticPath(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.Text(c.Route())
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/health"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "/health") {
		t.Errorf("Route() = %q", resp.body)
	}
}

func TestCtx_Latency_WithMarkStart(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.MarkStart()
		// do a tiny amount of work
		_ = strings.Repeat("x", 100)
		lat := c.Latency()
		if lat == 0 {
			return BadRequest("latency should be > 0 after MarkStart")
		}
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode == 400 {
		t.Error("Latency() returned 0 after MarkStart()")
	}
}

func TestCtx_Latency_WithoutMarkStart(t *testing.T) {
	app := New()
	c := newCtx(app)
	if c.Latency() != 0 {
		t.Errorf("Latency without MarkStart should be 0, got %v", c.Latency())
	}
}

func TestCtx_Context_Default(t *testing.T) {
	app := New()
	c := newCtx(app)
	ctx := c.Context()
	if ctx == nil {
		t.Error("Context() should not return nil")
	}
}

func TestCtx_Context_WithSetContext(t *testing.T) {
	app := New()
	c := newCtx(app)
	custom := context.WithValue(context.Background(), "key", "val")
	c.SetContext(custom)
	if c.Context() != custom {
		t.Error("Context() should return the custom context set via SetContext")
	}
}

func TestCtx_SetHeaderBytes(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.SetHeaderBytes("X-Custom", []byte("byte-value"))
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("X-Custom")
	if got != "byte-value" {
		t.Errorf("SetHeaderBytes: header = %q, want %q", got, "byte-value")
	}
}

func TestCtx_AddHeader_MultiValue(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Vary", "Accept")
		c.AddHeader("Vary", "Accept-Encoding")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Vary")
	if !strings.Contains(got, "Accept") || !strings.Contains(got, "Accept-Encoding") {
		t.Errorf("AddHeader multi-value: %q", got)
	}
}

func TestCtx_AddHeader_CacheControl(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Cache-Control", "no-cache")
		c.AddHeader("Cache-Control", "no-store")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Cache-Control")
	if !strings.Contains(got, "no-cache") || !strings.Contains(got, "no-store") {
		t.Errorf("AddHeader Cache-Control: %q", got)
	}
}

func TestCtx_AddHeader_ContentType(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Content-Type", "text/plain")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	// Content-Type should be set (may be overridden by Text())
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

func TestCtx_AddHeader_Location(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("Location", "/new-path")
		return c.Status(302).Text("")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	got := resp.headers.Get("Location")
	if got != "/new-path" {
		t.Errorf("AddHeader Location: %q", got)
	}
}

func TestCtx_AddHeader_InvalidKey(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		// Invalid header key with space should be skipped
		c.AddHeader("Bad Key", "value")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

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

// =============================================================================
// group.go — HEAD auto-registration via GET
// =============================================================================

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

// =============================================================================
// health.go — discoverHealthCheckers with lazy + named
// =============================================================================

type namedHealthService struct {
	name string
}

func (s *namedHealthService) Check(_ context.Context) error { return nil }

func TestDiscoverHealthCheckers_WithLazySingleton(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (*healthyDB, error) {
		return &healthyDB{}, nil
	})
	// Resolve the lazy to make it "done"
	_, _ = Use[*healthyDB](c)

	checkers := discoverHealthCheckers(c)
	found := false
	for _, ch := range checkers {
		if ch.checker != nil {
			found = true
		}
	}
	if !found {
		t.Error("discoverHealthCheckers should find resolved lazy singletons")
	}
}

func TestDiscoverHealthCheckers_WithUnresolvedLazy(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (*healthyDB, error) {
		return &healthyDB{}, nil
	})
	// Don't resolve — lazy is not "done"

	checkers := discoverHealthCheckers(c)
	for _, ch := range checkers {
		if _, ok := ch.checker.(*healthyDB); ok {
			t.Error("discoverHealthCheckers should NOT find unresolved lazy singletons")
		}
	}
}

func TestDiscoverHealthCheckers_WithNamedInstance(t *testing.T) {
	c := NewContainer()
	svc := &namedHealthService{name: "primary-db"}
	_ = c.GiveNamed("primary-db", svc)

	checkers := discoverHealthCheckers(c)
	found := false
	for _, ch := range checkers {
		if ch.name == "primary-db" {
			found = true
		}
	}
	if !found {
		t.Error("discoverHealthCheckers should find named health checkers")
	}
}

func TestDiscoverHealthCheckers_NilContainer(t *testing.T) {
	checkers := discoverHealthCheckers(nil)
	if checkers != nil {
		t.Error("discoverHealthCheckers(nil) should return nil")
	}
}

func TestDiscoverHealthCheckers_DedupSameInstance(t *testing.T) {
	c := NewContainer()
	svc := &healthyDB{}
	_ = c.Give(svc)
	// Register the same instance under a name
	_ = c.GiveNamed("db", svc)

	checkers := discoverHealthCheckers(c)
	count := 0
	for _, ch := range checkers {
		if _, ok := ch.checker.(*healthyDB); ok {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected dedup, got %d checkers for same instance", count)
	}
}

func TestDiscoverHealthCheckers_NonHealthChecker(t *testing.T) {
	c := NewContainer()
	type plainService struct{ Name string }
	_ = c.Give(&plainService{Name: "plain"})

	checkers := discoverHealthCheckers(c)
	if len(checkers) != 0 {
		t.Errorf("expected 0 checkers for non-HealthChecker, got %d", len(checkers))
	}
}

// =============================================================================
// Container: isRegistered covers transient branch
// =============================================================================

func TestIsRegistered_TransientBranch(t *testing.T) {
	c := NewContainer()
	_ = c.GiveTransient(func() (int, error) { return 1, nil })
	// Now trying to register a singleton of the same type should fail
	err := c.Give(42)
	if err == nil {
		t.Error("expected duplicate registration error for int (transient already registered)")
	}
}

func TestIsRegistered_LazyBranch(t *testing.T) {
	c := NewContainer()
	_ = c.GiveLazy(func() (int, error) { return 1, nil })
	// Now trying to register a transient of the same type should fail
	err := c.GiveTransient(func() (int, error) { return 2, nil })
	if err == nil {
		t.Error("expected duplicate registration error for int (lazy already registered)")
	}
}

// =============================================================================
// Container: GiveLazy retry on error
// =============================================================================

func TestGiveLazy_RetryOnError(t *testing.T) {
	c := NewContainer()
	callCount := 0
	_ = c.GiveLazy(func() (string, error) {
		callCount++
		if callCount == 1 {
			return "", errors.New("temporary failure")
		}
		return "success", nil
	})

	// First call should fail
	_, err := Use[string](c)
	if err == nil {
		t.Error("expected error on first call")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call should succeed (retry)
	val, err := Use[string](c)
	if err != nil {
		t.Fatalf("expected success on retry, got: %v", err)
	}
	if val != "success" {
		t.Errorf("val = %q", val)
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

// =============================================================================
// Context: Latency with MarkStart via middleware
// =============================================================================

func TestCtx_Latency_ViaMockRequest(t *testing.T) {
	app := New()
	var latency time.Duration
	app.Get("/test", func(c *Ctx) error {
		c.MarkStart()
		time.Sleep(1 * time.Millisecond)
		latency = c.Latency()
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if latency == 0 {
		t.Error("Latency should be > 0 after MarkStart + sleep")
	}
}

// =============================================================================
// wing_types.go — WingPlaintext, WingJSON, WingQuery, WingRender, wingFeatherOpt
// =============================================================================

func TestWingFeatherOptions(t *testing.T) {
	// These just create RouteOption funcs — calling them should not panic.
	opts := []RouteOption{
		WingPlaintext(),
		WingJSON(),
		WingQuery(),
		WingRender(),
	}
	for i, opt := range opts {
		if opt == nil {
			t.Errorf("Wing feather option %d is nil", i)
		}
		// Apply to a routeConfig to exercise the code
		var rc routeConfig
		opt(&rc)
		if rc.wingFeather == nil {
			t.Errorf("Wing feather option %d did not set feather", i)
		}
	}
}

// =============================================================================
// config.go — parseSize error branches (GB, KB parse errors)
// =============================================================================

func TestParseSize_GBError(t *testing.T) {
	_, err := parseSize("abcGB")
	if err == nil {
		t.Error("expected error for invalid GB value")
	}
}

func TestParseSize_KBError(t *testing.T) {
	_, err := parseSize("xyzKB")
	if err == nil {
		t.Error("expected error for invalid KB value")
	}
}

func TestParseSize_MBError(t *testing.T) {
	_, err := parseSize("badMB")
	if err == nil {
		t.Error("expected error for invalid MB value")
	}
}

// =============================================================================
// context.go — File with net/http transport (using TestClient)
// =============================================================================

func TestCtx_File_NonNetHTTP(t *testing.T) {
	// File requires net/http transport; mock transport should error
	app := New()
	app.Get("/file", func(c *Ctx) error {
		return c.File("/tmp/nonexistent.txt")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/file"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	// Should return 500 because mock doesn't support File
	if resp.statusCode == 200 {
		t.Error("File with non-nethttp transport should fail")
	}
}

// =============================================================================
// context.go — JSON with various data types
// =============================================================================

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

// =============================================================================
// container.go — GiveAs error paths
// =============================================================================

func TestGiveAs_NilInstance(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(nil, (*testInterface)(nil))
	if err == nil {
		t.Error("expected error for nil instance")
	}
}

func TestGiveAs_NilIfacePtr(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(&testImpl{}, nil)
	if err == nil {
		t.Error("expected error for nil ifacePtr")
	}
}

func TestGiveAs_NonPointerToInterface(t *testing.T) {
	c := NewContainer()
	err := c.GiveAs(&testImpl{}, "not a pointer")
	if err == nil {
		t.Error("expected error for non-pointer ifacePtr")
	}
}

func TestGiveAs_NotImplementing(t *testing.T) {
	c := NewContainer()
	type notImpl struct{}
	err := c.GiveAs(&notImpl{}, (*testInterface)(nil))
	if err == nil {
		t.Error("expected error when instance doesn't implement interface")
	}
}

func TestGiveAs_Duplicate(t *testing.T) {
	c := NewContainer()
	_ = c.GiveAs(&testImpl{msg: "a"}, (*testInterface)(nil))
	err := c.GiveAs(&testImpl{msg: "b"}, (*testInterface)(nil))
	if err == nil {
		t.Error("expected duplicate registration error")
	}
}

// =============================================================================
// container.go — Transient factory error path
// =============================================================================

func TestGiveTransient_FactoryError(t *testing.T) {
	c := NewContainer()
	_ = c.GiveTransient(func() (string, error) {
		return "", errors.New("factory failed")
	})
	_, err := Use[string](c)
	if err == nil {
		t.Error("expected error from failed transient factory")
	}
}

// =============================================================================
// router.go — isQuantifier
// =============================================================================

func TestIsQuantifier(t *testing.T) {
	quantifiers := []byte{'+', '*', '?', '{'}
	for _, c := range quantifiers {
		if !isQuantifier(c) {
			t.Errorf("isQuantifier(%q) = false, want true", string(c))
		}
	}
	nonQuantifiers := []byte{'a', 'z', '0', '.', '-', '[', ')'}
	for _, c := range nonQuantifiers {
		if isQuantifier(c) {
			t.Errorf("isQuantifier(%q) = true, want false", string(c))
		}
	}
}

// =============================================================================
// kruda.go — addRoute with WingFeather option (exercises opts path)
// =============================================================================

func TestApp_AddRoute_WithWingOption(t *testing.T) {
	app := New()
	app.Get("/health", func(c *Ctx) error {
		return c.Text("ok")
	}, WingPlaintext())
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
	}, WingJSON())
	app.Compile()

	req := &mockRequest{method: "GET", path: "/api/data"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("status = %d", resp.statusCode)
	}
}

// =============================================================================
// container.go — MustResolve success path
// =============================================================================

func TestMustResolve_Success(t *testing.T) {
	c := NewContainer()
	_ = c.Give("hello")

	app := New(WithContainer(c))
	app.Get("/test", func(ctx *Ctx) error {
		val := MustResolve[string](ctx)
		return ctx.Text(val)
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if !strings.Contains(resp.BodyString(), "hello") {
		t.Errorf("body = %q", resp.BodyString())
	}
}

// =============================================================================
// context.go — writeHeadersGeneric edge cases
// =============================================================================

func TestCtx_MultipleHeaders(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.SetHeader("X-First", "one")
		c.SetHeader("X-Second", "two")
		c.SetHeader("Cache-Control", "no-cache")
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.headers.Get("X-First") != "one" {
		t.Errorf("X-First = %q", resp.headers.Get("X-First"))
	}
	if resp.headers.Get("X-Second") != "two" {
		t.Errorf("X-Second = %q", resp.headers.Get("X-Second"))
	}
}

// =============================================================================
// context.go — Header from request
// =============================================================================

func TestCtx_Header_FromRequest(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		return c.Text(c.Header("X-Custom"))
	})
	app.Compile()

	req := &mockRequest{
		method:  "GET",
		path:    "/test",
		headers: map[string]string{"X-Custom": "custom-val"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "custom-val") {
		t.Errorf("body = %q", resp.body)
	}
}

// =============================================================================
// context.go — isBodyTooLarge
// =============================================================================

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

// =============================================================================
// error.go — MapErrorType
// =============================================================================

type customTestError struct {
	msg string
}

func (e *customTestError) Error() string { return e.msg }

func TestMapErrorType_Basic(t *testing.T) {
	app := New()
	MapErrorType[*customTestError](app, 422, "validation failed")

	app.Get("/fail", func(c *Ctx) error {
		return &customTestError{msg: "bad input"}
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 422 {
		t.Errorf("status = %d, want 422", resp.statusCode)
	}
}

func TestMapErrorType_PanicOnBareError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MapErrorType[error] should panic")
		}
	}()
	app := New()
	MapErrorType[error](app, 500, "catch all")
}

func TestCtx_Header_CachedSecondCall(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		// First call fetches from request, second call uses cache
		v1 := c.Header("X-Custom")
		v2 := c.Header("X-Custom")
		if v1 != v2 {
			return BadRequest("header values differ")
		}
		return c.Text(v1)
	})
	app.Compile()

	req := &mockRequest{
		method:  "GET",
		path:    "/test",
		headers: map[string]string{"X-Custom": "cached"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if !strings.Contains(string(resp.body), "cached") {
		t.Errorf("body = %q", resp.body)
	}
}
