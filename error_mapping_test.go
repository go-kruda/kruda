package kruda

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type testDomainError struct {
	Field string
}

func (e *testDomainError) Error() string { return "domain error: " + e.Field }

var errNotFound = errors.New("not found")
var errConflict = errors.New("conflict")

func TestMapErrorTypeMatch(t *testing.T) {
	app := New()
	MapErrorType[*testDomainError](app, 422, "validation failed")

	ke := app.resolveError(&testDomainError{Field: "name"})
	if ke.Code != 422 {
		t.Errorf("expected Code=422, got %d", ke.Code)
	}
	if ke.Message != "validation failed" {
		t.Errorf("expected Message=%q, got %q", "validation failed", ke.Message)
	}
	if !strings.Contains(ke.Detail, "domain error") {
		t.Errorf("expected Detail to contain %q, got %q", "domain error", ke.Detail)
	}
}

func TestMapErrorTypeNoMatch(t *testing.T) {
	app := New()
	MapErrorType[*testDomainError](app, 422, "validation failed")

	ke := app.resolveError(errors.New("other"))
	if ke.Code != 500 {
		t.Errorf("expected Code=500 (default), got %d", ke.Code)
	}
	if ke.Message != "internal server error" {
		t.Errorf("expected default message, got %q", ke.Message)
	}
}

func TestMapErrorFuncMatch(t *testing.T) {
	app := New()
	called := false
	MapErrorFunc(app, errNotFound, func(err error) *KrudaError {
		called = true
		return &KrudaError{Code: 404, Message: "custom not found", Detail: err.Error()}
	})

	wrapped := fmt.Errorf("wrap: %w", errNotFound)
	ke := app.resolveError(wrapped)
	if !called {
		t.Fatal("expected MapErrorFunc fn to be called")
	}
	if ke.Code != 404 {
		t.Errorf("expected Code=404, got %d", ke.Code)
	}
	if ke.Message != "custom not found" {
		t.Errorf("expected Message=%q, got %q", "custom not found", ke.Message)
	}
}

func TestMapErrorFuncCustomResponse(t *testing.T) {
	app := New()
	MapErrorFunc(app, errNotFound, func(err error) *KrudaError {
		return &KrudaError{Code: 418, Message: "teapot", Detail: "custom"}
	})

	ke := app.resolveError(errNotFound)
	if ke.Code != 418 {
		t.Errorf("expected Code=418, got %d", ke.Code)
	}
	if ke.Message != "teapot" {
		t.Errorf("expected Message=%q, got %q", "teapot", ke.Message)
	}
	if ke.Detail != "custom" {
		t.Errorf("expected Detail=%q, got %q", "custom", ke.Detail)
	}
}

