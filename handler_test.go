package kruda

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

type createUserIn struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type userOut struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type queryIn struct {
	Page int    `query:"page" default:"1"`
	Sort string `query:"sort" default:"created_at"`
}

type emptyOut struct{}

func TestTypedHandler_PostWithBody(t *testing.T) {
	app := New()
	var captured createUserIn

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: "u1", Name: c.In.Name}, nil
	}, nil)

	body := []byte(`{"name":"Alice","email":"alice@example.com"}`)
	c := bindCtx("POST", "/users", nil, nil, body)
	resp := c.writer.(*mockResponseWriter)

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Name != "Alice" {
		t.Errorf("In.Name = %q, want Alice", captured.Name)
	}
	if captured.Email != "alice@example.com" {
		t.Errorf("In.Email = %q, want alice@example.com", captured.Email)
	}
	// Response should be 200 JSON
	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}
	var out userOut
	if err := json.Unmarshal(resp.body, &out); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if out.ID != "u1" || out.Name != "Alice" {
		t.Errorf("response = %+v, want {u1 Alice}", out)
	}
}

func TestTypedHandler_GetWithQueryParams(t *testing.T) {
	app := New()
	var captured queryIn

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: "list", Name: fmt.Sprintf("page=%d,sort=%s", c.In.Page, c.In.Sort)}, nil
	}, nil)

	c := bindCtx("GET", "/users", nil, map[string]string{"page": "3", "sort": "name"}, nil)

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Page != 3 {
		t.Errorf("In.Page = %d, want 3", captured.Page)
	}
	if captured.Sort != "name" {
		t.Errorf("In.Sort = %q, want name", captured.Sort)
	}
}

func TestTypedHandler_GetWithQueryDefaults(t *testing.T) {
	app := New()
	var captured queryIn

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: "list"}, nil
	}, nil)

	// No query params — defaults should apply
	c := bindCtx("GET", "/users", nil, nil, nil)

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Page != 1 {
		t.Errorf("In.Page = %d, want 1 (default)", captured.Page)
	}
	if captured.Sort != "created_at" {
		t.Errorf("In.Sort = %q, want created_at (default)", captured.Sort)
	}
}

func TestTypedHandler_ValidationError_EmptyBody(t *testing.T) {
	app := New(WithValidator(NewValidator()))

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		t.Fatal("handler should not be called on validation failure")
		return nil, nil
	}, nil)

	// POST with empty required fields → validation should fail
	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"","email":""}`))

	err := h(c)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}
	if len(ve.Errors) == 0 {
		t.Fatal("expected at least one field error")
	}

	// Both name and email should fail "required"
	fields := make(map[string]bool)
	for _, fe := range ve.Errors {
		fields[fe.Field] = true
	}
	if !fields["name"] {
		t.Error("expected validation error for field 'name'")
	}
	if !fields["email"] {
		t.Error("expected validation error for field 'email'")
	}
}

func TestTypedHandler_ValidationError_InvalidEmail(t *testing.T) {
	app := New(WithValidator(NewValidator()))

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		t.Fatal("handler should not be called on validation failure")
		return nil, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Bob","email":"not-an-email"}`))

	err := h(c)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T: %v", err, err)
	}

	// name passes required, email fails email rule
	found := false
	for _, fe := range ve.Errors {
		if fe.Field == "email" && fe.Rule == "email" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected email field error with rule 'email', got %+v", ve.Errors)
	}
}

func TestTypedHandler_ValidationError_Returns422Code(t *testing.T) {
	app := New(WithValidator(NewValidator()))

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		return nil, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"","email":""}`))

	err := h(c)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	// Verify the JSON marshaling produces code 422
	data, marshalErr := ve.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("MarshalJSON failed: %v", marshalErr)
	}
	var resp struct {
		Code    int          `json:"code"`
		Message string       `json:"message"`
		Errors  []FieldError `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Code != 422 {
		t.Errorf("code = %d, want 422", resp.Code)
	}
	if resp.Message != "Validation failed" {
		t.Errorf("message = %q, want 'Validation failed'", resp.Message)
	}
}

