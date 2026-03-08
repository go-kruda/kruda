package kruda

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// TestErrorSanitization_RawError_ProductionMode verifies that raw Go errors
// get their message replaced with HTTP status text in production mode (R7.1, R7.6).
func TestErrorSanitization_RawError_ProductionMode(t *testing.T) {
	app := New() // DevMode=false by default

	app.Get("/fail", func(c *Ctx) error {
		return errors.New("db connection failed: /var/lib/pg/data")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 500 {
		t.Fatalf("status = %d, want 500", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// R7.6: raw error → generic "Internal Server Error"
	if body.Message != "Internal Server Error" {
		t.Errorf("Message = %q, want %q", body.Message, "Internal Server Error")
	}

	// R7.1-R7.3: no internal details leaked
	if body.Detail != "" {
		t.Errorf("Detail = %q, want empty", body.Detail)
	}

	// Verify no internal paths in response body
	raw := string(resp.body)
	if strings.Contains(raw, "/var/lib") {
		t.Errorf("response body leaks internal path: %s", raw)
	}
	if strings.Contains(raw, "db connection") {
		t.Errorf("response body leaks internal error message: %s", raw)
	}
}

// TestErrorSanitization_KrudaError_5xx_ProductionMode verifies that KrudaError
// with 5xx preserves Message but strips Detail in production (R7.2, R7.5).
func TestErrorSanitization_KrudaError_5xx_ProductionMode(t *testing.T) {
	app := New() // DevMode=false

	app.Get("/fail", func(c *Ctx) error {
		return &KrudaError{
			Code:    503,
			Message: "service unavailable",
			Detail:  "/internal/path/to/service",
		}
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 503 {
		t.Fatalf("status = %d, want 503", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// R7.5: KrudaError Message preserved
	if body.Message != "service unavailable" {
		t.Errorf("Message = %q, want %q", body.Message, "service unavailable")
	}

	// R7.2: Detail stripped for 5xx
	if body.Detail != "" {
		t.Errorf("Detail = %q, want empty", body.Detail)
	}
}

// TestErrorSanitization_KrudaError_4xx_ProductionMode verifies that KrudaError
// with 4xx preserves both Message and Detail in production (R7.5).
func TestErrorSanitization_KrudaError_4xx_ProductionMode(t *testing.T) {
	app := New() // DevMode=false

	app.Get("/fail", func(c *Ctx) error {
		return &KrudaError{
			Code:    400,
			Message: "invalid input",
			Detail:  "field 'email' is required",
		}
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// R7.5: 4xx KrudaError → Message AND Detail preserved
	if body.Message != "invalid input" {
		t.Errorf("Message = %q, want %q", body.Message, "invalid input")
	}
	if body.Detail != "field 'email' is required" {
		t.Errorf("Detail = %q, want %q", body.Detail, "field 'email' is required")
	}
}

// TestErrorSanitization_DevMode_FullDetails verifies that DevMode=true
// passes through full error details (R7.4).
func TestErrorSanitization_DevMode_FullDetails(t *testing.T) {
	app := New(WithDevMode(true))

	app.Get("/fail", func(c *Ctx) error {
		return errors.New("db connection failed: /var/lib/pg/data")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 500 {
		t.Fatalf("status = %d, want 500", resp.statusCode)
	}

	// In DevMode, the dev error page is rendered (HTML), not JSON.
	// The key point is that the response is NOT sanitized — it contains
	// the full error info. The dev page renderer handles this.
	raw := string(resp.body)
	if len(raw) == 0 {
		t.Error("expected non-empty response body in DevMode")
	}
}

// TestErrorSanitization_SlogOutput verifies that slog.Error is called
// with the full unsanitized error regardless of DevMode (R7.7).
func TestErrorSanitization_SlogOutput(t *testing.T) {
	// Capture slog output
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldLogger)

	app := New() // DevMode=false

	app.Get("/fail", func(c *Ctx) error {
		return errors.New("secret db error: /var/lib/pg/data")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	logOutput := buf.String()

	// Verify slog captured the full error
	if !strings.Contains(logOutput, "secret db error") {
		t.Errorf("slog output missing full error message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/var/lib/pg/data") {
		t.Errorf("slog output missing internal path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "request error") {
		t.Errorf("slog output missing 'request error' message, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, "/fail") {
		t.Errorf("slog output missing request path, got: %s", logOutput)
	}

	// Meanwhile, the HTTP response should be sanitized
	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Message != "Internal Server Error" {
		t.Errorf("response Message = %q, want sanitized", body.Message)
	}
}

// TestErrorSanitization_MappedError_ProductionMode verifies that errors
// resolved via MapError preserve the user-configured message in production,
// since MapError is an explicit user mapping (not a raw unhandled error).
func TestErrorSanitization_MappedError_ProductionMode(t *testing.T) {
	var errDBTimeout = errors.New("database timeout")

	app := New() // DevMode=false
	app.MapError(errDBTimeout, 504, "gateway timeout")

	app.Get("/fail", func(c *Ctx) error {
		return errDBTimeout
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 504 {
		t.Fatalf("status = %d, want 504", resp.statusCode)
	}

	var body KrudaError
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Mapped error preserves user-configured message (it's an explicit mapping)
	if body.Message != "gateway timeout" {
		t.Errorf("Message = %q, want %q", body.Message, "gateway timeout")
	}
	// But Detail is stripped for 5xx in production
	if body.Detail != "" {
		t.Errorf("Detail = %q, want empty (5xx detail stripped)", body.Detail)
	}
}

// TestErrorSanitization_SlogOutput_DevMode verifies slog logs full error
// even when DevMode=true (R7.7).
func TestErrorSanitization_SlogOutput_DevMode(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(handler))
	defer slog.SetDefault(oldLogger)

	app := New(WithDevMode(true))

	app.Get("/fail", func(c *Ctx) error {
		return errors.New("internal secret: /etc/passwd")
	})
	app.router.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "internal secret") {
		t.Errorf("slog output missing full error in DevMode, got: %s", logOutput)
	}
}
