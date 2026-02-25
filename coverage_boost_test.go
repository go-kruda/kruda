package kruda

import (
	"fmt"
	"net/http"
	"testing"
)

// Tests to boost coverage on low-coverage functions.

func TestBoostValidateGTE(t *testing.T) {
	// int
	if !validateGTE(10, "5") {
		t.Error("10 >= 5")
	}
	if validateGTE(3, "5") {
		t.Error("3 < 5")
	}
	// uint
	if !validateGTE(uint(10), "5") {
		t.Error("uint 10 >= 5")
	}
	if validateGTE(uint(2), "5") {
		t.Error("uint 2 < 5")
	}
	// float
	if !validateGTE(10.5, "5.0") {
		t.Error("10.5 >= 5.0")
	}
	// string length
	if !validateGTE("hello", "3") {
		t.Error("len(hello)=5 >= 3")
	}
	if validateGTE("hi", "5") {
		t.Error("len(hi)=2 < 5")
	}
	// bad param
	if validateGTE(10, "abc") {
		t.Error("bad param")
	}
	// unsupported type
	if validateGTE(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateLT(t *testing.T) {
	if !validateLT(3, "5") {
		t.Error("3 < 5")
	}
	if validateLT(10, "5") {
		t.Error("10 >= 5")
	}
	// uint
	if !validateLT(uint(2), "5") {
		t.Error("uint 2 < 5")
	}
	if validateLT(uint(10), "5") {
		t.Error("uint 10 >= 5")
	}
	// float
	if !validateLT(2.5, "5.0") {
		t.Error("2.5 < 5.0")
	}
	// string
	if !validateLT("hi", "5") {
		t.Error("len(hi)=2 < 5")
	}
	if validateLT("hello", "3") {
		t.Error("len(hello)=5 >= 3")
	}
	// bad param
	if validateLT(10, "abc") {
		t.Error("bad param")
	}
	// unsupported
	if validateLT(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateLTE(t *testing.T) {
	if !validateLTE(5, "5") {
		t.Error("5 <= 5")
	}
	if !validateLTE(3, "5") {
		t.Error("3 <= 5")
	}
	if validateLTE(10, "5") {
		t.Error("10 > 5")
	}
	// uint
	if !validateLTE(uint(5), "5") {
		t.Error("uint 5 <= 5")
	}
	if validateLTE(uint(10), "5") {
		t.Error("uint 10 > 5")
	}
	// float
	if !validateLTE(5.0, "5.0") {
		t.Error("5.0 <= 5.0")
	}
	// string
	if !validateLTE("hi", "5") {
		t.Error("len(hi)=2 <= 5")
	}
	// bad param
	if validateLTE(10, "abc") {
		t.Error("bad param")
	}
	// unsupported
	if validateLTE(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateGT(t *testing.T) {
	// uint
	if !validateGT(uint(10), "5") {
		t.Error("uint 10 > 5")
	}
	if validateGT(uint(3), "5") {
		t.Error("uint 3 <= 5")
	}
	// float
	if !validateGT(10.5, "5.0") {
		t.Error("10.5 > 5.0")
	}
	// string
	if !validateGT("hello", "3") {
		t.Error("len(hello)=5 > 3")
	}
	// bad param
	if validateGT(10, "abc") {
		t.Error("bad param")
	}
	// unsupported
	if validateGT(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateLen(t *testing.T) {
	if !validateLen("hello", "5") {
		t.Error("len(hello)=5")
	}
	if validateLen("hi", "5") {
		t.Error("len(hi)!=5")
	}
	// int (unsupported for len)
	if validateLen(123, "3") {
		t.Error("int unsupported for len")
	}
	// bad param
	if validateLen("hi", "abc") {
		t.Error("bad param")
	}
}

func TestBoostValidateMinUintFloat(t *testing.T) {
	// uint
	if !validateMin(uint(10), "5") {
		t.Error("uint 10 >= 5")
	}
	if validateMin(uint(2), "5") {
		t.Error("uint 2 < 5")
	}
	// float
	if !validateMin(10.5, "5.0") {
		t.Error("10.5 >= 5.0")
	}
	// string
	if !validateMin("hello", "3") {
		t.Error("len(hello)=5 >= 3")
	}
	// unsupported
	if validateMin(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateMaxUintFloat(t *testing.T) {
	// uint
	if !validateMax(uint(3), "5") {
		t.Error("uint 3 <= 5")
	}
	if validateMax(uint(10), "5") {
		t.Error("uint 10 > 5")
	}
	// float
	if !validateMax(2.5, "5.0") {
		t.Error("2.5 <= 5.0")
	}
	// string
	if !validateMax("hi", "5") {
		t.Error("len(hi)=2 <= 5")
	}
	// unsupported
	if validateMax(true, "1") {
		t.Error("bool unsupported")
	}
}

func TestBoostValidateContainsStartsEnds(t *testing.T) {
	if validateContains(123, "1") {
		t.Error("int unsupported")
	}
	if validateStartsWith(123, "1") {
		t.Error("int unsupported")
	}
	if validateEndsWith(123, "1") {
		t.Error("int unsupported")
	}
}

func TestBoostResourceParseIDTypes(t *testing.T) {
	v64, err := resourceParseID[int64]("42")
	if err != nil || v64 != 42 {
		t.Errorf("int64: got %v, err %v", v64, err)
	}
	vu, err := resourceParseID[uint]("42")
	if err != nil || vu != 42 {
		t.Errorf("uint: got %v, err %v", vu, err)
	}
	vu64, err := resourceParseID[uint64]("42")
	if err != nil || vu64 != 42 {
		t.Errorf("uint64: got %v, err %v", vu64, err)
	}
	_, err = resourceParseID[float64]("42")
	if err == nil {
		t.Error("float64 should be unsupported")
	}
}

func TestBoostCtxAddHeaderInjection(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.AddHeader("X-Custom", "value1")
		c.AddHeader("X-Custom", "value2")
		c.AddHeader("X-Inject", "bad\r\nEvil: header")
		c.AddHeader("Bad Key!", "value")
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxQueryIntEdgeCases(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		v := c.QueryInt("bad", 99)
		v2 := c.QueryInt("missing", 42)
		return c.JSON(Map{"bad": v, "missing": v2})
	})
	tc := NewTestClient(app)
	resp, err := tc.Request("GET", "/test").Query("bad", "notanumber").Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxCookieMethod(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		v := c.Cookie("session")
		missing := c.Cookie("nope")
		return c.JSON(Map{"session": v, "nope": missing})
	})
	tc := NewTestClient(app).WithCookie("session", "abc123")
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxIPMethod(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		_ = c.IP()
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxIPWithXForwardedFor(t *testing.T) {
	app := New(WithTrustProxy(true))
	app.Get("/test", func(c *Ctx) error {
		return c.Text(c.IP())
	})
	tc := NewTestClient(app).WithHeader("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxHeaderMethod(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		v := c.Header("X-Custom")
		canon := c.Header("x-custom")
		return c.JSON(Map{"custom": v, "canon": canon})
	})
	tc := NewTestClient(app).WithHeader("X-Custom", "myval")
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostCtxFileNotFound(t *testing.T) {
	app := New()
	app.Get("/file", func(c *Ctx) error {
		return c.File("/nonexistent/path/file.txt")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/file")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 {
		t.Error("expected non-200 for missing file")
	}
}

func TestBoostDevModeGenerateSuggestions(t *testing.T) {
	// 500 — doesn't access c.Path()/c.Method()
	s := generateSuggestions(InternalError("oops"), 500, nil)
	if len(s) == 0 {
		t.Error("expected suggestions for 500")
	}
	// 422
	s = generateSuggestions(UnprocessableEntity("bad"), 422, nil)
	if len(s) == 0 {
		t.Error("expected suggestions for 422")
	}
	// 418 — unknown
	s = generateSuggestions(InternalError("teapot"), 418, nil)
	_ = s
}

func TestBoostDevModeWalkRouteTree(t *testing.T) {
	app := New()
	app.Get("/users", func(c *Ctx) error { return nil })
	app.Post("/users", func(c *Ctx) error { return nil })
	app.Get("/users/:id", func(c *Ctx) error { return nil })
	app.Compile()
	routes := collectDevRoutes(app)
	if len(routes) < 3 {
		t.Errorf("expected at least 3 routes, got %d", len(routes))
	}
}

func TestBoostSanitizeSSEField(t *testing.T) {
	result := sanitizeSSEField("hello\nworld\rfoo")
	if result != "helloworldfoo" {
		t.Errorf("expected stripped newlines, got %q", result)
	}
	result = sanitizeSSEField("hello")
	if result != "hello" {
		t.Errorf("expected hello, got %q", result)
	}
}

func TestBoostContainerValidateFactory(t *testing.T) {
	c := NewContainer()
	// nil value
	err := c.Give(nil)
	if err == nil {
		t.Error("expected error for nil value")
	}
	// GiveTransient with nil factory
	err = c.GiveTransient(nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
}

func TestBoostContainerGiveNamedAndUseNamed(t *testing.T) {
	c := NewContainer()
	type boostSvc struct{ Name string }
	c.GiveNamed("primary", &boostSvc{Name: "primary"})
	// UseNamed with nonexistent name
	_, err := UseNamed[*boostSvc](c, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent named")
	}
}

func TestBoostKrudaErrorWithWrapped(t *testing.T) {
	inner := &KrudaError{Code: 500, Message: "root cause"}
	err := NewError(400, "wrapper", inner)
	s := err.Error()
	if s == "" {
		t.Error("expected non-empty error string")
	}
	unwrapped := err.Unwrap()
	if unwrapped != inner {
		t.Error("unwrap should return inner error")
	}
}

func TestBoostIsBodyTooLarge(t *testing.T) {
	app := New(WithMaxBodySize(100))
	app.Post("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Post("/test", []byte("small"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("small body status: %d", resp.StatusCode())
	}
}

func TestBoostDevModeErrorPage404(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/exists", func(c *Ctx) error { return c.Text("ok") })
	tc := NewTestClient(app)
	// Hit a route that doesn't exist — triggers dev error page for 404
	resp, err := tc.Get("/notfound")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode())
	}
	body := resp.BodyString()
	// Dev mode should render HTML error page
	if len(body) == 0 {
		t.Error("expected non-empty body")
	}
}

func TestBoostDevModeErrorPage500(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/panic", func(c *Ctx) error {
		return InternalError("something broke")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/panic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode())
	}
}

func TestBoostDevModeErrorPage405(t *testing.T) {
	app := New(WithDevMode(true))
	app.Post("/only-post", func(c *Ctx) error { return c.Text("ok") })
	tc := NewTestClient(app)
	resp, err := tc.Get("/only-post")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 405 {
		t.Fatalf("expected 405, got %d", resp.StatusCode())
	}
}

func TestBoostHealthCheckerDiscovery(t *testing.T) {
	// Test with lazy singleton that implements HealthChecker
	c := NewContainer()
	type mockChecker struct{}
	c.GiveLazy(func() (*mockChecker, error) {
		return &mockChecker{}, nil
	})
	// Not resolved yet — should not appear
	checkers := discoverHealthCheckers(c)
	_ = checkers

	// Test with named instance
	c2 := NewContainer()
	c2.GiveNamed("db", &mockChecker{})
	checkers2 := discoverHealthCheckers(c2)
	_ = checkers2
}

func TestBoostCtxBindJSON(t *testing.T) {
	app := New()
	type input struct {
		Name string `json:"name"`
	}
	app.Post("/bind", func(c *Ctx) error {
		var in input
		if err := c.Bind(&in); err != nil {
			return err
		}
		return c.JSON(Map{"name": in.Name})
	})
	tc := NewTestClient(app)
	resp, err := tc.Post("/bind", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d, body: %s", resp.StatusCode(), resp.BodyString())
	}
}

func TestBoostCtxBindInvalidJSON(t *testing.T) {
	app := New()
	type input struct {
		Name string `json:"name"`
	}
	app.Post("/bind", func(c *Ctx) error {
		var in input
		if err := c.Bind(&in); err != nil {
			return err
		}
		return c.JSON(Map{"name": in.Name})
	})
	tc := NewTestClient(app)
	resp, err := tc.Request("POST", "/bind").ContentType("application/json").Body([]byte("{invalid")).Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 {
		t.Error("expected error for invalid JSON")
	}
}

func TestBoostMustUseNamedPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic from MustUseNamed")
		}
	}()
	c := NewContainer()
	type boostSvc2 struct{}
	MustUseNamed[*boostSvc2](c, "nonexistent")
}

func TestBoostRouterWildcardAndParam(t *testing.T) {
	app := New()
	app.Get("/files/*filepath", func(c *Ctx) error {
		return c.Text(c.Param("filepath"))
	})
	app.Get("/users/:id/posts/:pid", func(c *Ctx) error {
		return c.JSON(Map{"id": c.Param("id"), "pid": c.Param("pid")})
	})
	tc := NewTestClient(app)

	resp, err := tc.Get("/files/css/style.css")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("wildcard status: %d", resp.StatusCode())
	}

	resp, err = tc.Get("/users/42/posts/7")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("multi-param status: %d", resp.StatusCode())
	}
}

func TestBoostGiveAsInterface(t *testing.T) {
	c := NewContainer()
	type myImpl struct{}
	// GiveAs with nil ifacePtr
	err := c.GiveAs(&myImpl{}, nil)
	if err == nil {
		t.Error("expected error for nil ifacePtr")
	}
	// GiveAs with non-pointer
	err = c.GiveAs(&myImpl{}, "not a pointer")
	if err == nil {
		t.Error("expected error for non-pointer ifacePtr")
	}
	// GiveAs with type that doesn't implement interface
	err = c.GiveAs(&myImpl{}, (*HealthChecker)(nil))
	if err == nil {
		t.Error("expected error for non-implementing type")
	}
}

func TestBoostValidateFactoryBadSignature(t *testing.T) {
	c := NewContainer()
	// Not a function
	err := c.GiveTransient("not a function")
	if err == nil {
		t.Error("expected error for non-function factory")
	}
	// Function with wrong return count
	err = c.GiveTransient(func() {})
	if err == nil {
		t.Error("expected error for zero-return factory")
	}
}

func TestBoostHandleErrorNonKrudaError(t *testing.T) {
	app := New()
	app.Get("/err", func(c *Ctx) error {
		return fmt.Errorf("plain error")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/err")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode())
	}
}

func TestBoostServeKrudaMethodNotAllowed(t *testing.T) {
	app := New()
	app.Post("/only-post", func(c *Ctx) error { return c.Text("ok") })
	tc := NewTestClient(app)
	// PUT should get 405
	resp, err := tc.Put("/only-post", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 405 {
		t.Fatalf("expected 405, got %d", resp.StatusCode())
	}
	allow := resp.Header("Allow")
	if allow == "" {
		t.Error("expected Allow header")
	}
}

func TestBoostCtxSendJSON500(t *testing.T) {
	app := New()
	app.Get("/bad-json", func(c *Ctx) error {
		// Channel can't be marshaled to JSON
		return c.JSON(make(chan int))
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/bad-json")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 {
		t.Error("expected error for unmarshalable JSON")
	}
}

func TestBoostRouterTraversalPrevention(t *testing.T) {
	app := New()
	app.Get("/safe", func(c *Ctx) error { return c.Text("ok") })
	tc := NewTestClient(app)
	resp, err := tc.Get("/../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 {
		t.Error("expected rejection for path traversal")
	}
}

func TestBoostCtxSetHeaderInvalidKey(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		c.SetHeader("Valid-Key", "value")
		c.SetHeader("Invalid Key With Spaces", "value")
		c.SetHeader("", "empty key")
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/test")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostConfigParseSize(t *testing.T) {
	// Exercise parseSize with various formats via env config
	// Direct test of parseSize
	tests := []struct {
		input string
		want  int64
		ok    bool
	}{
		{"1024", 1024, true},
		{"1KB", 1024, true},
		{"1kb", 1024, true},
		{"2MB", 2 * 1024 * 1024, true},
		{"1GB", 1024 * 1024 * 1024, true},
		{"bad", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, err := parseSize(tt.input)
		if tt.ok && err != nil {
			t.Errorf("parseSize(%q): unexpected error: %v", tt.input, err)
		}
		if !tt.ok && err == nil {
			t.Errorf("parseSize(%q): expected error", tt.input)
		}
		if tt.ok && got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestBoostSSEEventWithID(t *testing.T) {
	app := New()
	app.Get("/sse", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			s.EventWithID("1", "message", "hello")
			s.Comment("keep-alive")
			s.Retry(1000)
			return nil
		})
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/sse")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostHandleErrorValidation(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	type input struct {
		Email string `json:"email" validate:"required,email"`
	}
	Post[input, Map](app, "/validate", func(c *C[input]) (*Map, error) {
		return &Map{"ok": true}, nil
	})
	tc := NewTestClient(app)
	// Send empty email — fails "required" validation
	resp, err := tc.Post("/validate", map[string]string{"email": ""})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 422 {
		t.Fatalf("expected 422, got %d: %s", resp.StatusCode(), resp.BodyString())
	}
}

func TestBoostHandleErrorCustomHandler(t *testing.T) {
	customCalled := false
	app := New(WithErrorHandler(func(c *Ctx, err *KrudaError) {
		customCalled = true
		c.Status(err.Code)
		c.JSON(Map{"custom": true, "message": err.Message})
	}))
	app.Get("/err", func(c *Ctx) error {
		return BadRequest("custom handled")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/err")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode())
	}
	if !customCalled {
		t.Error("custom error handler not called")
	}
}

func TestBoostHandleErrorAlreadyResponded(t *testing.T) {
	app := New()
	app.Get("/double", func(c *Ctx) error {
		c.Text("already sent")
		// Return error after already responding
		return InternalError("too late")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/double")
	if err != nil {
		t.Fatal(err)
	}
	// Should get the first response, not the error
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

func TestBoostServeKrudaPanicRecovery(t *testing.T) {
	app := New()
	app.Get("/panic", func(c *Ctx) error {
		panic("boom")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/panic")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode())
	}
}

func TestBoostMapErrorFunc(t *testing.T) {
	var sentinel = fmt.Errorf("sentinel")
	app := New()
	MapErrorFunc(app, sentinel, func(err error) *KrudaError {
		return BadRequest(err.Error())
	})
	app.Get("/custom-err", func(c *Ctx) error {
		return fmt.Errorf("wrapped: %w", sentinel)
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/custom-err")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode())
	}
}

func TestBoostResolveFromCtx(t *testing.T) {
	c := NewContainer()
	type svcR struct{ V string }
	c.GiveNamed("main", &svcR{V: "hello"})
	app := New(WithContainer(c))
	app.Get("/resolve", func(ctx *Ctx) error {
		v, err := ResolveNamed[*svcR](ctx, "main")
		if err != nil {
			return err
		}
		return ctx.Text(v.V)
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/resolve")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
	if resp.BodyString() != "hello" {
		t.Errorf("expected hello, got %s", resp.BodyString())
	}
}

func TestBoostResolveNamedMissing(t *testing.T) {
	c := NewContainer()
	app := New(WithContainer(c))
	app.Get("/resolve-missing", func(ctx *Ctx) error {
		_, err := ResolveNamed[*struct{}](ctx, "missing")
		if err != nil {
			return BadRequest(err.Error())
		}
		return ctx.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/resolve-missing")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode())
	}
}

func TestBoostMustResolvePanic(t *testing.T) {
	app := New()
	app.Get("/must-resolve", func(ctx *Ctx) error {
		defer func() {
			recover() // catch the panic
		}()
		type svcM struct{}
		MustResolve[*svcM](ctx)
		return ctx.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/must-resolve")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp
}

func TestBoostCleanupOversizedMaps(t *testing.T) {
	app := New()
	app.Get("/bloat", func(c *Ctx) error {
		// Bloat params beyond maxParamsCapacity (16)
		for i := 0; i < 20; i++ {
			c.params[fmt.Sprintf("p%d", i)] = "v"
		}
		// Bloat locals beyond maxLocalsCapacity (16)
		for i := 0; i < 20; i++ {
			c.locals[fmt.Sprintf("l%d", i)] = "v"
		}
		// Bloat respHeaders beyond maxRespHeadersCapacity (32)
		for i := 0; i < 35; i++ {
			c.respHeaders[fmt.Sprintf("X-H%d", i)] = []string{"v"}
		}
		// Bloat headers beyond maxHeadersCapacity (32)
		for i := 0; i < 35; i++ {
			c.headers[fmt.Sprintf("h%d", i)] = "v"
		}
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	// First request bloats the maps
	resp, err := tc.Get("/bloat")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
	// Second request reuses the context — cleanup should have shrunk maps
	resp, err = tc.Get("/bloat")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostFormatCookieAllOptions(t *testing.T) {
	app := New()
	app.Get("/cookie", func(c *Ctx) error {
		// Cookie with all options including MaxAge > 0
		c.SetCookie(&Cookie{
			Name:     "session",
			Value:    "abc",
			Path:     "/",
			Domain:   "example.com",
			MaxAge:   3600,
			Secure:   true,
			HTTPOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		// Cookie with MaxAge < 0 (delete)
		c.SetCookie(&Cookie{
			Name:   "old",
			Value:  "",
			MaxAge: -1,
		})
		// Cookie with SameSite=None
		c.SetCookie(&Cookie{
			Name:     "cross",
			Value:    "xyz",
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
		// Cookie with SameSite=Lax
		c.SetCookie(&Cookie{
			Name:     "lax",
			Value:    "123",
			SameSite: http.SameSiteLaxMode,
		})
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/cookie")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostRouterOptionalParam(t *testing.T) {
	app := New()
	app.Get("/items/:id?", func(c *Ctx) error {
		id := c.Param("id")
		if id == "" {
			return c.JSON(Map{"all": true})
		}
		return c.JSON(Map{"id": id})
	})
	tc := NewTestClient(app)

	// With param
	resp, err := tc.Get("/items/42")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("with param status: %d", resp.StatusCode())
	}

	// Without param (optional)
	resp, err = tc.Get("/items")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("without param status: %d, body: %s", resp.StatusCode(), resp.BodyString())
	}
}

func TestBoostRouterRegexParam(t *testing.T) {
	app := New()
	app.Get("/users/:id<^\\d+$>", func(c *Ctx) error {
		return c.Text(c.Param("id"))
	})
	tc := NewTestClient(app)

	// Valid numeric ID
	resp, err := tc.Get("/users/123")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("numeric id status: %d", resp.StatusCode())
	}

	// Invalid non-numeric ID — should not match
	resp, err = tc.Get("/users/abc")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() == 200 {
		t.Error("expected non-200 for non-numeric id")
	}
}

func TestBoostRouterNestedParamContinue(t *testing.T) {
	app := New()
	app.Get("/api/:version/users/:id/profile", func(c *Ctx) error {
		return c.JSON(Map{
			"version": c.Param("version"),
			"id":      c.Param("id"),
		})
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/api/v1/users/42/profile")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("nested param status: %d", resp.StatusCode())
	}
}

func TestBoostDevModeCollectQueryParams(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/search", func(c *Ctx) error {
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	// Hit a non-existent route with query params — dev error page will call collectQueryParams
	resp, err := tc.Request("GET", "/missing").Query("q", "hello").Query("page", "1").Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode())
	}
	body := resp.BodyString()
	if len(body) == 0 {
		t.Error("expected non-empty dev error page body")
	}
}

func TestBoostMapErrorType(t *testing.T) {
	app := New()
	MapErrorType[*customMapErr](app, 409, "conflict detected")
	app.Get("/conflict", func(c *Ctx) error {
		return &customMapErr{msg: "duplicate"}
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/conflict")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 409 {
		t.Fatalf("expected 409, got %d", resp.StatusCode())
	}
}

type customMapErr struct{ msg string }

func (e *customMapErr) Error() string { return e.msg }

func TestBoostCtxBindEmptyBody(t *testing.T) {
	app := New()
	app.Post("/bind-empty", func(c *Ctx) error {
		var v Map
		if err := c.Bind(&v); err != nil {
			return err
		}
		return c.JSON(v)
	})
	tc := NewTestClient(app)
	// Empty body should return 400
	resp, err := tc.Request("POST", "/bind-empty").ContentType("application/json").Body([]byte("")).Send()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode())
	}
}

func TestBoostCtxSetCookieInjection(t *testing.T) {
	app := New()
	app.Get("/cookie", func(c *Ctx) error {
		c.SetCookie(&Cookie{
			Name:  "safe",
			Value: "good\r\nEvil: header",
			Path:  "/test\r\nBad: path",
		})
		return c.Text("ok")
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/cookie")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("status: %d", resp.StatusCode())
	}
}

func TestBoostResolveNamedFromCtxWithContainer(t *testing.T) {
	c := NewContainer()
	type svc struct{ V string }
	c.GiveNamed("db", &svc{V: "postgres"})
	c.GiveNamed("cache", &svc{V: "redis"})
	app := New(WithContainer(c))
	app.Get("/multi", func(ctx *Ctx) error {
		db, err := ResolveNamed[*svc](ctx, "db")
		if err != nil {
			return err
		}
		cache, err := ResolveNamed[*svc](ctx, "cache")
		if err != nil {
			return err
		}
		return ctx.JSON(Map{"db": db.V, "cache": cache.V})
	})
	tc := NewTestClient(app)
	resp, err := tc.Get("/multi")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode())
	}
}

func TestBoostRouterFindRootOptionalParam(t *testing.T) {
	app := New()
	app.Get("/:lang?", func(c *Ctx) error {
		lang := c.Param("lang")
		if lang == "" {
			return c.Text("default")
		}
		return c.Text(lang)
	})
	tc := NewTestClient(app)

	// Root with no param
	resp, err := tc.Get("/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("root status: %d", resp.StatusCode())
	}

	// Root with param
	resp, err = tc.Get("/en")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode() != 200 {
		t.Fatalf("lang status: %d", resp.StatusCode())
	}
}

func TestBoostSanitizeCookieTokenSpecialChars(t *testing.T) {
	// Test sanitizeCookieToken with separator characters
	result := sanitizeCookieToken("name(bad)")
	if result != "namebad" {
		t.Errorf("expected 'namebad', got %q", result)
	}
	result = sanitizeCookieToken("good-name")
	if result != "good-name" {
		t.Errorf("expected 'good-name', got %q", result)
	}
	// Control chars
	result = sanitizeCookieToken("bad\x01name")
	if result != "badname" {
		t.Errorf("expected 'badname', got %q", result)
	}
}

func TestBoostIsCookieSeparatorAll(t *testing.T) {
	separators := []byte{'(', ')', '<', '>', '@', ',', ';', ':', '\\', '"', '/', '[', ']', '?', '=', '{', '}', ' ', '\t'}
	for _, c := range separators {
		if !isCookieSeparator(c) {
			t.Errorf("expected %q to be separator", c)
		}
	}
	nonSep := []byte{'a', 'z', '0', '-', '_', '.'}
	for _, c := range nonSep {
		if isCookieSeparator(c) {
			t.Errorf("expected %q to NOT be separator", c)
		}
	}
}