func TestTypedHandler_NoValidator_SkipsValidation(t *testing.T) {
	// App without validator — validation tags are ignored
	app := New()
	var called bool

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		called = true
		return &userOut{ID: "ok"}, nil
	}, nil)

	// Empty fields would fail validation, but no validator is configured
	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"","email":""}`))

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("handler should have been called (no validator)")
	}
}

func TestTypedHandler_NonNilResponse200(t *testing.T) {
	app := New()

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		return &userOut{ID: "u42", Name: "Alice"}, nil
	}, nil)

	c := bindCtx("GET", "/users", nil, nil, nil)
	resp := c.writer.(*mockResponseWriter)

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}
	if !c.Responded() {
		t.Error("Responded() should be true after JSON response")
	}

	var out userOut
	if err := json.Unmarshal(resp.body, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.ID != "u42" || out.Name != "Alice" {
		t.Errorf("body = %+v, want {u42 Alice}", out)
	}
}

func TestTypedHandler_NilResponse204(t *testing.T) {
	app := New()

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		return nil, nil // nil result → 204
	}, nil)

	c := bindCtx("GET", "/users", nil, nil, nil)
	resp := c.writer.(*mockResponseWriter)

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.statusCode != 204 {
		t.Errorf("status = %d, want 204", resp.statusCode)
	}
	if !c.Responded() {
		t.Error("Responded() should be true after NoContent")
	}
	if len(resp.body) != 0 {
		t.Errorf("body should be empty for 204, got %q", resp.body)
	}
}

func TestTypedHandler_ErrorPropagated(t *testing.T) {
	app := New()
	sentinel := errors.New("something went wrong")

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		return nil, sentinel
	}, nil)

	c := bindCtx("GET", "/users", nil, nil, nil)

	err := h(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel error", err)
	}
	// Response should NOT have been sent — error is returned to caller
	if c.Responded() {
		t.Error("Responded() should be false when handler returns error")
	}
}

func TestTypedHandler_KrudaErrorPropagated(t *testing.T) {
	app := New()

	h := buildTypedHandler[queryIn, userOut](app, "GET", "/test", func(c *C[queryIn]) (*userOut, error) {
		return nil, BadRequest("invalid request")
	}, nil)

	c := bindCtx("GET", "/users", nil, nil, nil)

	err := h(c)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var ke *KrudaError
	if !errors.As(err, &ke) {
		t.Fatalf("expected *KrudaError, got %T", err)
	}
	if ke.Code != 400 {
		t.Errorf("error code = %d, want 400", ke.Code)
	}
}

func TestShortHandler_GetX_NonNil200(t *testing.T) {
	app := New()

	// GetX registers via app.Get internally, so we use the full registration path
	var captured queryIn
	GetX[queryIn, userOut](app, "/users", func(c *C[queryIn]) *userOut {
		captured = c.In
		return &userOut{ID: "x1", Name: "ShortGet"}
	})
	app.router.Compile()

	// Simulate request through ServeKruda
	req := &bindMockRequest{
		mockRequest: mockRequest{method: "GET", path: "/users"},
		queryParams: map[string]string{"page": "2", "sort": "id"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}
	if captured.Page != 2 {
		t.Errorf("In.Page = %d, want 2", captured.Page)
	}

	var out userOut
	if err := json.Unmarshal(resp.body, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.ID != "x1" || out.Name != "ShortGet" {
		t.Errorf("body = %+v, want {x1 ShortGet}", out)
	}
}

func TestShortHandler_PostX_NonNil200(t *testing.T) {
	app := New()

	PostX[createUserIn, userOut](app, "/users", func(c *C[createUserIn]) *userOut {
		return &userOut{ID: "p1", Name: c.In.Name}
	})
	app.router.Compile()

	req := &bindMockRequest{
		mockRequest: mockRequest{
			method: "POST",
			path:   "/users",
			body:   []byte(`{"name":"PostXUser","email":"px@test.com"}`),
		},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Errorf("status = %d, want 200", resp.statusCode)
	}

	var out userOut
	if err := json.Unmarshal(resp.body, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if out.Name != "PostXUser" {
		t.Errorf("Name = %q, want PostXUser", out.Name)
	}
}

func TestShortHandler_GetX_Nil204(t *testing.T) {
	app := New()

	GetX[queryIn, userOut](app, "/empty", func(c *C[queryIn]) *userOut {
		return nil // nil → 204
	})
	app.router.Compile()

	req := &bindMockRequest{
		mockRequest: mockRequest{method: "GET", path: "/empty"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 204 {
		t.Errorf("status = %d, want 204", resp.statusCode)
	}
	if len(resp.body) != 0 {
		t.Errorf("body should be empty for 204, got %q", resp.body)
	}
}

func TestShortHandler_PostX_Nil204(t *testing.T) {
	app := New()

	PostX[createUserIn, emptyOut](app, "/fire-and-forget", func(c *C[createUserIn]) *emptyOut {
		return nil // nil → 204
	})
	app.router.Compile()

	req := &bindMockRequest{
		mockRequest: mockRequest{
			method: "POST",
			path:   "/fire-and-forget",
			body:   []byte(`{"name":"test","email":"t@t.com"}`),
		},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 204 {
		t.Errorf("status = %d, want 204", resp.statusCode)
	}
}

func TestTypedHandler_PostWithParams(t *testing.T) {
	type mixedIn struct {
		ID   string `param:"id"`
		Name string `json:"name"`
	}
	app := New()
	var captured mixedIn

	h := buildTypedHandler[mixedIn, userOut](app, "POST", "/users/:id", func(c *C[mixedIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: c.In.ID, Name: c.In.Name}, nil
	}, nil)

	c := bindCtx("POST", "/users/abc", map[string]string{"id": "abc"}, nil, []byte(`{"name":"Mixed"}`))

	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "abc" {
		t.Errorf("In.ID = %q, want abc", captured.ID)
	}
	if captured.Name != "Mixed" {
		t.Errorf("In.Name = %q, want Mixed", captured.Name)
	}
}

var _ = fmt.Sprintf

func TestOnParse_HookCalled(t *testing.T) {
	app := New()
	var hookInput any
	app.OnParse(func(c *Ctx, input any) error {
		hookInput = input
		return nil
	})

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Alice","email":"alice@test.com"}`))
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hookInput == nil {
		t.Fatal("OnParse hook was not called")
	}
	// hookInput should be a pointer to createUserIn
	if req, ok := hookInput.(*createUserIn); ok {
		if req.Name != "Alice" {
			t.Errorf("hook input Name = %q, want Alice", req.Name)
		}
	} else {
		t.Errorf("hook input type = %T, want *createUserIn", hookInput)
	}
}

