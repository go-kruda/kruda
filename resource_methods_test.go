package kruda

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// resource.go — resourceParseID: int64, uint, uint64, unsupported type branches
// =============================================================================

type mockInt64Item struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type mockInt64ItemService struct {
	getFn    func(ctx context.Context, id int64) (mockInt64Item, error)
	listFn   func(ctx context.Context, page, limit int) ([]mockInt64Item, int, error)
	createFn func(ctx context.Context, item mockInt64Item) (mockInt64Item, error)
	updateFn func(ctx context.Context, id int64, item mockInt64Item) (mockInt64Item, error)
	deleteFn func(ctx context.Context, id int64) error
}

func (s *mockInt64ItemService) List(ctx context.Context, page, limit int) ([]mockInt64Item, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, page, limit)
	}
	return nil, 0, nil
}
func (s *mockInt64ItemService) Create(ctx context.Context, item mockInt64Item) (mockInt64Item, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}
func (s *mockInt64ItemService) Get(ctx context.Context, id int64) (mockInt64Item, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return mockInt64Item{}, nil
}
func (s *mockInt64ItemService) Update(ctx context.Context, id int64, item mockInt64Item) (mockInt64Item, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	return item, nil
}
func (s *mockInt64ItemService) Delete(ctx context.Context, id int64) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestResourceParseID_Int64(t *testing.T) {
	app := New()
	var receivedID int64
	svc := &mockInt64ItemService{
		getFn: func(_ context.Context, id int64) (mockInt64Item, error) {
			receivedID = id
			return mockInt64Item{ID: id, Name: "Test"}, nil
		},
	}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/999"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200, body: %s", resp.statusCode, resp.body)
	}
	if receivedID != 999 {
		t.Errorf("received id = %d, want 999", receivedID)
	}
}

func TestResourceParseID_Int64_Invalid(t *testing.T) {
	app := New()
	svc := &mockInt64ItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/not-a-number"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.statusCode)
	}
}

type mockUintItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type mockUintItemService struct {
	getFn    func(ctx context.Context, id uint) (mockUintItem, error)
	listFn   func(ctx context.Context, page, limit int) ([]mockUintItem, int, error)
	createFn func(ctx context.Context, item mockUintItem) (mockUintItem, error)
	updateFn func(ctx context.Context, id uint, item mockUintItem) (mockUintItem, error)
	deleteFn func(ctx context.Context, id uint) error
}

func (s *mockUintItemService) List(ctx context.Context, page, limit int) ([]mockUintItem, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, page, limit)
	}
	return nil, 0, nil
}
func (s *mockUintItemService) Create(ctx context.Context, item mockUintItem) (mockUintItem, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}
func (s *mockUintItemService) Get(ctx context.Context, id uint) (mockUintItem, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return mockUintItem{}, nil
}
func (s *mockUintItemService) Update(ctx context.Context, id uint, item mockUintItem) (mockUintItem, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	return item, nil
}
func (s *mockUintItemService) Delete(ctx context.Context, id uint) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestResourceParseID_Uint(t *testing.T) {
	app := New()
	var receivedID uint
	svc := &mockUintItemService{
		getFn: func(_ context.Context, id uint) (mockUintItem, error) {
			receivedID = id
			return mockUintItem{ID: id, Name: "Test"}, nil
		},
	}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/42"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200, body: %s", resp.statusCode, resp.body)
	}
	if receivedID != 42 {
		t.Errorf("received id = %d, want 42", receivedID)
	}
}

func TestResourceParseID_Uint_Invalid(t *testing.T) {
	app := New()
	svc := &mockUintItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.statusCode)
	}
}

type mockUint64Item struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
}

type mockUint64ItemService struct {
	getFn    func(ctx context.Context, id uint64) (mockUint64Item, error)
	listFn   func(ctx context.Context, page, limit int) ([]mockUint64Item, int, error)
	createFn func(ctx context.Context, item mockUint64Item) (mockUint64Item, error)
	updateFn func(ctx context.Context, id uint64, item mockUint64Item) (mockUint64Item, error)
	deleteFn func(ctx context.Context, id uint64) error
}

func (s *mockUint64ItemService) List(ctx context.Context, page, limit int) ([]mockUint64Item, int, error) {
	if s.listFn != nil {
		return s.listFn(ctx, page, limit)
	}
	return nil, 0, nil
}
func (s *mockUint64ItemService) Create(ctx context.Context, item mockUint64Item) (mockUint64Item, error) {
	if s.createFn != nil {
		return s.createFn(ctx, item)
	}
	return item, nil
}
func (s *mockUint64ItemService) Get(ctx context.Context, id uint64) (mockUint64Item, error) {
	if s.getFn != nil {
		return s.getFn(ctx, id)
	}
	return mockUint64Item{}, nil
}
func (s *mockUint64ItemService) Update(ctx context.Context, id uint64, item mockUint64Item) (mockUint64Item, error) {
	if s.updateFn != nil {
		return s.updateFn(ctx, id, item)
	}
	return item, nil
}
func (s *mockUint64ItemService) Delete(ctx context.Context, id uint64) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func TestResourceParseID_Uint64(t *testing.T) {
	app := New()
	var receivedID uint64
	svc := &mockUint64ItemService{
		getFn: func(_ context.Context, id uint64) (mockUint64Item, error) {
			receivedID = id
			return mockUint64Item{ID: id, Name: "Test"}, nil
		},
	}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/18446744073709551"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200, body: %s", resp.statusCode, resp.body)
	}
	if receivedID != 18446744073709551 {
		t.Errorf("received id = %d, want 18446744073709551", receivedID)
	}
}

