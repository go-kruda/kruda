package session

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
)

// --- Mock transport types ---

// mockRequest implements transport.Request for testing with cookie support.
type mockRequest struct {
	method  string
	path    string
	headers map[string]string
	cookies map[string]string
	body    []byte
}

func (r *mockRequest) Method() string               { return r.method }
func (r *mockRequest) Path() string                 { return r.path }
func (r *mockRequest) Header(key string) string     { return r.headers[key] }
func (r *mockRequest) Body() ([]byte, error)        { return r.body, nil }
func (r *mockRequest) QueryParam(key string) string { return "" }
func (r *mockRequest) RemoteAddr() string           { return "127.0.0.1" }
func (r *mockRequest) RawRequest() any              { return nil }
func (r *mockRequest) Context() context.Context     { return context.Background() }

func (r *mockRequest) Cookie(name string) string {
	if r.cookies != nil {
		return r.cookies[name]
	}
	return ""
}

func (r *mockRequest) MultipartForm(int64) (*multipart.Form, error) {
	return nil, fmt.Errorf("not supported")
}

// Compile-time interface check.
var _ transport.Request = (*mockRequest)(nil)

// mockResponseWriter implements transport.ResponseWriter for testing.
type mockResponseWriter struct {
	statusCode int
	headers    *mockHeaderMap
	body       []byte
}

func newMockResponse() *mockResponseWriter {
	return &mockResponseWriter{
		statusCode: 200,
		headers:    &mockHeaderMap{h: make(map[string]string)},
	}
}

func (w *mockResponseWriter) WriteHeader(code int)        { w.statusCode = code }
func (w *mockResponseWriter) Header() transport.HeaderMap { return w.headers }
func (w *mockResponseWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

// Compile-time interface check.
var _ transport.ResponseWriter = (*mockResponseWriter)(nil)

// mockHeaderMap implements transport.HeaderMap for testing.
type mockHeaderMap struct {
	h map[string]string
}

func (m *mockHeaderMap) Set(key, value string) { m.h[http.CanonicalHeaderKey(key)] = value }
func (m *mockHeaderMap) Get(key string) string { return m.h[http.CanonicalHeaderKey(key)] }
func (m *mockHeaderMap) Add(key, value string) {
	key = http.CanonicalHeaderKey(key)
	if existing := m.h[key]; existing != "" {
		m.h[key] = existing + ", " + value
	} else {
		m.h[key] = value
	}
}
func (m *mockHeaderMap) Del(key string) { delete(m.h, http.CanonicalHeaderKey(key)) }

// Compile-time interface check.
var _ transport.HeaderMap = (*mockHeaderMap)(nil)

// --- Test helpers ---

// okBody is the lazy-send response body used by tests.
// Session middleware sets cookies AFTER c.Next() returns, so handlers must use
// lazy-send (c.SetBody) rather than eager-send (c.JSON) to ensure cookies are
// included in the writeHeaders flush.
var okBody = []byte(`{"ok":true}`)

func newReq(method, path string, cookies map[string]string) *mockRequest {
	if cookies == nil {
		cookies = make(map[string]string)
	}
	return &mockRequest{
		method:  method,
		path:    path,
		headers: make(map[string]string),
		cookies: cookies,
	}
}

// extractSetCookie parses a Set-Cookie header to find a specific cookie value.
func extractSetCookie(setCookie, name string) string {
	for _, part := range strings.Split(setCookie, ", ") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name+"=") {
			value := strings.TrimPrefix(part, name+"=")
			if idx := strings.Index(value, ";"); idx >= 0 {
				value = value[:idx]
			}
			return value
		}
	}
	return ""
}

// --- Tests ---

func TestSession_NewSessionCreated(t *testing.T) {
	var sess *Session
	handler := func(c *kruda.Ctx) error {
		sess = GetSession(c)
		sess.Set("hello", "world")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	if sess == nil {
		t.Fatal("expected session to be set in context")
	}

	if !sess.IsNew() {
		t.Fatal("expected IsNew() to be true for first request")
	}

	// Set-Cookie header should be present with default cookie name.
	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}
	cookieValue := extractSetCookie(setCookie, "_session")
	if cookieValue == "" {
		t.Fatal("expected _session cookie in Set-Cookie header")
	}
	// Session ID should be 64 hex characters (32 bytes).
	if len(cookieValue) != 64 {
		t.Fatalf("expected 64 char session ID, got %d: %q", len(cookieValue), cookieValue)
	}
}