func TestOnParse_HookOrdering(t *testing.T) {
	app := New()
	var order []int
	app.OnParse(func(c *Ctx, input any) error {
		order = append(order, 1)
		return nil
	})
	app.OnParse(func(c *Ctx, input any) error {
		order = append(order, 2)
		return nil
	})
	app.OnParse(func(c *Ctx, input any) error {
		order = append(order, 3)
		return nil
	})

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Bob","email":"bob@test.com"}`))
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("hook order = %v, want [1 2 3]", order)
	}
}

func TestOnParse_ErrorStopsPipeline(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	sentinel := errors.New("parse hook failed")
	var hook2Called bool
	var handlerCalled bool

	app.OnParse(func(c *Ctx, input any) error {
		return sentinel
	})
	app.OnParse(func(c *Ctx, input any) error {
		hook2Called = true
		return nil
	})

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		handlerCalled = true
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Alice","email":"alice@test.com"}`))
	err := h(c)
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want sentinel", err)
	}
	if hook2Called {
		t.Error("hook2 should not be called after hook1 errors")
	}
	if handlerCalled {
		t.Error("handler should not be called after hook error")
	}
}

func TestOnParse_MutationVisibility(t *testing.T) {
	app := New(WithValidator(NewValidator()))

	// Hook that trims whitespace from Name
	app.OnParse(func(c *Ctx, input any) error {
		if req, ok := input.(*createUserIn); ok {
			req.Name = "Mutated"
		}
		return nil
	})

	var captured createUserIn
	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Original","email":"a@b.com"}`))
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.Name != "Mutated" {
		t.Errorf("In.Name = %q, want Mutated (hook should have modified it)", captured.Name)
	}
}

