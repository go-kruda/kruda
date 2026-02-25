package kruda

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 13: Resource Status Codes
// For random page/limit, List→200, Get→200, Create→201, Update→200, Delete→204.
// ---------------------------------------------------------------------------

func TestPropertyResourceStatusCodes(t *testing.T) {
	f := func(page uint8, limit uint8) bool {
		pg := int(page)%100 + 1  // 1-100
		lim := int(limit)%50 + 1 // 1-50

		app := New()
		svc := &mockUserService{
			listFn: func(_ context.Context, p, l int) ([]mockUser, int, error) {
				return []mockUser{{ID: "1", Name: "A"}}, 1, nil
			},
			getFn: func(_ context.Context, id string) (mockUser, error) {
				return mockUser{ID: id, Name: "Test"}, nil
			},
			createFn: func(_ context.Context, item mockUser) (mockUser, error) {
				item.ID = "new"
				return item, nil
			},
			updateFn: func(_ context.Context, id string, item mockUser) (mockUser, error) {
				item.ID = id
				return item, nil
			},
			deleteFn: func(_ context.Context, id string) error {
				return nil
			},
		}
		Resource(app, "/users", svc)
		app.Compile()

		// List → 200
		req := &mockRequest{
			method: "GET",
			path:   "/users",
			query:  map[string]string{"page": fmt.Sprintf("%d", pg), "limit": fmt.Sprintf("%d", lim)},
		}
		resp := newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 200 {
			return false
		}

		// Get → 200
		req = &mockRequest{method: "GET", path: "/users/abc"}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 200 {
			return false
		}

		// Create → 201
		req = &mockRequest{
			method:  "POST",
			path:    "/users",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    []byte(`{"name":"X"}`),
		}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 201 {
			return false
		}

		// Update → 200
		req = &mockRequest{
			method:  "PUT",
			path:    "/users/abc",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    []byte(`{"name":"Y"}`),
		}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 200 {
			return false
		}

		// Delete → 204
		req = &mockRequest{method: "DELETE", path: "/users/abc"}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 204 {
			return false
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 14: Resource List Pagination
// For random page (1-100) and limit (1-50), response JSON has matching
// page/limit fields.
// ---------------------------------------------------------------------------

func TestPropertyResourceListPagination(t *testing.T) {
	f := func(pageRaw uint8, limitRaw uint8) bool {
		pg := int(pageRaw)%100 + 1  // 1-100
		lim := int(limitRaw)%50 + 1 // 1-50

		app := New()
		svc := &mockUserService{
			listFn: func(_ context.Context, p, l int) ([]mockUser, int, error) {
				return []mockUser{}, 0, nil
			},
		}
		Resource(app, "/users", svc)
		app.Compile()

		req := &mockRequest{
			method: "GET",
			path:   "/users",
			query:  map[string]string{"page": fmt.Sprintf("%d", pg), "limit": fmt.Sprintf("%d", lim)},
		}
		resp := newMockResponse()
		app.ServeKruda(resp, req)

		if resp.statusCode != 200 {
			return false
		}

		var body struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		}
		if err := json.Unmarshal(resp.body, &body); err != nil {
			return false
		}
		return body.Page == pg && body.Limit == lim
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 15: Resource Error Passthrough
// When service returns error, status is 500 (non-KrudaError).
// ---------------------------------------------------------------------------

func TestPropertyResourceErrorPassthrough(t *testing.T) {
	f := func(msg string) bool {
		if msg == "" {
			msg = "fail"
		}

		app := New()
		svc := &mockUserService{
			listFn: func(_ context.Context, p, l int) ([]mockUser, int, error) {
				return nil, 0, errors.New(msg)
			},
			getFn: func(_ context.Context, id string) (mockUser, error) {
				return mockUser{}, errors.New(msg)
			},
			createFn: func(_ context.Context, item mockUser) (mockUser, error) {
				return mockUser{}, errors.New(msg)
			},
			updateFn: func(_ context.Context, id string, item mockUser) (mockUser, error) {
				return mockUser{}, errors.New(msg)
			},
			deleteFn: func(_ context.Context, id string) error {
				return errors.New(msg)
			},
		}
		Resource(app, "/users", svc)
		app.Compile()

		// List error → 500
		req := &mockRequest{method: "GET", path: "/users"}
		resp := newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 500 {
			return false
		}

		// Get error → 500
		req = &mockRequest{method: "GET", path: "/users/abc"}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 500 {
			return false
		}

		// Create error → 500
		req = &mockRequest{
			method:  "POST",
			path:    "/users",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    []byte(`{"name":"X"}`),
		}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 500 {
			return false
		}

		// Update error → 500
		req = &mockRequest{
			method:  "PUT",
			path:    "/users/abc",
			headers: map[string]string{"Content-Type": "application/json"},
			body:    []byte(`{"name":"Y"}`),
		}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 500 {
			return false
		}

		// Delete error → 500
		req = &mockRequest{method: "DELETE", path: "/users/abc"}
		resp = newMockResponse()
		app.ServeKruda(resp, req)
		if resp.statusCode != 500 {
			return false
		}

		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 16: Resource Method Filtering
// For random subset of methods via WithResourceOnly, only those methods
// return 200 (or 201/204), others return 404/405.
// ---------------------------------------------------------------------------

func TestPropertyResourceMethodFiltering(t *testing.T) {
	// allMethods maps HTTP method to the expected success status code.
	type methodInfo struct {
		method      string
		successCode int
		path        string
		needsBody   bool
	}
	allMethods := []methodInfo{
		{"GET", 200, "/users", false},        // List
		{"POST", 201, "/users", true},        // Create
		{"PUT", 200, "/users/abc", true},     // Update
		{"DELETE", 204, "/users/abc", false}, // Delete
	}

	// resourceMethodForHTTP maps HTTP method to the method string used in WithResourceOnly.
	// GET covers both List and Get; we test List here.

	f := func(mask uint8) bool {
		// Use lower 4 bits to select which methods to include.
		// Ensure at least one method is selected.
		bits := mask & 0x0F
		if bits == 0 {
			bits = 1 // at least GET
		}

		var only []string
		enabled := make(map[string]bool)
		for i, m := range allMethods {
			if bits&(1<<uint(i)) != 0 {
				only = append(only, m.method)
				enabled[m.method] = true
			}
		}

		app := New()
		svc := &mockUserService{
			listFn: func(_ context.Context, p, l int) ([]mockUser, int, error) {
				return []mockUser{}, 0, nil
			},
			getFn: func(_ context.Context, id string) (mockUser, error) {
				return mockUser{ID: id, Name: "T"}, nil
			},
			createFn: func(_ context.Context, item mockUser) (mockUser, error) {
				item.ID = "new"
				return item, nil
			},
			updateFn: func(_ context.Context, id string, item mockUser) (mockUser, error) {
				item.ID = id
				return item, nil
			},
			deleteFn: func(_ context.Context, id string) error {
				return nil
			},
		}
		Resource(app, "/users", svc, WithResourceOnly(only...))
		app.Compile()

		for _, m := range allMethods {
			var req *mockRequest
			if m.needsBody {
				req = &mockRequest{
					method:  m.method,
					path:    m.path,
					headers: map[string]string{"Content-Type": "application/json"},
					body:    []byte(`{"name":"X"}`),
				}
			} else {
				req = &mockRequest{method: m.method, path: m.path}
			}
			resp := newMockResponse()
			app.ServeKruda(resp, req)

			if enabled[m.method] {
				// Should succeed with expected status code
				if resp.statusCode != m.successCode {
					return false
				}
			} else {
				// Should be rejected: 404 or 405
				if resp.statusCode != 404 && resp.statusCode != 405 {
					return false
				}
			}
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
