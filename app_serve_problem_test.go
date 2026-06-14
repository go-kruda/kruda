package kruda

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveOne(t *testing.T, app *App, method, path string) (int, string, string) {
	t.Helper()
	app.Compile()
	r := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, r)
	res := w.Result()
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header.Get("Content-Type"), string(body)
}

func TestHandleError_problemJSON_kruda(t *testing.T) {
	app := New(WithProblemJSON())
	app.Get("/x", func(c *Ctx) error {
		return NotFound("user not found").WithType("https://e/nf").With("userId", "42")
	})
	status, ctype, body := serveOne(t, app, "GET", "/x")
	if status != 404 {
		t.Fatalf("status=%d", status)
	}
	if ctype != "application/problem+json; charset=utf-8" {
		t.Fatalf("ctype=%q", ctype)
	}
	for _, want := range []string{`"type":"https://e/nf"`, `"status":404`, `"userId":"42"`, `"instance":"/x"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
}

func TestHandleError_problemJSON_validation(t *testing.T) {
	app := New(WithProblemJSON())
	app.Get("/v", func(c *Ctx) error {
		return &ValidationError{Errors: []FieldError{{Field: "email", Rule: "required", Message: "email is required"}}}
	})
	status, ctype, body := serveOne(t, app, "GET", "/v")
	if status != 422 || ctype != "application/problem+json; charset=utf-8" {
		t.Fatalf("status=%d ctype=%q", status, ctype)
	}
	for _, want := range []string{`"title":"Validation failed"`, `"errors":[`, `"field":"email"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %s missing %s", body, want)
		}
	}
}

func TestHandleError_default_unchanged(t *testing.T) {
	app := New()
	app.Get("/x", func(c *Ctx) error { return NotFound("user not found") })
	_, ctype, _ := serveOne(t, app, "GET", "/x")
	if strings.Contains(ctype, "problem+json") {
		t.Fatalf("default path must not emit problem+json, got %q", ctype)
	}
}

func TestHandleError_customHandlerWins(t *testing.T) {
	app := New(WithProblemJSON(), WithErrorHandler(func(c *Ctx, e *KrudaError) {
		_ = c.Status(e.Code).JSON(Map{"custom": e.Message})
	}))
	app.Get("/x", func(c *Ctx) error { return NotFound("nope") })
	_, ctype, body := serveOne(t, app, "GET", "/x")
	if strings.Contains(ctype, "problem+json") || !strings.Contains(body, `"custom"`) {
		t.Fatalf("custom handler must win: ctype=%q body=%s", ctype, body)
	}
}