func TestSession_DataPersistsAcrossRequests(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()

	cfg := Config{
		Store: store,
	}

	var sessionID string

	// First request: set data.
	setHandler := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("user", "alice")
		sess.Set("count", 42)
		sessionID = sess.ID()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", setHandler)
	app.Post("/test", setHandler)

	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/test", nil))

	if sessionID == "" {
		t.Fatal("expected session ID to be set")
	}

	// Second request: read data using the same session cookie.
	var gotUser any
	var gotCount any
	var isNew bool
	readHandler := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		gotUser = sess.Get("user")
		gotCount = sess.Get("count")
		isNew = sess.IsNew()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app2 := kruda.New()
	app2.Use(New(cfg))
	app2.Get("/test", readHandler)

	resp2 := newMockResponse()
	app2.ServeKruda(resp2, newReq("GET", "/test", map[string]string{
		"_session": sessionID,
	}))

	if resp2.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.statusCode)
	}
	if isNew {
		t.Fatal("expected IsNew() to be false on second request")
	}
	if gotUser != "alice" {
		t.Fatalf("expected user=alice, got %v", gotUser)
	}
	if gotCount != 42 {
		t.Fatalf("expected count=42, got %v", gotCount)
	}
}

func TestSession_SetGetDelete(t *testing.T) {
	var sess *Session
	handler := func(c *kruda.Ctx) error {
		sess = GetSession(c)

		// Set values.
		sess.Set("a", 1)
		sess.Set("b", "two")
		sess.Set("c", true)

		// Get values.
		if sess.Get("a") != 1 {
			t.Fatal("expected a=1")
		}
		if sess.Get("b") != "two" {
			t.Fatal("expected b=two")
		}
		if sess.Get("c") != true {
			t.Fatal("expected c=true")
		}

		// Delete a key.
		sess.Delete("b")
		if sess.Get("b") != nil {
			t.Fatal("expected b to be nil after delete")
		}

		// Other keys should still exist.
		if sess.Get("a") != 1 {
			t.Fatal("expected a=1 after deleting b")
		}

		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

func TestSession_Clear(t *testing.T) {
	var sess *Session
	handler := func(c *kruda.Ctx) error {
		sess = GetSession(c)
		sess.Set("a", 1)
		sess.Set("b", 2)
		sess.Set("c", 3)

		sess.Clear()

		if sess.Get("a") != nil {
			t.Fatal("expected a to be nil after Clear")
		}
		if sess.Get("b") != nil {
			t.Fatal("expected b to be nil after Clear")
		}
		if sess.Get("c") != nil {
			t.Fatal("expected c to be nil after Clear")
		}

		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

func TestSession_GetString(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		sess := GetSession(c)

		// GetString with no value set.
		if v := sess.GetString("missing"); v != "" {
			t.Fatalf("expected empty string, got %q", v)
		}

		// GetString with default.
		if v := sess.GetString("missing", "default"); v != "default" {
			t.Fatalf("expected 'default', got %q", v)
		}

		// GetString with actual string value.
		sess.Set("name", "alice")
		if v := sess.GetString("name"); v != "alice" {
			t.Fatalf("expected 'alice', got %q", v)
		}

		// GetString with non-string value should return default.
		sess.Set("count", 42)
		if v := sess.GetString("count", "fallback"); v != "fallback" {
			t.Fatalf("expected 'fallback' for non-string value, got %q", v)
		}

		// GetString with non-string value and no default.
		if v := sess.GetString("count"); v != "" {
			t.Fatalf("expected empty string for non-string value without default, got %q", v)
		}

		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

func TestSession_GetInt(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		sess := GetSession(c)

		// GetInt with no value set.
		if v := sess.GetInt("missing"); v != 0 {
			t.Fatalf("expected 0, got %d", v)
		}

		// GetInt with default.
		if v := sess.GetInt("missing", 99); v != 99 {
			t.Fatalf("expected 99, got %d", v)
		}

		// GetInt with actual int value.
		sess.Set("count", 42)
		if v := sess.GetInt("count"); v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}

		// GetInt with non-int value should return default.
		sess.Set("name", "alice")
		if v := sess.GetInt("name", 100); v != 100 {
			t.Fatalf("expected 100 for non-int value, got %d", v)
		}

		// GetInt with non-int value and no default.
		if v := sess.GetInt("name"); v != 0 {
			t.Fatalf("expected 0 for non-int value without default, got %d", v)
		}

		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

func TestSession_IsNew(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	cfg := Config{Store: store}

	var firstIsNew bool
	var firstSessionID string
	handler := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		firstIsNew = sess.IsNew()
		firstSessionID = sess.ID()
		sess.Set("visited", true)
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if !firstIsNew {
		t.Fatal("expected IsNew() = true on first request")
	}

	// Second request with the session cookie.
	var secondIsNew bool
	handler2 := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		secondIsNew = sess.IsNew()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app2 := kruda.New()
	app2.Use(New(cfg))
	app2.Get("/test", handler2)

	resp2 := newMockResponse()
	app2.ServeKruda(resp2, newReq("GET", "/test", map[string]string{
		"_session": firstSessionID,
	}))

	if secondIsNew {
		t.Fatal("expected IsNew() = false on second request with existing session")
	}
}

func TestSession_Destroy(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	cfg := Config{Store: store}

	var sessionID string

	// First request: create session with data.
	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("user", "alice")
		sessionID = sess.ID()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if sessionID == "" {
		t.Fatal("expected session ID")
	}

	// Second request: destroy the session.
	app2 := kruda.New()
	app2.Use(New(cfg))
	app2.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Destroy()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp2 := newMockResponse()
	app2.ServeKruda(resp2, newReq("GET", "/test", map[string]string{
		"_session": sessionID,
	}))

	// Set-Cookie header should expire the cookie (Max-Age=0).
	setCookie := resp2.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header to expire cookie")
	}
	if !strings.Contains(setCookie, "Max-Age=0") {
		t.Fatalf("expected Max-Age=0 to expire cookie, got %q", setCookie)
	}

	// The session should be removed from the store.
	data, err := store.Get(sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Fatal("expected session data to be deleted from store")
	}

	// Third request: using the old session ID should create a new session.
	var thirdIsNew bool
	app3 := kruda.New()
	app3.Use(New(cfg))
	app3.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		thirdIsNew = sess.IsNew()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp3 := newMockResponse()
	app3.ServeKruda(resp3, newReq("GET", "/test", map[string]string{
		"_session": sessionID,
	}))

	if !thirdIsNew {
		t.Fatal("expected IsNew() = true after session was destroyed")
	}
}

func TestSession_SkipFunction(t *testing.T) {
	cfg := Config{
		Skip: func(method, path string) bool {
			return path == "/health"
		},
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/health", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if sess != nil {
			t.Fatal("expected no session when skipped")
		}
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if sess == nil {
			t.Fatal("expected session for non-skipped path")
		}
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	// Skipped path: no session, no Set-Cookie.
	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/health", nil))

	if resp1.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp1.statusCode)
	}
	setCookie1 := resp1.headers.Get("Set-Cookie")
	if setCookie1 != "" {
		t.Fatalf("expected no Set-Cookie for skipped path, got %q", setCookie1)
	}

	// Non-skipped path: session should be created.
	resp2 := newMockResponse()
	app.ServeKruda(resp2, newReq("GET", "/test", nil))

	if resp2.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp2.statusCode)
	}
	setCookie2 := resp2.headers.Get("Set-Cookie")
	if setCookie2 == "" {
		t.Fatal("expected Set-Cookie for non-skipped path")
	}
}

