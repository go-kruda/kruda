package kruda

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// validatedUser exercises the validation path: it has validate tags so a
// configured Validator produces 422s on invalid create/update bodies.
type validatedUser struct {
	ID    string `json:"id"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

type validatedUserService struct {
	createFn func(ctx context.Context, item validatedUser) (validatedUser, error)
	updateFn func(ctx context.Context, id string, item validatedUser) (validatedUser, error)
}

func (s *validatedUserService) List(_ context.Context, _, _ int) ([]validatedUser, int, error) {
	return nil, 0, nil
}
func (s *validatedUserService) Create(ctx context.Context, item validatedUser) (validatedUser, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}
func (s *validatedUserService) Get(_ context.Context, id string) (validatedUser, error) {
	return validatedUser{ID: id}, nil
}
func (s *validatedUserService) Update(ctx context.Context, id string, item validatedUser) (validatedUser, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	item.ID = id
	return item, nil
}
func (s *validatedUserService) Delete(_ context.Context, _ string) error { return nil }

// =============================================================================
// §5.2 Validation engaged (422), gated identically to typed routes
// =============================================================================

func TestResourceCreate_Validation422(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"","email":"not-an-email"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 422 {
		t.Fatalf("status = %d, want 422\nbody: %s", resp.statusCode, resp.body)
	}
	var body struct {
		Code   int `json:"code"`
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Code != 422 {
		t.Errorf("body.code = %d, want 422", body.Code)
	}
	if len(body.Errors) == 0 {
		t.Error("expected at least one field error")
	}
}

func TestResourceCreate_ValidationPasses201(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"Alice","email":"alice@example.com"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", resp.statusCode, resp.body)
	}
}

func TestResourceUpdate_Validation422(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"","email":"bad"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 422 {
		t.Fatalf("status = %d, want 422\nbody: %s", resp.statusCode, resp.body)
	}
}

func TestResourceUpdate_ValidationPasses200(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"Bob","email":"bob@example.com"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}
}

// No validator configured → no 422 even on invalid data (typed-identical).
func TestResourceCreate_NoValidatorNo422(t *testing.T) {
	app := New() // no WithValidator
	Resource(app, "/users", &validatedUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"","email":"not-an-email"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode == 422 {
		t.Fatalf("status = 422 but no validator configured; want non-422\nbody: %s", resp.body)
	}
	if resp.statusCode != 201 {
		t.Fatalf("status = %d, want 201 (service echoes item)\nbody: %s", resp.statusCode, resp.body)
	}
}

// T without validate tags → no 422 even with a validator configured.
func TestResourceCreate_NoTagsNo422(t *testing.T) {
	app := New(WithValidator(NewValidator()))
	Resource(app, "/users", &mockUserService{
		createFn: func(_ context.Context, item mockUser) (mockUser, error) { return item, nil },
	})
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":""}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 201 {
		t.Fatalf("status = %d, want 201 (mockUser has no validate tags)\nbody: %s", resp.statusCode, resp.body)
	}
}

// =============================================================================
// §5.2 / Goal 3: malformed JSON → 400 "invalid request body" (was 500)
// =============================================================================

func TestResourceCreate_MalformedJSON400(t *testing.T) {
	app := New()
	Resource(app, "/users", &mockUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{invalid json}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", resp.statusCode, resp.body)
	}
	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Message != "invalid request body" {
		t.Errorf("message = %q, want %q", body.Message, "invalid request body")
	}
}

func TestResourceUpdate_MalformedJSON400(t *testing.T) {
	app := New()
	Resource(app, "/users", &mockUserService{})
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{invalid json}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", resp.statusCode, resp.body)
	}
}

// =============================================================================
// §5.3 Pagination → 400 on invalid, defaults when absent, clamp over cap
// =============================================================================

func TestResourceList_PaginationDefaults(t *testing.T) {
	app := New()
	var gotPage, gotLimit int
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			gotPage, gotLimit = page, limit
			return nil, 0, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	if gotPage != 1 || gotLimit != 20 {
		t.Errorf("page,limit = %d,%d, want 1,20", gotPage, gotLimit)
	}
}

func TestResourceList_PaginationInvalid400(t *testing.T) {
	cases := []map[string]string{
		{"page": "abc"},
		{"page": "0"},
		{"limit": "-1"},
		{"limit": "abc"},
		{"page": "1.5"},
	}
	for _, q := range cases {
		app := New()
		Resource(app, "/users", &mockUserService{})
		app.Compile()
		req := &mockRequest{method: "GET", path: "/users", query: q}
		resp := newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 400 {
			t.Errorf("query %v: status = %d, want 400\nbody: %s", q, resp.statusCode, resp.body)
		}
	}
}

func TestResourceList_PaginationInvalidMessage(t *testing.T) {
	app := New()
	Resource(app, "/users", &mockUserService{})
	app.Compile()
	req := &mockRequest{method: "GET", path: "/users", query: map[string]string{"page": "abc"}}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.statusCode)
	}
	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	want := `invalid query parameter "page": expected int`
	if body.Message != want {
		t.Errorf("message = %q, want %q", body.Message, want)
	}
}

func TestResourceList_LimitClamped(t *testing.T) {
	app := New()
	var gotLimit int
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			gotLimit = limit
			return nil, 0, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users", query: map[string]string{"limit": "1000"}}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}
	if gotLimit != maxResourceListLimit {
		t.Errorf("service limit = %d, want clamped %d", gotLimit, maxResourceListLimit)
	}
	// Envelope must also report the clamped limit.
	var body struct {
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Limit != maxResourceListLimit {
		t.Errorf("envelope limit = %d, want %d", body.Limit, maxResourceListLimit)
	}
}

// =============================================================================
// §5.4 ResourceList[T] envelope shape (identical JSON to the old Map)
// =============================================================================

func TestResourceList_EnvelopeShape(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{{ID: "1", Name: "A"}}, 7, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users", query: map[string]string{"page": "3", "limit": "9"}}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.body, &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, k := range []string{"data", "total", "page", "limit"} {
		if _, ok := raw[k]; !ok {
			t.Errorf("envelope missing key %q\nbody: %s", k, resp.body)
		}
	}
	var env ResourceList[mockUser]
	if err := json.Unmarshal(resp.body, &env); err != nil {
		t.Fatalf("ResourceList unmarshal: %v", err)
	}
	if env.Total != 7 || env.Page != 3 || env.Limit != 9 || len(env.Data) != 1 {
		t.Errorf("envelope = %+v, want total=7 page=3 limit=9 len(data)=1", env)
	}
}

// =============================================================================
// §5.7 Unsupported ID kind → registration panics
// =============================================================================

type int32Item struct {
	ID int32 `json:"id"`
}

type int32ItemService struct{}

func (int32ItemService) List(_ context.Context, _, _ int) ([]int32Item, int, error) {
	return nil, 0, nil
}
func (int32ItemService) Create(_ context.Context, item int32Item) (int32Item, error) {
	return item, nil
}
func (int32ItemService) Get(_ context.Context, id int32) (int32Item, error) {
	return int32Item{ID: id}, nil
}
func (int32ItemService) Update(_ context.Context, id int32, item int32Item) (int32Item, error) {
	return item, nil
}
func (int32ItemService) Delete(_ context.Context, _ int32) error { return nil }

func TestResource_UnsupportedIDKindPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for unsupported ID kind int32")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "unsupported") {
			t.Errorf("panic = %v, want message containing 'unsupported'", r)
		}
	}()
	app := New()
	Resource[int32Item, int32](app, "/items", int32ItemService{})
}
