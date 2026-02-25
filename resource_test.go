package kruda

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Mock types for Resource tests
// ---------------------------------------------------------------------------

type mockUser struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type mockUserService struct {
	listFn   func(ctx context.Context, page, limit int) ([]mockUser, int, error)
	createFn func(ctx context.Context, item mockUser) (mockUser, error)
	getFn    func(ctx context.Context, id string) (mockUser, error)
	updateFn func(ctx context.Context, id string, item mockUser) (mockUser, error)
	deleteFn func(ctx context.Context, id string) error
}

func (s *mockUserService) List(ctx context.Context, page, limit int) ([]mockUser, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, page, limit)
	}
	return nil, 0, nil
}

func (s *mockUserService) Create(ctx context.Context, item mockUser) (mockUser, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}

func (s *mockUserService) Get(ctx context.Context, id string) (mockUser, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return mockUser{}, nil
}

func (s *mockUserService) Update(ctx context.Context, id string, item mockUser) (mockUser, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	return item, nil
}
func (s *mockUserService) Delete(ctx context.Context, id string) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

// mockItem + mockItemService for int ID tests
type mockItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mockItemService struct {
	getFn    func(ctx context.Context, id int) (mockItem, error)
	listFn   func(ctx context.Context, page, limit int) ([]mockItem, int, error)
	createFn func(ctx context.Context, item mockItem) (mockItem, error)
	updateFn func(ctx context.Context, id int, item mockItem) (mockItem, error)
	deleteFn func(ctx context.Context, id int) error
}

func (s *mockItemService) List(ctx context.Context, page, limit int) ([]mockItem, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, page, limit)
	}
	return nil, 0, nil
}
func (s *mockItemService) Create(ctx context.Context, item mockItem) (mockItem, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}
func (s *mockItemService) Get(ctx context.Context, id int) (mockItem, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return mockItem{}, nil
}
func (s *mockItemService) Update(ctx context.Context, id int, item mockItem) (mockItem, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	return item, nil
}
func (s *mockItemService) Delete(ctx context.Context, id int) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test 1: GET /users — list with pagination
// ---------------------------------------------------------------------------

func TestResourceList(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{
				{ID: "1", Name: "Alice"},
				{ID: "2", Name: "Bob"},
			}, 10, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method: "GET",
		path:   "/users",
		query:  map[string]string{"page": "2", "limit": "5"},
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var body struct {
		Data  []mockUser `json:"data"`
		Total int        `json:"total"`
		Page  int        `json:"page"`
		Limit int        `json:"limit"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Total != 10 {
		t.Errorf("total = %d, want 10", body.Total)
	}
	if body.Page != 2 {
		t.Errorf("page = %d, want 2", body.Page)
	}
	if body.Limit != 5 {
		t.Errorf("limit = %d, want 5", body.Limit)
	}
	if len(body.Data) != 2 {
		t.Errorf("data length = %d, want 2", len(body.Data))
	}
}

// ---------------------------------------------------------------------------
// Test 2: GET /users/abc — get single resource
// ---------------------------------------------------------------------------

func TestResourceGet(t *testing.T) {
	app := New()
	svc := &mockUserService{
		getFn: func(_ context.Context, id string) (mockUser, error) {
			return mockUser{ID: id, Name: "Alice"}, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var user mockUser
	if err := json.Unmarshal(resp.body, &user); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if user.ID != "abc" {
		t.Errorf("id = %q, want abc", user.ID)
	}
	if user.Name != "Alice" {
		t.Errorf("name = %q, want Alice", user.Name)
	}
}

// ---------------------------------------------------------------------------
// Test 3: POST /users — create resource
// ---------------------------------------------------------------------------

func TestResourceCreate(t *testing.T) {
	app := New()
	svc := &mockUserService{
		createFn: func(_ context.Context, item mockUser) (mockUser, error) {
			item.ID = "new-id"
			return item, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"Charlie"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", resp.statusCode, resp.body)
	}

	var user mockUser
	if err := json.Unmarshal(resp.body, &user); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if user.ID != "new-id" {
		t.Errorf("id = %q, want new-id", user.ID)
	}
	if user.Name != "Charlie" {
		t.Errorf("name = %q, want Charlie", user.Name)
	}
}

// ---------------------------------------------------------------------------
// Test 4: PUT /users/abc — update resource
// ---------------------------------------------------------------------------

func TestResourceUpdate(t *testing.T) {
	app := New()
	svc := &mockUserService{
		updateFn: func(_ context.Context, id string, item mockUser) (mockUser, error) {
			item.ID = id
			return item, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"Updated"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var user mockUser
	if err := json.Unmarshal(resp.body, &user); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if user.ID != "abc" {
		t.Errorf("id = %q, want abc", user.ID)
	}
	if user.Name != "Updated" {
		t.Errorf("name = %q, want Updated", user.Name)
	}
}

// ---------------------------------------------------------------------------
// Test 5: DELETE /users/abc — delete resource
// ---------------------------------------------------------------------------

func TestResourceDelete(t *testing.T) {
	app := New()
	deletedID := ""
	svc := &mockUserService{
		deleteFn: func(_ context.Context, id string) error {
			deletedID = id
			return nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "DELETE", path: "/users/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 204 {
		t.Fatalf("status = %d, want 204\nbody: %s", resp.statusCode, resp.body)
	}
	if deletedID != "abc" {
		t.Errorf("deleted id = %q, want abc", deletedID)
	}
}

// ---------------------------------------------------------------------------
// Test 6: Service error propagation
// ---------------------------------------------------------------------------

func TestResourceServiceError(t *testing.T) {
	app := New()
	svc := &mockUserService{
		getFn: func(_ context.Context, id string) (mockUser, error) {
			return mockUser{}, errors.New("db connection failed")
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	// Non-KrudaError defaults to 500
	if resp.statusCode != 500 {
		t.Fatalf("status = %d, want 500\nbody: %s", resp.statusCode, resp.body)
	}
}

// ---------------------------------------------------------------------------
// Test 7: WithResourceOnly — only GET registered
// ---------------------------------------------------------------------------

func TestResourceWithOnly(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{}, 0, nil
		},
	}
	Resource(app, "/users", svc, WithResourceOnly("GET"))
	app.Compile()

	// GET should work
	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("GET status = %d, want 200", resp.statusCode)
	}

	// POST should not be registered
	req = &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"X"}`),
	}
	resp = newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 404 && resp.statusCode != 405 {
		t.Errorf("POST status = %d, want 404 or 405", resp.statusCode)
	}
}