func TestSession_CustomCookieName(t *testing.T) {
	cfg := Config{
		CookieName: "my_session",
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("key", "value")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}
	cookieValue := extractSetCookie(setCookie, "my_session")
	if cookieValue == "" {
		t.Fatalf("expected my_session cookie, got %q", setCookie)
	}
}

func TestSession_CustomCookiePath(t *testing.T) {
	cfg := Config{
		CookiePath: "/app",
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("key", "value")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	setCookie := resp.headers.Get("Set-Cookie")
	if !strings.Contains(setCookie, "Path=/app") {
		t.Fatalf("expected Path=/app in Set-Cookie, got %q", setCookie)
	}
}

func TestSession_InvalidSessionID_CreatesNew(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	cfg := Config{Store: store}

	var isNew bool
	var sessID string

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		isNew = sess.IsNew()
		sessID = sess.ID()
		sess.Set("created", true)
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	// Send request with an invalid/non-existent session ID.
	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", map[string]string{
		"_session": "this-is-not-a-valid-session-id-in-store",
	}))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	if !isNew {
		t.Fatal("expected IsNew() = true for invalid session ID")
	}

	// A new session ID should have been generated (not the invalid one).
	if sessID == "this-is-not-a-valid-session-id-in-store" {
		t.Fatal("expected a new session ID to be generated, not the invalid one")
	}

	// Set-Cookie should be set with the new session ID.
	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header for new session")
	}
	cookieValue := extractSetCookie(setCookie, "_session")
	if cookieValue != sessID {
		t.Fatalf("expected Set-Cookie session ID to match %q, got %q", sessID, cookieValue)
	}
}

