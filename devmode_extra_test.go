package kruda

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// --- DevMode flag (env var + explicit option) ---

func TestNew_DevMode_EnvVar(t *testing.T) {
	os.Setenv("KRUDA_ENV", "development")
	defer os.Unsetenv("KRUDA_ENV")

	app := New()
	if !app.config.DevMode {
		t.Error("DevMode should be true when KRUDA_ENV=development")
	}
}

func TestNew_DevMode_Explicit(t *testing.T) {
	app := New(WithDevMode(true))
	if !app.config.DevMode {
		t.Error("DevMode should be true")
	}
}

// --- DevMode generateSuggestions: 405 / 422 / 413 / 500 ---

func TestGenerateSuggestions_405_Direct(t *testing.T) {
	app := New(WithDevMode(true))
	c := newCtx(app)
	c.method = "POST"
	c.path = "/api/data"

	suggestions := generateSuggestions(errors.New("test"), 405, c)
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "doesn't accept") {
			found = true
		}
	}
	if !found {
		t.Errorf("405 suggestions should mention 'doesn't accept', got %v", suggestions)
	}
}

func TestGenerateSuggestions_422(t *testing.T) {
	app := New(WithDevMode(true))
	c := newCtx(app)
	c.method = "POST"
	c.path = "/api/users"

	suggestions := generateSuggestions(errors.New("test"), 422, c)
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "request body") {
			found = true
		}
	}
	if !found {
		t.Errorf("422 suggestions should mention request body, got %v", suggestions)
	}
}

func TestGenerateSuggestions_413(t *testing.T) {
	app := New(WithDevMode(true))
	c := newCtx(app)
	c.method = "POST"
	c.path = "/upload"

	suggestions := generateSuggestions(errors.New("test"), 413, c)
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "maximum size") {
			found = true
		}
	}
	if !found {
		t.Errorf("413 suggestions should mention max size, got %v", suggestions)
	}
}

func TestGenerateSuggestions_500(t *testing.T) {
	app := New(WithDevMode(true))
	c := newCtx(app)
	c.method = "GET"
	c.path = "/fail"

	suggestions := generateSuggestions(errors.New("test"), 500, c)
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "stack trace") {
			found = true
		}
	}
	if !found {
		t.Errorf("500 suggestions should mention stack trace, got %v", suggestions)
	}
}

// --- DevMode buildSourceLines edge cases ---

func TestBuildSourceLines_NearStart(t *testing.T) {
	lines := []string{"line1", "line2", "line3", "line4", "line5"}
	result := buildSourceLines(lines, 1, 2) // target=1, radius=2
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// First line should start at 1
	if result[0].Number != 1 {
		t.Errorf("first line number = %d, want 1", result[0].Number)
	}
	// The error line should be line 1
	foundError := false
	for _, sl := range result {
		if sl.IsError && sl.Number == 1 {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected line 1 to be marked as error")
	}
}

func TestBuildSourceLines_BeyondEnd(t *testing.T) {
	lines := []string{"a", "b", "c"}
	result := buildSourceLines(lines, 3, 10) // radius larger than file
	if len(result) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Should not go beyond len(lines)
	lastLine := result[len(result)-1]
	if lastLine.Number > len(lines) {
		t.Errorf("last line number %d exceeds file length %d", lastLine.Number, len(lines))
	}
}

// --- DevMode collectRequestHeaders / collectQueryParams (nil + provider) ---

func TestCollectRequestHeaders_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.request = nil
	h := collectRequestHeaders(c)
	if len(h) != 0 {
		t.Errorf("expected empty headers for nil request, got %d", len(h))
	}
}

func TestCollectQueryParams_NilRequest(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.request = nil
	q := collectQueryParams(c)
	if len(q) != 0 {
		t.Errorf("expected empty query params for nil request, got %d", len(q))
	}
}

