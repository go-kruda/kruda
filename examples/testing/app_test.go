// Example: Testing — Actual Test Functions Using TestClient
//
// This file demonstrates how to write tests for a Kruda application
// using the built-in TestClient. No real HTTP server is started —
// everything runs in-memory for fast, parallel-safe tests.
//
// Run: go test -tags kruda_stdjson -v ./examples/testing/
package main

import (
	"testing"

	"github.com/go-kruda/kruda"
)

// ---------------------------------------------------------------------------
// Basic: GET request and status code assertion
// ---------------------------------------------------------------------------

func TestHealthCheck(t *testing.T) {
	// 1. Create the app (same factory used by main)
	app := NewApp()

	// 2. Create a test client — no server needed
	tc := kruda.NewTestClient(app)

	// 3. Send a GET request
	resp, err := tc.Get("/health")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4. Assert status code
	if resp.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode())
	}

	// 5. Parse JSON response
	var body map[string]string
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}
}

// ---------------------------------------------------------------------------
// POST with JSON body
// ---------------------------------------------------------------------------

func TestCreateTodo(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	// Post sends a JSON body when given a struct or map
	resp, err := tc.Post("/todos", map[string]string{
		"title": "Write tests",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode() != 201 {
		t.Errorf("expected 201, got %d", resp.StatusCode())
	}

	var todo Todo
	if err := resp.JSON(&todo); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if todo.Title != "Write tests" {
		t.Errorf("expected title=%q, got %q", "Write tests", todo.Title)
	}
	if todo.ID == "" {
		t.Error("expected non-empty ID")
	}
}

// ---------------------------------------------------------------------------
// Request builder: headers, query params, cookies
// ---------------------------------------------------------------------------

func TestRequestBuilder(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	// Use the fluent Request builder for full control
	resp, err := tc.Request("GET", "/health").
		Header("Accept", "application/json").
		Header("X-Custom", "test-value").
		Send()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode())
	}
}

// ---------------------------------------------------------------------------
// Error responses: 404 and 422
// ---------------------------------------------------------------------------

func TestNotFound(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	resp, err := tc.Get("/todos/999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode())
	}
}

func TestValidationError(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	// Empty title should return 422
	resp, err := tc.Post("/todos", map[string]string{
		"title": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 422 {
		t.Errorf("expected 422, got %d", resp.StatusCode())
	}
}

// ---------------------------------------------------------------------------
// End-to-end: create then retrieve
// ---------------------------------------------------------------------------

func TestCreateAndGet(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	// Create a todo
	createResp, err := tc.Post("/todos", map[string]string{
		"title": "Buy milk",
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}
	var created Todo
	if err := createResp.JSON(&created); err != nil {
		t.Fatalf("parse create response: %v", err)
	}

	// Retrieve it by ID
	getResp, err := tc.Get("/todos/" + created.ID)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if getResp.StatusCode() != 200 {
		t.Errorf("expected 200, got %d", getResp.StatusCode())
	}

	var fetched Todo
	if err := getResp.JSON(&fetched); err != nil {
		t.Fatalf("parse get response: %v", err)
	}
	if fetched.Title != "Buy milk" {
		t.Errorf("expected title=%q, got %q", "Buy milk", fetched.Title)
	}
}

// ---------------------------------------------------------------------------
// Response headers
// ---------------------------------------------------------------------------

func TestResponseHeaders(t *testing.T) {
	app := NewApp()
	tc := kruda.NewTestClient(app)

	resp, err := tc.Post("/todos", map[string]string{"title": "Check headers"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Kruda sets Content-Type for JSON responses
	ct := resp.Header("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header to be set")
	}
}