func TestOnParse_NoHooks_SkipsCleanly(t *testing.T) {
	app := New()
	// No OnParse hooks registered

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("POST", "/users", nil, nil, []byte(`{"name":"Alice","email":"a@b.com"}`))
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProvideNeed_RoundTrip(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	c.Provide("user_id", 42)
	val, ok := Need[int](c, "user_id")
	if !ok {
		t.Fatal("Need should return true for existing key")
	}
	if val != 42 {
		t.Errorf("Need returned %d, want 42", val)
	}
}

func TestProvideNeed_StringType(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	c.Provide("role", "admin")
	val, ok := Need[string](c, "role")
	if !ok {
		t.Fatal("Need should return true")
	}
	if val != "admin" {
		t.Errorf("Need returned %q, want admin", val)
	}
}

func TestNeed_MissingKey(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	val, ok := Need[string](c, "nonexistent")
	if ok {
		t.Error("Need should return false for missing key")
	}
	if val != "" {
		t.Errorf("Need should return zero value, got %q", val)
	}
}

func TestNeed_TypeMismatch(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	c.Provide("count", 42)              // int
	val, ok := Need[string](c, "count") // ask for string
	if ok {
		t.Error("Need should return false for type mismatch")
	}
	if val != "" {
		t.Errorf("Need should return zero value, got %q", val)
	}
}

type testUser struct {
	Name string
	Role string
}

func TestProvideNeed_StructPointer(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	user := &testUser{Name: "Alice", Role: "admin"}
	c.Provide("user", user)

	got, ok := Need[*testUser](c, "user")
	if !ok {
		t.Fatal("Need should return true")
	}
	if got.Name != "Alice" || got.Role != "admin" {
		t.Errorf("Need returned %+v, want {Alice admin}", got)
	}
}

func TestProvideNeed_OverwriteKey(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.reset(newMockResponse(), &mockRequest{method: "GET", path: "/"})

	c.Provide("key", "first")
	c.Provide("key", "second")

	val, ok := Need[string](c, "key")
	if !ok {
		t.Fatal("Need should return true")
	}
	if val != "second" {
		t.Errorf("Need returned %q, want second", val)
	}
}

func TestTypedHandler_MalformedJSON(t *testing.T) {
	app := New()

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		t.Fatal("handler should not be called with malformed JSON")
		return nil, nil
	}, nil)

	cases := []struct {
		name string
		body string
	}{
		{"truncated", `{"name":"Alice`},
		{"trailing comma", `{"name":"Alice",}`},
		{"bare value", `not json`},
		{"empty string", ``},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := bindCtx("POST", "/users", nil, nil, []byte(tc.body))
			err := h(c)
			if err == nil {
				t.Error("expected error for malformed JSON")
			}
		})
	}
}

func TestTypedHandler_EmptyBodyOnPost(t *testing.T) {
	app := New()

	h := buildTypedHandler[createUserIn, userOut](app, "POST", "/users", func(c *C[createUserIn]) (*userOut, error) {
		return &userOut{ID: "ok"}, nil
	}, nil)

	// nil body
	c := bindCtx("POST", "/users", nil, nil, nil)
	err := h(c)
	if err == nil {
		t.Log("nil body accepted — handler got zero-value struct")
	}
}

func TestTypedHandler_WrongContentType(t *testing.T) {
	app := New()
	app.Post("/users", func(c *Ctx) error {
		var req createUserIn
		if err := c.Bind(&req); err != nil {
			return BadRequest("invalid body")
		}
		return c.JSON(Map{"name": req.Name})
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("POST", "/users").
		ContentType("application/x-www-form-urlencoded").
		Body([]byte("name=Alice&email=alice@test.com")).
		Send()
	if err != nil {
		t.Fatal(err)
	}
	// Form-urlencoded sent to a JSON handler — should fail or behave gracefully
	if resp.StatusCode() == 200 {
		t.Log("framework accepted form-urlencoded as JSON — worth documenting")
	}
}

func TestTypedHandler_LargeNestedJSON(t *testing.T) {
	type nested struct {
		Items []createUserIn `json:"items"`
	}
	app := New()
	var captured nested

	h := buildTypedHandler[nested, userOut](app, "POST", "/bulk", func(c *C[nested]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: "bulk"}, nil
	}, nil)

	// Build a large but valid payload
	body := `{"items":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			body += ","
		}
		body += fmt.Sprintf(`{"name":"user%d","email":"u%d@test.com"}`, i, i)
	}
	body += `]}`

	c := bindCtx("POST", "/bulk", nil, nil, []byte(body))
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(captured.Items) != 100 {
		t.Errorf("expected 100 items, got %d", len(captured.Items))
	}
}

func TestTypedHandler_IntParamConversion(t *testing.T) {
	type idIn struct {
		ID int `param:"id"`
	}
	app := New()
	var captured idIn

	h := buildTypedHandler[idIn, userOut](app, "GET", "/users/:id", func(c *C[idIn]) (*userOut, error) {
		captured = c.In
		return &userOut{ID: fmt.Sprintf("%d", c.In.ID)}, nil
	}, nil)

	c := bindCtx("GET", "/users/42", map[string]string{"id": "42"}, nil, nil)
	err := h(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != 42 {
		t.Errorf("In.ID = %d, want 42", captured.ID)
	}
}

func TestTypedHandler_InvalidIntParam(t *testing.T) {
	type idIn struct {
		ID int `param:"id"`
	}
	app := New()

	h := buildTypedHandler[idIn, userOut](app, "GET", "/users/:id", func(c *C[idIn]) (*userOut, error) {
		return &userOut{ID: "ok"}, nil
	}, nil)

	c := bindCtx("GET", "/users/abc", map[string]string{"id": "abc"}, nil, nil)
	err := h(c)
	if err == nil {
		t.Error("expected error when param 'id' is not an integer")
	}
}