// ---------------------------------------------------------------------------
// Test 8: WithResourceExcept — DELETE excluded
// ---------------------------------------------------------------------------

func TestResourceWithExcept(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{}, 0, nil
		},
	}
	Resource(app, "/users", svc, WithResourceExcept("DELETE"))
	app.Compile()

	// GET should work
	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("GET status = %d, want 200", resp.statusCode)
	}

	// DELETE should not be registered
	req = &mockRequest{method: "DELETE", path: "/users/abc"}
	resp = newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 404 && resp.statusCode != 405 {
		t.Errorf("DELETE status = %d, want 404 or 405", resp.statusCode)
	}
}

// ---------------------------------------------------------------------------
// Test 9: WithResourceMiddleware — middleware sets header
// ---------------------------------------------------------------------------

func TestResourceWithMiddleware(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{}, 0, nil
		},
	}
	Resource(app, "/users", svc, WithResourceMiddleware(func(c *Ctx) error {
		c.SetHeader("X-Resource", "users")
		return c.Next()
	}))
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}
	if resp.headers.Get("X-Resource") != "users" {
		t.Errorf("X-Resource = %q, want users", resp.headers.Get("X-Resource"))
	}
}

// ---------------------------------------------------------------------------
// Test 10: String ID parsing
// ---------------------------------------------------------------------------

func TestResourceParseIDString(t *testing.T) {
	app := New()
	var receivedID string
	svc := &mockUserService{
		getFn: func(_ context.Context, id string) (mockUser, error) {
			receivedID = id
			return mockUser{ID: id, Name: "Test"}, nil
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/hello-world-123"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}
	if receivedID != "hello-world-123" {
		t.Errorf("received id = %q, want hello-world-123", receivedID)
	}
}

// ---------------------------------------------------------------------------
// Test 11: Int ID parsing
// ---------------------------------------------------------------------------

func TestResourceParseIDInt(t *testing.T) {
	app := New()
	var receivedID int
	svc := &mockItemService{
		getFn: func(_ context.Context, id int) (mockItem, error) {
			receivedID = id
			return mockItem{ID: id, Name: "Widget"}, nil
		},
	}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/42"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}
	if receivedID != 42 {
		t.Errorf("received id = %d, want 42", receivedID)
	}
}

// ---------------------------------------------------------------------------
// Test 12: Invalid int ID returns 400
// ---------------------------------------------------------------------------

func TestResourceInvalidID(t *testing.T) {
	app := New()
	svc := &mockItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/not-a-number"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", resp.statusCode, resp.body)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if body.Code != 400 {
		t.Errorf("body.Code = %d, want 400", body.Code)
	}
}

// ---------------------------------------------------------------------------
// Test 13: GroupResource — routes registered with group prefix
// ---------------------------------------------------------------------------

func TestGroupResource(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{{ID: "1", Name: "Alice"}}, 1, nil
		},
		getFn: func(_ context.Context, id string) (mockUser, error) {
			return mockUser{ID: id, Name: "Alice"}, nil
		},
	}

	g := app.Group("/api/v1")
	GroupResource(g, "/users", svc)
	app.Compile()

	// List via group prefix
	req := &mockRequest{method: "GET", path: "/api/v1/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("GET /api/v1/users status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var body struct {
		Data []mockUser `json:"data"`
	}
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if len(body.Data) != 1 {
		t.Errorf("data length = %d, want 1", len(body.Data))
	}

	// Get via group prefix
	req = &mockRequest{method: "GET", path: "/api/v1/users/xyz"}
	resp = newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("GET /api/v1/users/xyz status = %d, want 200\nbody: %s", resp.statusCode, resp.body)
	}

	var user mockUser
	if err := json.Unmarshal(resp.body, &user); err != nil {
		t.Fatalf("invalid JSON: %v\nbody: %s", err, resp.body)
	}
	if user.ID != "xyz" {
		t.Errorf("id = %q, want xyz", user.ID)
	}
}