func TestSession_GetNilSession(t *testing.T) {
	// When no middleware is active, GetSession should return nil.
	app := kruda.New()
	var sess *Session
	app.Get("/test", func(c *kruda.Ctx) error {
		sess = GetSession(c)
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if sess != nil {
		t.Fatal("expected nil session when middleware is not active")
	}
}

func TestSession_GetReturnsNilForMissingKey(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		sess := GetSession(c)
		if v := sess.Get("nonexistent"); v != nil {
			t.Fatalf("expected nil for missing key, got %v", v)
		}
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	}

	app := kruda.New()
	app.Use(New())
	app.Get("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}
}

func TestSession_UnmodifiedSessionNoCookie(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	cfg := Config{Store: store}

	// First request: create session with data.
	var sessionID string
	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("key", "value")
		sessionID = sess.ID()
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp1 := newMockResponse()
	app.ServeKruda(resp1, newReq("GET", "/test", nil))

	// Second request: don't modify session.
	app2 := kruda.New()
	app2.Use(New(cfg))
	app2.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		// Just read, don't modify.
		_ = sess.Get("key")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp2 := newMockResponse()
	app2.ServeKruda(resp2, newReq("GET", "/test", map[string]string{
		"_session": sessionID,
	}))

	// No Set-Cookie should be set for unmodified existing session.
	setCookie := resp2.headers.Get("Set-Cookie")
	if setCookie != "" {
		t.Fatalf("expected no Set-Cookie for unmodified session, got %q", setCookie)
	}
}

func TestSession_CookieAttributes(t *testing.T) {
	cfg := Config{
		CookieName:     "test_sess",
		CookiePath:     "/app",
		CookieDomain:   "example.com",
		CookieSecure:   true,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
	}

	app := kruda.New()
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("key", "value")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}

	checks := []struct {
		desc     string
		contains string
	}{
		{"cookie name", "test_sess="},
		{"path", "Path=/app"},
		{"domain", "Domain=example.com"},
		{"secure", "Secure"},
		{"httponly", "HttpOnly"},
		{"samesite", "SameSite=Strict"},
	}

	for _, check := range checks {
		if !strings.Contains(setCookie, check.contains) {
			t.Fatalf("expected %s (%q) in Set-Cookie, got %q", check.desc, check.contains, setCookie)
		}
	}
}

func TestSession_DefaultConfig(t *testing.T) {
	// Test that default config values are applied.
	app := kruda.New()
	app.Use(New())
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("key", "value")
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}

	// Default cookie name is "_session".
	if !strings.Contains(setCookie, "_session=") {
		t.Fatalf("expected default cookie name _session, got %q", setCookie)
	}

	// Default path is "/".
	if !strings.Contains(setCookie, "Path=/") {
		t.Fatalf("expected default Path=/, got %q", setCookie)
	}

	// Default SameSite is Lax.
	if !strings.Contains(setCookie, "SameSite=Lax") {
		t.Fatalf("expected default SameSite=Lax, got %q", setCookie)
	}
}

func TestSession_SessionIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateSessionID()
		if ids[id] {
			t.Fatalf("duplicate session ID generated at iteration %d", i)
		}
		ids[id] = true
	}
}

func TestSession_SessionIDLength(t *testing.T) {
	id := generateSessionID()
	// 32 bytes = 64 hex characters.
	if len(id) != 64 {
		t.Fatalf("expected 64 char session ID, got %d: %q", len(id), id)
	}
}

func TestSession_MultipleSetCookieSameSession(t *testing.T) {
	// Verify that setting multiple values in one request results in a single cookie.
	app := kruda.New()
	app.Use(New())
	app.Get("/test", func(c *kruda.Ctx) error {
		sess := GetSession(c)
		sess.Set("a", 1)
		sess.Set("b", 2)
		sess.Set("c", 3)
		c.SetContentType("application/json").SetBody(okBody)
		return nil
	})

	resp := newMockResponse()
	app.ServeKruda(resp, newReq("GET", "/test", nil))

	setCookie := resp.headers.Get("Set-Cookie")
	// Should have exactly one _session cookie.
	count := strings.Count(setCookie, "_session=")
	if count != 1 {
		t.Fatalf("expected exactly 1 _session cookie, found %d in %q", count, setCookie)
	}
}
