package kruda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda/transport"
)

// mockRequest implements transport.Request.
type mockRequest struct {
	method  string
	path    string
	headers map[string]string
	body    []byte
	query   map[string]string
}

func (r *mockRequest) Method() string           { return r.method }
func (r *mockRequest) Path() string             { return r.path }
func (r *mockRequest) Header(key string) string { return r.headers[key] }
func (r *mockRequest) Body() ([]byte, error)    { return r.body, nil }
func (r *mockRequest) QueryParam(key string) string {
	if r.query != nil {
		return r.query[key]
	}
	return ""
}
func (r *mockRequest) RemoteAddr() string        { return "127.0.0.1" }
func (r *mockRequest) Cookie(name string) string { return "" }
func (r *mockRequest) RawRequest() any           { return nil }
func (r *mockRequest) Context() context.Context  { return context.Background() }

func (r *mockRequest) MultipartForm(int64) (*multipart.Form, error) {
	return nil, fmt.Errorf("not supported")
}

// mockResponseWriter implements transport.ResponseWriter.
type mockResponseWriter struct {
	statusCode int
	headers    mockHeaderMap
	body       []byte
}

func newMockResponse() *mockResponseWriter {
	return &mockResponseWriter{headers: mockHeaderMap{h: make(map[string]string)}}
}

func (w *mockResponseWriter) WriteHeader(code int)        { w.statusCode = code }
func (w *mockResponseWriter) Header() transport.HeaderMap { return &w.headers }
func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

// mockHeaderMap implements transport.HeaderMap.
type mockHeaderMap struct {
	h map[string]string
}

func (m *mockHeaderMap) Set(key, value string) { m.h[key] = value }
func (m *mockHeaderMap) Get(key string) string { return m.h[key] }
func (m *mockHeaderMap) Add(key, value string) {
	if existing := m.h[key]; existing != "" {
		m.h[key] = existing + ", " + value
	} else {
		m.h[key] = value
	}
}
func (m *mockHeaderMap) Del(key string) { delete(m.h, key) }

func TestNew_Defaults(t *testing.T) {
	app := New()

	if app == nil {
		t.Fatal("New() returned nil")
	}
	if app.router == nil {
		t.Error("router should not be nil")
	}
	if app.transport == nil {
		t.Error("transport should not be nil")
	}
	if app.errorMap == nil {
		t.Error("errorMap should not be nil")
	}

	// Default config values
	if app.config.ReadTimeout != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", app.config.ReadTimeout)
	}
	if app.config.WriteTimeout != 30*time.Second {
		t.Errorf("WriteTimeout = %v, want 30s", app.config.WriteTimeout)
	}
	if app.config.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %v, want 120s", app.config.IdleTimeout)
	}
	if app.config.BodyLimit != 4*1024*1024 {
		t.Errorf("BodyLimit = %d, want 4MB", app.config.BodyLimit)
	}
	if app.config.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 10s", app.config.ShutdownTimeout)
	}
	if app.config.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if app.config.JSONEncoder == nil {
		t.Error("JSONEncoder should not be nil")
	}
	if app.config.JSONDecoder == nil {
		t.Error("JSONDecoder should not be nil")
	}
}

func TestNew_WithOptions(t *testing.T) {
	app := New(
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(10*time.Second),
		WithBodyLimit(1024),
		WithShutdownTimeout(30*time.Second),
	)

	if app.config.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", app.config.ReadTimeout)
	}
	if app.config.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", app.config.WriteTimeout)
	}
	if app.config.BodyLimit != 1024 {
		t.Errorf("BodyLimit = %d, want 1024", app.config.BodyLimit)
	}
	if app.config.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 30s", app.config.ShutdownTimeout)
	}
}

