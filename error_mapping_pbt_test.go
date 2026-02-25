package kruda

import (
	"errors"
	"strings"
	"testing"
	"testing/quick"
)

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 21: Error Value Mapping Format
// For random status codes (100-599) and message strings (alphanumeric 1-30
// chars), register MapError for a sentinel error, make handler return that
// error via TestClient, verify JSON response has code, message, and detail
// fields matching.
// ---------------------------------------------------------------------------

func TestPropertyErrorValueMappingFormat(t *testing.T) {
	f := func(statusCode uint16, message string) bool {
		code := int(statusCode)%500 + 100 // 100-599
		if !isAlphanumRange(message, 1, 30) {
			return true // skip
		}

		sentinel := errors.New("test-sentinel")
		app := New()
		app.MapError(sentinel, code, message)
		app.Get("/err", func(c *Ctx) error { return sentinel })
		app.Compile()

		tc := NewTestClient(app)
		resp, err := tc.Get("/err")
		if err != nil {
			return false
		}
		if resp.StatusCode() != code {
			return false
		}

		var body map[string]any
		if resp.JSON(&body) != nil {
			return false
		}
		if int(body["code"].(float64)) != code {
			return false
		}
		if body["message"] != message {
			return false
		}
		if body["detail"] != sentinel.Error() {
			return false
		}
		return true
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 22: Error Type Mapping
// Define a custom error type. For random field values, register
// MapErrorType, verify resolveError matches.
// ---------------------------------------------------------------------------

type pbtTypedError struct{ Val string }

func (e *pbtTypedError) Error() string { return "typed: " + e.Val }

func TestPropertyErrorTypeMapping(t *testing.T) {
	f := func(val string) bool {
		if !isAlphanumRange(val, 1, 20) {
			return true // skip
		}
		app := New()
		MapErrorType[*pbtTypedError](app, 422, "type matched")
		ke := app.resolveError(&pbtTypedError{Val: val})
		return ke.Code == 422 && ke.Message == "type matched" && strings.Contains(ke.Detail, val)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 23: Error Mapping Order
// Register MapError for a sentinel with status A, then MapErrorFunc for
// same sentinel with status B. Verify resolveError returns status A
// (errorMap wins over errorFuncs).
// ---------------------------------------------------------------------------

func TestPropertyErrorMappingOrder(t *testing.T) {
	f := func(a, b uint16) bool {
		codeA := int(a)%500 + 100
		codeB := int(b)%500 + 100
		if codeA == codeB {
			return true // skip ambiguous
		}

		sentinel := errors.New("order-test")
		app := New()
		app.MapError(sentinel, codeA, "from map")
		MapErrorFunc(app, sentinel, func(err error) *KrudaError {
			return &KrudaError{Code: codeB, Message: "from func"}
		})
		ke := app.resolveError(sentinel)
		return ke.Code == codeA
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// ---------------------------------------------------------------------------
// Feature: phase4-ecosystem, Property 24: MapErrorFunc Transformation
// For random status codes and messages, MapErrorFunc with custom fn,
// verify resolveError returns exactly what fn returns.
// ---------------------------------------------------------------------------

func TestPropertyMapErrorFuncTransformation(t *testing.T) {
	f := func(statusCode uint16, message string) bool {
		code := int(statusCode)%500 + 100
		if !isAlphanumRange(message, 1, 30) {
			return true // skip
		}

		sentinel := errors.New("func-test")
		app := New()
		MapErrorFunc(app, sentinel, func(err error) *KrudaError {
			return &KrudaError{Code: code, Message: message, Detail: "custom"}
		})
		ke := app.resolveError(sentinel)
		return ke.Code == code && ke.Message == message && ke.Detail == "custom"
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