func TestResourceParseID_Uint64_Invalid(t *testing.T) {
	app := New()
	svc := &mockUint64ItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/items/xyz"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.statusCode)
	}
}

// =============================================================================
// resource.go — registerResource: LIST-only and GET_BY_ID-only paths
// =============================================================================

func TestResourceWithOnly_ListOnly(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return []mockUser{{ID: "1", Name: "Alice"}}, 1, nil
		},
	}
	Resource(app, "/users", svc, WithResourceOnly("LIST"))
	app.Compile()

	// LIST should work
	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("GET /users status = %d, want 200", resp.statusCode)
	}

	// GET by ID should NOT work (only LIST registered)
	req = &mockRequest{method: "GET", path: "/users/1"}
	resp = newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode == 200 {
		t.Error("GET /users/1 should not be registered with LIST-only")
	}
}

func TestResourceWithOnly_GetByIDOnly(t *testing.T) {
	app := New()
	svc := &mockUserService{
		getFn: func(_ context.Context, id string) (mockUser, error) {
			return mockUser{ID: id, Name: "Alice"}, nil
		},
	}
	Resource(app, "/users", svc, WithResourceOnly("GET_BY_ID"))
	app.Compile()

	// GET by ID should work
	req := &mockRequest{method: "GET", path: "/users/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode != 200 {
		t.Errorf("GET /users/abc status = %d, want 200", resp.statusCode)
	}

	// LIST should NOT work
	req = &mockRequest{method: "GET", path: "/users"}
	resp = newMockResponse()
	app.ServeKruda(resp, req)
	if resp.statusCode == 200 {
		t.Error("GET /users should not be registered with GET_BY_ID-only")
	}
}

// =============================================================================
// resource.go — service error paths (list error, create error, update error, delete error)
// =============================================================================

func TestResourceList_Error(t *testing.T) {
	app := New()
	svc := &mockUserService{
		listFn: func(_ context.Context, page, limit int) ([]mockUser, int, error) {
			return nil, 0, errors.New("db error")
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 500 {
		t.Errorf("status = %d, want 500", resp.statusCode)
	}
}

func TestResourceCreate_Error(t *testing.T) {
	app := New()
	svc := &mockUserService{
		createFn: func(_ context.Context, item mockUser) (mockUser, error) {
			return mockUser{}, errors.New("create failed")
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"Alice"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 500 {
		t.Errorf("status = %d, want 500", resp.statusCode)
	}
}

func TestResourceUpdate_Error(t *testing.T) {
	app := New()
	svc := &mockUserService{
		updateFn: func(_ context.Context, id string, item mockUser) (mockUser, error) {
			return mockUser{}, errors.New("update failed")
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

	if resp.statusCode != 500 {
		t.Errorf("status = %d, want 500", resp.statusCode)
	}
}

func TestResourceDelete_Error(t *testing.T) {
	app := New()
	svc := &mockUserService{
		deleteFn: func(_ context.Context, id string) error {
			return errors.New("delete failed")
		},
	}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{method: "DELETE", path: "/users/abc"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 500 {
		t.Errorf("status = %d, want 500", resp.statusCode)
	}
}

func TestResourceUpdate_InvalidID(t *testing.T) {
	app := New()
	svc := &mockItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/items/not-a-number",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{"name":"X"}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Errorf("status = %d, want 400", resp.statusCode)
	}
}

func TestResourceDelete_InvalidID(t *testing.T) {
	app := New()
	svc := &mockItemService{}
	Resource(app, "/items", svc)
	app.Compile()

	req := &mockRequest{method: "DELETE", path: "/items/not-a-number"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Errorf("status = %d, want 400", resp.statusCode)
	}
}

func TestResourceCreate_InvalidBody(t *testing.T) {
	app := New()
	svc := &mockUserService{}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method:  "POST",
		path:    "/users",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{invalid json}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	// Should fail with a bind error
	if resp.statusCode == 201 {
		t.Error("invalid body should not return 201")
	}
}

func TestResourceUpdate_InvalidBody(t *testing.T) {
	app := New()
	svc := &mockUserService{}
	Resource(app, "/users", svc)
	app.Compile()

	req := &mockRequest{
		method:  "PUT",
		path:    "/users/abc",
		headers: map[string]string{"Content-Type": "application/json"},
		body:    []byte(`{invalid json}`),
	}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode == 200 {
		t.Error("invalid body should not return 200")
	}
}

func TestResourceWithCustomIDParam(t *testing.T) {
	app := New()
	var receivedID string
	svc := &mockUserService{
		getFn: func(_ context.Context, id string) (mockUser, error) {
			receivedID = id
			return mockUser{ID: id, Name: "Test"}, nil
		},
	}
	Resource(app, "/users", svc, WithResourceIDParam("userId"))
	app.Compile()

	req := &mockRequest{method: "GET", path: "/users/custom-id-123"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200, body: %s", resp.statusCode, resp.body)
	}
	if receivedID != "custom-id-123" {
		t.Errorf("received id = %q, want custom-id-123", receivedID)
	}
}

// =============================================================================
// resource.go — resourceParseID unsupported type (float64)
// =============================================================================

func TestResourceParseID_UnsupportedType(t *testing.T) {
	_, err := resourceParseID[float64]("3.14")
	if err == nil {
		t.Error("expected error for unsupported ID type float64")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("error = %q, want to contain 'unsupported'", err.Error())
	}
}
