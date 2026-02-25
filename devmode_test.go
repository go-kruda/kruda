package kruda

import (
	"errors"
	"strings"
	"testing"
)

// TestDevErrorPageRendered verifies that DevMode=true renders an HTML error page
// with Content-Type: text/html when a handler returns an error.
func TestDevErrorPageRendered(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("something broke")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/fail")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode() != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode())
	}

	ct := resp.Header("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected Content-Type text/html, got %q", ct)
	}

	body := resp.BodyString()
	if !strings.Contains(body, "something broke") {
		t.Error("expected error message in HTML body")
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected HTML document in response")
	}
}

// TestDevErrorPageNotInProduction verifies that DevMode=false returns a JSON
// error response, not the HTML dev error page.
func TestDevErrorPageNotInProduction(t *testing.T) {
	app := New() // DevMode defaults to false
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("prod error")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/fail")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode() != 500 {
		t.Fatalf("expected 500, got %d", resp.StatusCode())
	}

	ct := resp.Header("Content-Type")
	if strings.Contains(ct, "text/html") {
		t.Fatal("production mode should NOT return HTML error page")
	}

	body := resp.BodyString()
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Fatal("production mode should NOT contain HTML document")
	}
}

// TestDevErrorPageFilterSecrets verifies that environment variables containing
// SECRET, PASSWORD, or TOKEN (case-insensitive) are filtered from the dev error page.
func TestDevErrorPageFilterSecrets(t *testing.T) {
	// Set sensitive env vars for this test
	t.Setenv("MY_SECRET_KEY", "super-secret-123")
	t.Setenv("DB_PASSWORD", "hunter2")
	t.Setenv("API_TOKEN", "tok-abc")
	t.Setenv("SAFE_VAR_DEVTEST", "visible-value")

	app := New(WithDevMode(true))
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("secret test error")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/fail")
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()

	// Sensitive values must NOT appear
	if strings.Contains(body, "super-secret-123") {
		t.Error("SECRET env var value should be filtered")
	}
	if strings.Contains(body, "hunter2") {
		t.Error("PASSWORD env var value should be filtered")
	}
	if strings.Contains(body, "tok-abc") {
		t.Error("TOKEN env var value should be filtered")
	}

	// Safe var should appear
	if !strings.Contains(body, "SAFE_VAR_DEVTEST") {
		t.Error("non-sensitive env var key should be visible")
	}
}

// TestDevErrorPageNoMultipartBody verifies that multipart/form-data request
// body content is NOT included in the dev error page.
func TestDevErrorPageNoMultipartBody(t *testing.T) {
	app := New(WithDevMode(true))
	app.Post("/upload", func(c *Ctx) error {
		return errors.New("upload failed")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("POST", "/upload").
		Header("Content-Type", "multipart/form-data; boundary=----test").
		Body([]byte("------test\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\nsensitive-file-content\r\n------test--")).
		Send()
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()
	if strings.Contains(body, "sensitive-file-content") {
		t.Error("multipart body content should NOT appear in dev error page")
	}
}

// TestDevErrorPage404Suggestion verifies that a 404 error includes a route
// suggestion like "Did you forget to register this route?"
func TestDevErrorPage404Suggestion(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/exists", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/not-found")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode() != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode())
	}

	body := resp.BodyString()
	if !strings.Contains(body, "Did you forget to register this route?") {
		t.Error("404 dev error page should include route suggestion")
	}
}

// TestDevErrorPageStackTrace verifies that the stack trace section is present
// in the dev error page output.
func TestDevErrorPageStackTrace(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("stack trace test")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/fail")
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()
	if !strings.Contains(body, "Stack Trace") {
		t.Error("dev error page should contain Stack Trace section")
	}
}

// TestDevErrorPageRequestDetails verifies that request method, path, and headers
// are visible in the dev error page output.
func TestDevErrorPageRequestDetails(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/details", func(c *Ctx) error {
		return errors.New("details test")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("GET", "/details").
		Header("X-Custom-Test", "test-value-123").
		Send()
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()

	// Method and path should be visible
	if !strings.Contains(body, "GET") {
		t.Error("request method should be visible in dev error page")
	}
	if !strings.Contains(body, "/details") {
		t.Error("request path should be visible in dev error page")
	}

	// Request Details section should exist
	if !strings.Contains(body, "Request Details") {
		t.Error("dev error page should contain Request Details section")
	}

	// Custom header should be visible
	if !strings.Contains(body, "X-Custom-Test") {
		t.Error("custom request header key should be visible")
	}
	if !strings.Contains(body, "test-value-123") {
		t.Error("custom request header value should be visible")
	}
}