func TestApp_RouteRegistration(t *testing.T) {
	app := New()
	h := func(c *Ctx) error { return nil }

	app.Get("/get", h)
	app.Post("/post", h)
	app.Put("/put", h)
	app.Delete("/delete", h)
	app.Patch("/patch", h)

	var params routeParams
	methods := map[string]string{
		"GET":    "/get",
		"POST":   "/post",
		"PUT":    "/put",
		"DELETE": "/delete",
		"PATCH":  "/patch",
	}
	for method, path := range methods {
		params.reset()
		if app.router.find(method, path, &params) == nil {
			t.Errorf("%s %s should be registered", method, path)
		}
	}
}

func TestApp_Use(t *testing.T) {
	app := New()
	mw1 := func(c *Ctx) error { return c.Next() }
	mw2 := func(c *Ctx) error { return c.Next() }

	app.Use(mw1, mw2)

	if len(app.middleware) != 2 {
		t.Fatalf("middleware count = %d, want 2", len(app.middleware))
	}
}

func TestApp_MethodChaining(t *testing.T) {
	app := New()
	h := func(c *Ctx) error { return nil }

	ret := app.Get("/a", h).Post("/b", h).Put("/c", h).Delete("/d", h).Patch("/e", h)
	if ret != app {
		t.Error("chained route methods should return the same *App")
	}
}

func TestApp_ServeKruda_Success(t *testing.T) {
	app := New()

	called := false
	app.Get("/hello", func(c *Ctx) error {
		called = true
		return c.JSON(Map{"msg": "ok"})
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/hello"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if !called {
		t.Error("handler should have been called")
	}
	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}
	var body map[string]any
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if body["msg"] != "ok" {
		t.Errorf("body[msg] = %v, want ok", body["msg"])
	}
}