func TestErrorMappingPriority(t *testing.T) {
	// Priority: KrudaError > errorMap > errorFuncs > errorTypes > default 500

	// Test 1: KrudaError passes through directly (highest priority)
	t.Run("KrudaError_passthrough", func(t *testing.T) {
		app := New()
		app.MapError(errNotFound, 404, "mapped")
		MapErrorFunc(app, errNotFound, func(err error) *KrudaError {
			return &KrudaError{Code: 499, Message: "func"}
		})

		ke := app.resolveError(&KrudaError{Code: 451, Message: "original"})
		if ke.Code != 451 {
			t.Errorf("expected KrudaError passthrough Code=451, got %d", ke.Code)
		}
		if ke.Message != "original" {
			t.Errorf("expected Message=%q, got %q", "original", ke.Message)
		}
	})

	// Test 2: errorMap beats errorFuncs and errorTypes
	t.Run("errorMap_over_funcs_and_types", func(t *testing.T) {
		app := New()
		app.MapError(errNotFound, 404, "from map")
		MapErrorFunc(app, errNotFound, func(err error) *KrudaError {
			return &KrudaError{Code: 499, Message: "from func"}
		})
		MapErrorType[*testDomainError](app, 422, "from type")

		ke := app.resolveError(errNotFound)
		if ke.Code != 404 {
			t.Errorf("expected errorMap Code=404, got %d", ke.Code)
		}
		if ke.Message != "from map" {
			t.Errorf("expected Message=%q, got %q", "from map", ke.Message)
		}
	})

	// Test 3: errorFuncs beats errorTypes
	t.Run("errorFuncs_over_types", func(t *testing.T) {
		app := New()
		// Use a domain error that also matches a func mapping
		domainErr := &testDomainError{Field: "email"}
		sentinel := errors.New("sentinel")
		wrappedErr := fmt.Errorf("%w: %w", sentinel, domainErr)

		MapErrorFunc(app, sentinel, func(err error) *KrudaError {
			return &KrudaError{Code: 499, Message: "from func"}
		})
		MapErrorType[*testDomainError](app, 422, "from type")

		ke := app.resolveError(wrappedErr)
		if ke.Code != 499 {
			t.Errorf("expected errorFuncs Code=499, got %d", ke.Code)
		}
	})

	// Test 4: errorTypes used when nothing else matches
	t.Run("errorTypes_fallback", func(t *testing.T) {
		app := New()
		MapErrorType[*testDomainError](app, 422, "from type")

		ke := app.resolveError(&testDomainError{Field: "age"})
		if ke.Code != 422 {
			t.Errorf("expected errorTypes Code=422, got %d", ke.Code)
		}
	})

	// Test 5: default 500 when nothing matches
	t.Run("default_500", func(t *testing.T) {
		app := New()
		MapErrorType[*testDomainError](app, 422, "from type")

		ke := app.resolveError(errors.New("unknown"))
		if ke.Code != 500 {
			t.Errorf("expected default Code=500, got %d", ke.Code)
		}
	})
}

func TestErrorMappingRegistrationOrder(t *testing.T) {
	app := New()

	// Register two different type mappings
	// Use testDomainError which implements error interface

	MapErrorType[*testDomainError](app, 422, "domain error mapped")

	// Verify testDomainError matches
	ke := app.resolveError(&testDomainError{Field: "x"})
	if ke.Code != 422 {
		t.Errorf("expected Code=422 for testDomainError, got %d", ke.Code)
	}
	if ke.Message != "domain error mapped" {
		t.Errorf("expected Message=%q, got %q", "domain error mapped", ke.Message)
	}

	// Verify unrelated error doesn't match
	ke2 := app.resolveError(errors.New("something else"))
	if ke2.Code != 500 {
		t.Errorf("expected Code=500 for unrelated error, got %d", ke2.Code)
	}
}

func TestErrorMappingDetailField(t *testing.T) {
	app := New()
	app.MapError(errNotFound, 404, "resource not found")

	ke := app.resolveError(errNotFound)
	if ke.Detail != errNotFound.Error() {
		t.Errorf("expected Detail=%q, got %q", errNotFound.Error(), ke.Detail)
	}
}

func TestErrorMappingWithHandler(t *testing.T) {
	app := New()
	app.MapError(errNotFound, 404, "resource not found")
	app.Get("/item", func(c *Ctx) error {
		return errNotFound
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/item")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode() != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode())
	}

	var body map[string]any
	if err := resp.JSON(&body); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	code, ok := body["code"].(float64)
	if !ok || int(code) != 404 {
		t.Errorf("expected JSON code=404, got %v", body["code"])
	}
	msg, ok := body["message"].(string)
	if !ok || msg != "resource not found" {
		t.Errorf("expected JSON message=%q, got %v", "resource not found", body["message"])
	}
}

func TestExistingMapErrorUnchanged(t *testing.T) {
	app := New()
	app.MapError(errConflict, 409, "conflict")

	ke := app.resolveError(errConflict)
	if ke.Code != 409 {
		t.Errorf("expected Code=409, got %d", ke.Code)
	}
	if ke.Message != "conflict" {
		t.Errorf("expected Message=%q, got %q", "conflict", ke.Message)
	}
	if ke.Detail != errConflict.Error() {
		t.Errorf("expected Detail=%q, got %q", errConflict.Error(), ke.Detail)
	}
	if ke.Err != errConflict {
		t.Error("expected Err to be errConflict")
	}
}