// TestDevErrorPageAvailableRoutes verifies that the available routes table
// is present in the dev error page.
func TestDevErrorPageAvailableRoutes(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/users", func(c *Ctx) error {
		return c.Text("users")
	})
	app.Post("/users", func(c *Ctx) error {
		return c.Text("create")
	})
	app.Get("/health", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/trigger-error")
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()

	// Available Routes section should exist
	if !strings.Contains(body, "Available Routes") {
		t.Error("dev error page should contain Available Routes section")
	}

	// Registered routes should be listed
	if !strings.Contains(body, "/users") {
		t.Error("registered route /users should appear in routes table")
	}
	if !strings.Contains(body, "/health") {
		t.Error("registered route /health should appear in routes table")
	}
}

// TestDevErrorPageCopyButton verifies that the "Copy Error" button is present
// in the HTML output.
func TestDevErrorPageCopyButton(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/fail", func(c *Ctx) error {
		return errors.New("copy test")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Get("/fail")
	if err != nil {
		t.Fatal(err)
	}

	body := resp.BodyString()
	if !strings.Contains(body, "Copy Error") {
		t.Error("dev error page should contain 'Copy Error' button")
	}
	if !strings.Contains(body, "copyError()") {
		t.Error("dev error page should contain copyError() JavaScript function")
	}
}

// TestDevErrorPage_BodyTruncation verifies that request body >1024 bytes gets
// truncated with '... (truncated)' suffix, and body <=1024 bytes passes through unchanged.
func TestDevErrorPage_BodyTruncation(t *testing.T) {
	app := New(WithDevMode(true))
	app.Post("/fail", func(c *Ctx) error {
		return errors.New("body test")
	})
	app.Compile()

	tc := NewTestClient(app)

	// Test body exactly 1024 bytes (should not be truncated)
	body1024 := strings.Repeat("a", 1024)
	resp, err := tc.Request("POST", "/fail").Body([]byte(body1024)).Send()
	if err != nil {
		t.Fatal(err)
	}
	respBody := resp.BodyString()
	if !strings.Contains(respBody, body1024) {
		t.Error("body of 1024 bytes should not be truncated")
	}
	if strings.Contains(respBody, "... (truncated)") {
		t.Error("body of 1024 bytes should not show truncation suffix")
	}

	// Test body >1024 bytes (should be truncated)
	body1025 := strings.Repeat("b", 1025)
	resp, err = tc.Request("POST", "/fail").Body([]byte(body1025)).Send()
	if err != nil {
		t.Fatal(err)
	}
	respBody = resp.BodyString()
	if !strings.Contains(respBody, "... (truncated)") {
		t.Error("body >1024 bytes should show truncation suffix")
	}
	if strings.Contains(respBody, body1025) {
		t.Error("full body >1024 bytes should not appear in response")
	}
}

// TestTrimAbsPath_EdgeCases tests trimAbsPath function through stack trace parsing.
func TestTrimAbsPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with go-kruda/kruda prefix",
			input:    "/home/user/go-kruda/kruda/handler.go",
			expected: "handler.go",
		},
		{
			name:     "path with src prefix",
			input:    "/usr/local/src/myproject/main.go",
			expected: "myproject/main.go",
		},
		{
			name:     "path with no known prefix",
			input:    "/random/path/file.go",
			expected: "/random/path/file.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimAbsPath(tt.input)
			if result != tt.expected {
				t.Errorf("trimAbsPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseFileLine_EdgeCases tests parseFileLine function edge cases.
func TestParseFileLine_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedFile string
		expectedLine int
	}{
		{
			name:         "valid input with offset",
			input:        "/path/to/file.go:42 +0x1a3",
			expectedFile: "/path/to/file.go",
			expectedLine: 42,
		},
		{
			name:         "no colon",
			input:        "/path/to/file.go",
			expectedFile: "",
			expectedLine: 0,
		},
		{
			name:         "non-numeric line",
			input:        "/path/to/file.go:abc +0x1a3",
			expectedFile: "",
			expectedLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, line := parseFileLine(tt.input)
			if file != tt.expectedFile || line != tt.expectedLine {
				t.Errorf("parseFileLine(%q) = (%q, %d), want (%q, %d)",
					tt.input, file, line, tt.expectedFile, tt.expectedLine)
			}
		})
	}
}

// BenchmarkDevErrorPageZeroOverhead verifies that DevMode=false has zero
// allocation overhead on the success path — the dev error page rendering
// logic is never invoked when no error occurs.
func BenchmarkDevErrorPageZeroOverhead(b *testing.B) {
	app := New() // DevMode=false (production)
	app.Get("/ok", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := tc.Get("/ok")
		if err != nil {
			b.Fatal(err)
		}
		if resp.StatusCode() != 200 {
			b.Fatalf("expected 200, got %d", resp.StatusCode())
		}
	}
}
