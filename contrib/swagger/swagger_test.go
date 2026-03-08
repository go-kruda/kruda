package swagger

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kruda/kruda"
)

func TestNew_DefaultConfig(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New())
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, `"/openapi.json"`) {
		t.Error("expected default SpecURL /openapi.json in response body")
	}
	if !strings.Contains(body, "<title>API Documentation</title>") {
		t.Error("expected default title 'API Documentation' in response body")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New(Config{
		SpecURL: "/api/v2/spec.json",
		Title:   "My Custom API",
	}))
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, `"/api/v2/spec.json"`) {
		t.Error("expected custom SpecURL in response body")
	}
	if !strings.Contains(body, "<title>My Custom API</title>") {
		t.Error("expected custom title in response body")
	}
}

func TestNew_DarkMode(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New(Config{DarkMode: true}))
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}

	body := resp.Body.String()
	if !strings.Contains(body, "filter: invert(88%)") {
		t.Error("expected dark mode CSS in response body when DarkMode is true")
	}
	if !strings.Contains(body, "background: #1a1a2e") {
		t.Error("expected dark background style when DarkMode is true")
	}

	// Verify light mode does NOT include dark CSS
	appLight := kruda.New(kruda.NetHTTP())
	appLight.Get("/docs", New())
	appLight.Compile()

	reqLight := httptest.NewRequest("GET", "/docs", nil)
	respLight := httptest.NewRecorder()
	appLight.ServeHTTP(respLight, reqLight)

	bodyLight := respLight.Body.String()
	if strings.Contains(bodyLight, "filter: invert(88%)") {
		t.Error("expected no dark mode CSS when DarkMode is false")
	}
}

func TestNew_HTMLContentType(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New())
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	ct := resp.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Errorf("expected Content-Type to start with text/html, got %s", ct)
	}
}

func TestNew_ContainsSwaggerUI(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New())
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	body := resp.Body.String()

	// Check for Swagger UI bundle JS
	if !strings.Contains(body, "swagger-ui-bundle.js") {
		t.Error("expected swagger-ui-bundle.js reference in response body")
	}

	// Check for Swagger UI CSS
	if !strings.Contains(body, "swagger-ui.css") {
		t.Error("expected swagger-ui.css reference in response body")
	}

	// Check for SwaggerUIBundle initialization
	if !strings.Contains(body, "SwaggerUIBundle(") {
		t.Error("expected SwaggerUIBundle initialization in response body")
	}

	// Check for StandaloneLayout
	if !strings.Contains(body, "StandaloneLayout") {
		t.Error("expected StandaloneLayout in SwaggerUIBundle config")
	}

	// Check for swagger-ui div
	if !strings.Contains(body, `<div id="swagger-ui"></div>`) {
		t.Error("expected swagger-ui div in response body")
	}
}

func TestConfig_Defaults(t *testing.T) {
	var cfg Config
	cfg.defaults()

	if cfg.SpecURL != "/openapi.json" {
		t.Errorf("expected default SpecURL '/openapi.json', got %q", cfg.SpecURL)
	}
	if cfg.Title != "API Documentation" {
		t.Errorf("expected default Title 'API Documentation', got %q", cfg.Title)
	}
	if cfg.DarkMode {
		t.Error("expected default DarkMode to be false")
	}
}

func TestConfig_DefaultsPreservesCustomValues(t *testing.T) {
	cfg := Config{
		SpecURL:  "/custom/spec.json",
		Title:    "Custom Title",
		DarkMode: true,
	}
	cfg.defaults()

	if cfg.SpecURL != "/custom/spec.json" {
		t.Errorf("defaults() should not overwrite custom SpecURL, got %q", cfg.SpecURL)
	}
	if cfg.Title != "Custom Title" {
		t.Errorf("defaults() should not overwrite custom Title, got %q", cfg.Title)
	}
	if !cfg.DarkMode {
		t.Error("defaults() should not overwrite custom DarkMode")
	}
}

func TestNew_HTMLEscaping(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/docs", New(Config{
		Title:   `<script>alert("xss")</script>`,
		SpecURL: `"/><script>alert(1)</script>`,
	}))
	app.Compile()

	req := httptest.NewRequest("GET", "/docs", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	body := resp.Body.String()

	// Ensure raw script tags are NOT present (they should be escaped)
	if strings.Contains(body, `<script>alert("xss")</script>`) {
		t.Error("title should be HTML-escaped to prevent XSS")
	}
	if strings.Contains(body, `"/><script>alert(1)</script>`) {
		t.Error("specURL should be HTML-escaped to prevent XSS")
	}
}