// allHeadersRequest implements AllHeadersProvider and AllQueryProvider.
type allHeadersRequest struct {
	mockRequest
	hdrs  map[string]string
	query map[string]string
}

func (r *allHeadersRequest) AllHeaders() map[string]string { return r.hdrs }
func (r *allHeadersRequest) AllQuery() map[string]string   { return r.query }

func TestCollectRequestHeaders_WithProvider(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.request = &allHeadersRequest{
		mockRequest: mockRequest{method: "GET", path: "/test"},
		hdrs:        map[string]string{"X-Test": "val"},
	}
	h := collectRequestHeaders(c)
	if h["X-Test"] != "val" {
		t.Errorf("expected X-Test=val, got %v", h)
	}
}

func TestCollectQueryParams_WithProvider(t *testing.T) {
	app := New()
	c := newCtx(app)
	c.request = &allHeadersRequest{
		mockRequest: mockRequest{method: "GET", path: "/test"},
		query:       map[string]string{"foo": "bar"},
	}
	q := collectQueryParams(c)
	if q["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", q)
	}
}

// --- DevMode walkRouteTree with params + wildcards ---

func TestWalkRouteTree_WithParams(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/users/:id", func(c *Ctx) error { return c.Text("ok") })
	app.Get("/files/*filepath", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()

	routes := collectDevRoutes(app)
	foundParam := false
	foundWildcard := false
	for _, r := range routes {
		if strings.Contains(r.Path, ":id") {
			foundParam = true
		}
		if strings.Contains(r.Path, "*filepath") {
			foundWildcard = true
		}
	}
	if !foundParam {
		t.Error("routes should include :id param path")
	}
	if !foundWildcard {
		t.Error("routes should include *filepath wildcard path")
	}
}

// --- DevMode env var filtering ---

func TestFilterEnvVars(t *testing.T) {
	vars := filterEnvVars()
	// Should not contain any sensitive keys
	for key := range vars {
		if isSensitiveEnvKey(key) {
			t.Errorf("sensitive key %q should be filtered", key)
		}
	}
}

func TestIsSensitiveEnvKey(t *testing.T) {
	tests := []struct {
		key       string
		sensitive bool
	}{
		{"DB_PASSWORD", true},
		{"API_TOKEN", true},
		{"MY_SECRET", true},
		{"APP_PORT", false},
		{"GOPATH", false},
		{"AWS_CREDENTIAL_FILE", true},
		{"PRIVATE_KEY_PATH", true},
	}
	for _, tt := range tests {
		if got := isSensitiveEnvKey(tt.key); got != tt.sensitive {
			t.Errorf("isSensitiveEnvKey(%q) = %v, want %v", tt.key, got, tt.sensitive)
		}
	}
}

// --- DevMode renderDevErrorPage production gate ---

func TestRenderDevErrorPage_ProductionReturnsNil(t *testing.T) {
	app := New() // DevMode = false
	c := newCtx(app)
	c.method = "GET"
	c.path = "/test"
	result := renderDevErrorPage(c, errors.New("test"), 500)
	if result != nil {
		t.Error("renderDevErrorPage should return nil in production mode")
	}
}

// --- DevMode 405 page via TestClient ---

func TestDevMode_405_ViaTestClient(t *testing.T) {
	app := New(WithDevMode(true))
	app.Get("/only-get", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, err := tc.Request("POST", "/only-get").Send()
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode() != 405 {
		t.Errorf("status = %d, want 405", resp.StatusCode())
	}
	body := resp.BodyString()
	if !strings.Contains(body, "doesn't accept") && !strings.Contains(body, "method not allowed") {
		t.Error("405 dev error page should include method not allowed indication")
	}
}

// --- AllHeadersProvider/AllQueryProvider interface assertions ---

var _ transport.AllHeadersProvider = (*allHeadersRequest)(nil)
var _ transport.AllQueryProvider = (*allHeadersRequest)(nil)