func TestApp_ServeKruda_404(t *testing.T) {
	app := New()
	app.Get("/exists", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/nope"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 404 {
		t.Errorf("status = %d, want 404", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 404 {
		t.Errorf("body.Code = %d, want 404", body.Code)
	}
}

func TestApp_ServeKruda_405(t *testing.T) {
	app := New()
	app.Get("/resource", func(c *Ctx) error { return c.Text("ok") })
	app.router.Compile()

	req := &mockRequest{method: "POST", path: "/resource"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 405 {
		t.Errorf("status = %d, want 405", resp.statusCode)
	}

	allow := resp.headers.h["Allow"]
	if !strings.Contains(allow, "GET") {
		t.Errorf("Allow header = %q, want to contain GET", allow)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 405 {
		t.Errorf("body.Code = %d, want 405", body.Code)
	}
}

func TestApp_HandleError(t *testing.T) {
	app := New()

	app.Get("/fail", func(c *Ctx) error {
		return BadRequest("invalid input")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Errorf("status = %d, want 400", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Code != 400 {
		t.Errorf("body.Code = %d, want 400", body.Code)
	}
	if body.Message != "invalid input" {
		t.Errorf("body.Message = %q, want %q", body.Message, "invalid input")
	}
}

func TestApp_OnShutdown(t *testing.T) {
	app := New()

	called := 0
	app.OnShutdown(func() { called++ })
	app.OnShutdown(func() { called++ })

	if len(app.hooks.OnShutdown) != 2 {
		t.Fatalf("OnShutdown hooks = %d, want 2", len(app.hooks.OnShutdown))
	}

	for _, fn := range app.hooks.OnShutdown {
		fn()
	}
	if called != 2 {
		t.Errorf("called = %d, want 2", called)
	}
}

func TestApp_All(t *testing.T) {
	app := New()
	app.All("/any", func(c *Ctx) error { return nil })

	var params routeParams
	for _, method := range standardMethods {
		params.reset()
		if app.router.find(method, "/any", &params) == nil {
			t.Errorf("%s /any should be registered via All()", method)
		}
	}
}

func TestApp_ServeKruda_WithMiddleware(t *testing.T) {
	app := New()

	var order []string

	app.Use(func(c *Ctx) error {
		order = append(order, "mw1")
		return c.Next()
	})
	app.Use(func(c *Ctx) error {
		order = append(order, "mw2")
		return c.Next()
	})
	app.Get("/test", func(c *Ctx) error {
		order = append(order, "handler")
		return c.JSON(Map{"ok": true})
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}

	want := []string{"mw1", "mw2", "handler"}
	if len(order) != len(want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("execution order[%d] = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}

func TestIntegration_TypedPOST_ValidationError_422(t *testing.T) {
	type CreateUserReq struct {
		Name  string `json:"name" validate:"required,min=2"`
		Email string `json:"email" validate:"required,email"`
	}
	type UserRes struct {
		ID string `json:"id"`
	}

	app := New(WithValidator(NewValidator()))
	Post[CreateUserReq, UserRes](app, "/users", func(c *C[CreateUserReq]) (*UserRes, error) {
		t.Error("handler should not be called when validation fails")
		return &UserRes{ID: "1"}, nil
	})
	app.router.Compile()

	// Send invalid body: empty name, invalid email
	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"","email":"not-an-email"}`),
	}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	// R5.1: must respond with 422
	if resp.statusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.statusCode)
	}

	// R5.2: must be structured JSON from ValidationError.MarshalJSON()
	var body struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Errors  []struct {
			Field   string `json:"field"`
			Rule    string `json:"rule"`
			Param   string `json:"param"`
			Message string `json:"message"`
			Value   string `json:"value"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON response: %v\nbody: %s", err, resp.body)
	}

	if body.Code != 422 {
		t.Errorf("body.code = %d, want 422", body.Code)
	}
	if body.Message != "Validation failed" {
		t.Errorf("body.message = %q, want %q", body.Message, "Validation failed")
	}
	if len(body.Errors) == 0 {
		t.Fatal("expected validation errors, got none")
	}

	// Verify we got errors for both fields
	fieldErrors := make(map[string]bool)
	for _, fe := range body.Errors {
		fieldErrors[fe.Field+":"+fe.Rule] = true
	}
	if !fieldErrors["name:required"] {
		t.Error("expected error for name:required")
	}
	if !fieldErrors["email:email"] {
		t.Error("expected error for email:email")
	}
}

func TestIntegration_TypedGET_QueryParams(t *testing.T) {
	type ListReq struct {
		Page  int    `query:"page" default:"1"`
		Limit int    `query:"limit" default:"10"`
		Sort  string `query:"sort" default:"id"`
	}
	type ListRes struct {
		Page  int    `json:"page"`
		Limit int    `json:"limit"`
		Sort  string `json:"sort"`
	}

	app := New()
	Get[ListReq, ListRes](app, "/items", func(c *C[ListReq]) (*ListRes, error) {
		return &ListRes{
			Page:  c.In.Page,
			Limit: c.In.Limit,
			Sort:  c.In.Sort,
		}, nil
	})
	app.router.Compile()

	// Send GET with query params
	req := &mockRequest{
		method: "GET",
		path:   "/items",
		query: map[string]string{
			"page":  "3",
			"limit": "25",
			"sort":  "name",
		},
	}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var body ListRes
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}

	if body.Page != 3 {
		t.Errorf("page = %d, want 3", body.Page)
	}
	if body.Limit != 25 {
		t.Errorf("limit = %d, want 25", body.Limit)
	}
	if body.Sort != "name" {
		t.Errorf("sort = %q, want %q", body.Sort, "name")
	}
}

func TestIntegration_OnError_FiresForValidationErrors(t *testing.T) {
	type Req struct {
		Name string `json:"name" validate:"required"`
	}
	type Res struct {
		OK bool `json:"ok"`
	}

	app := New(WithValidator(NewValidator()))

	var mu sync.Mutex
	var hookCalled bool
	var hookCode int

	app.hooks.OnError = append(app.hooks.OnError, func(c *Ctx, err error) {
		mu.Lock()
		defer mu.Unlock()
		hookCalled = true
		// The error passed to OnError hooks is *KrudaError wrapping the ValidationError
		var ke *KrudaError
		if errors.As(err, &ke) {
			hookCode = ke.Code
		}
	})

	Post[Req, Res](app, "/test", func(c *C[Req]) (*Res, error) {
		t.Error("handler should not be called")
		return &Res{OK: true}, nil
	})
	app.router.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/test",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":""}`),
	}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	mu.Lock()
	defer mu.Unlock()

	if !hookCalled {
		t.Fatal("OnError hook should have been called for validation errors")
	}
	if hookCode != 422 {
		t.Errorf("hook received code = %d, want 422", hookCode)
	}
	if resp.statusCode != 422 {
		t.Errorf("status = %d, want 422", resp.statusCode)
	}
}

func TestIntegration_CustomErrorHandler_ReceivesValidationError(t *testing.T) {
	type Req struct {
		Email string `json:"email" validate:"required,email"`
	}
	type Res struct {
		OK bool `json:"ok"`
	}

	var receivedVE *ValidationError

	app := New(
		WithValidator(NewValidator()),
		WithErrorHandler(func(c *Ctx, ke *KrudaError) {
			var ve *ValidationError
			if errors.As(ke, &ve) {
				receivedVE = ve
			}
			// Custom response
			c.Status(422)
			_ = c.JSON(Map{
				"custom":   true,
				"n_errors": len(ve.Errors),
			})
		}),
	)

	Post[Req, Res](app, "/check", func(c *C[Req]) (*Res, error) {
		t.Error("handler should not be called")
		return &Res{OK: true}, nil
	})
	app.router.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/check",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"email":"bad"}`),
	}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if receivedVE == nil {
		t.Fatal("custom ErrorHandler should have received a *ValidationError")
	}
	if len(receivedVE.Errors) == 0 {
		t.Error("ValidationError should have at least one FieldError")
	}

	// Verify the custom handler's response was used
	if resp.statusCode != 422 {
		t.Errorf("status = %d, want 422", resp.statusCode)
	}

	var body map[string]any
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body["custom"] != true {
		t.Error("expected custom=true in response from custom error handler")
	}
}

// mockFlusherResponse implements transport.ResponseWriter + http.Flusher
type mockFlusherResponse struct {
	mockResponseWriter
	flushCount int
}

func (m *mockFlusherResponse) Flush() {
	m.flushCount++
}

func newMockFlusherResponse() *mockFlusherResponse {
	return &mockFlusherResponse{
		mockResponseWriter: *newMockResponse(),
	}
}

func TestIntegration_SSE_Streaming(t *testing.T) {
	app := New()

	app.Get("/events", func(c *Ctx) error {
		return c.SSE(func(s *SSEStream) error {
			if err := s.Event("greeting", "hello"); err != nil {
				return err
			}
			if err := s.Data(42); err != nil {
				return err
			}
			if err := s.Comment("keep-alive"); err != nil {
				return err
			}
			return nil
		})
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/events"}
	resp := newMockFlusherResponse()

	app.ServeKruda(resp, req)

	// Check SSE headers
	ct := resp.headers.Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	cc := resp.headers.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", cc)
	}
	conn := resp.headers.Get("Connection")
	if conn != "keep-alive" {
		t.Errorf("Connection = %q, want keep-alive", conn)
	}

	// Check body contains SSE events
	body := string(resp.body)
	if !strings.Contains(body, "event: greeting\n") {
		t.Error("body should contain 'event: greeting'")
	}
	if !strings.Contains(body, "data: \"hello\"\n") {
		t.Errorf("body should contain data hello, got: %s", body)
	}
	if !strings.Contains(body, "data: 42\n") {
		t.Error("body should contain 'data: 42'")
	}
	if !strings.Contains(body, ": keep-alive\n") {
		t.Error("body should contain comment ': keep-alive'")
	}

	// Check flushing happened
	if resp.flushCount < 3 {
		t.Errorf("flush count = %d, want >= 3", resp.flushCount)
	}
}

func TestIntegration_OpenAPI_ServedAtPath(t *testing.T) {
	type ItemReq struct {
		Name string `json:"name" validate:"required"`
	}
	type ItemRes struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	app := New(
		WithValidator(NewValidator()),
		WithOpenAPIInfo("Items API", "1.0.0", "An items API"),
		WithOpenAPIPath("/api/spec.json"),
	)

	Post[ItemReq, ItemRes](app, "/items", func(c *C[ItemReq]) (*ItemRes, error) {
		return &ItemRes{ID: "1", Name: c.In.Name}, nil
	}, WithDescription("Create item"), WithTags("items"))

	// Build OpenAPI spec and register handler (simulating what Listen does)
	specJSON, err := app.buildOpenAPISpec()
	if err != nil {
		t.Fatalf("buildOpenAPISpec failed: %v", err)
	}
	// Register the OpenAPI handler manually (Listen does this automatically)
	app.Get(app.config.openAPIPath, func(c *Ctx) error {
		c.SetHeader("Content-Type", "application/json")
		c.SetHeader("Cache-Control", "public, max-age=3600")
		return c.sendBytes(specJSON)
	})
	app.router.Compile()

	// Request the OpenAPI spec
	req := &mockRequest{method: "GET", path: "/api/spec.json"}
	resp := newMockResponse()

	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	// Check Content-Type
	ct := resp.headers.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// Parse the spec
	var spec map[string]any
	if err := json.Unmarshal(resp.body, &spec); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}

	// Verify OpenAPI version
	if spec["openapi"] != "3.1.0" {
		t.Errorf("openapi = %v, want 3.1.0", spec["openapi"])
	}

	// Verify info
	info := spec["info"].(map[string]any)
	if info["title"] != "Items API" {
		t.Errorf("title = %v, want Items API", info["title"])
	}

	// Verify paths contain /items
	paths := spec["paths"].(map[string]any)
	if _, ok := paths["/items"]; !ok {
		t.Error("spec should contain /items path")
	}

	// Verify POST /items has description
	itemsPath := paths["/items"].(map[string]any)
	postOp := itemsPath["post"].(map[string]any)
	if postOp["description"] != "Create item" {
		t.Errorf("description = %v, want Create item", postOp["description"])
	}

	// Verify 422 response exists (has validation)
	responses := postOp["responses"].(map[string]any)
	if _, ok := responses["422"]; !ok {
		t.Error("POST /items should have 422 response (has validation)")
	}
}

func TestCtxQuery_DelegatesToTransport(t *testing.T) {
	app := New()
	req := &mockRequest{
		method: "GET",
		path:   "/test",
		query:  map[string]string{"page": "5", "sort": "name"},
	}
	resp := newMockResponse()

	c := newCtx(app)
	c.reset(resp, req)

	// Query delegates to transport's QueryParam
	if got := c.Query("page"); got != "5" {
		t.Errorf("Query(page) = %q, want 5", got)
	}
	if got := c.Query("sort"); got != "name" {
		t.Errorf("Query(sort) = %q, want name", got)
	}
}

func TestCtxQuery_MissingKeyReturnsDefault(t *testing.T) {
	app := New()
	req := &mockRequest{
		method: "GET",
		path:   "/test",
		query:  map[string]string{"a": "1"},
	}
	resp := newMockResponse()

	c := newCtx(app)
	c.reset(resp, req)

	// Missing key without default returns ""
	if got := c.Query("missing"); got != "" {
		t.Errorf("Query(missing) = %q, want empty", got)
	}

	// Missing key with default returns default
	if got := c.Query("missing", "fallback"); got != "fallback" {
		t.Errorf("Query(missing, fallback) = %q, want fallback", got)
	}

	// Present key ignores default
	if got := c.Query("a", "fallback"); got != "1" {
		t.Errorf("Query(a, fallback) = %q, want 1", got)
	}
}

func TestCtxQuery_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	// request is nil — should not panic, return default
	if got := c.Query("any", "safe"); got != "safe" {
		t.Errorf("Query with nil request = %q, want safe", got)
	}
	if got := c.Query("any"); got != "" {
		t.Errorf("Query with nil request (no default) = %q, want empty", got)
	}
}
