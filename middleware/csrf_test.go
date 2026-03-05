package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/go-kruda/kruda"
)

// --- CSRF-specific mock request with cookie support ---

type csrfMockRequest struct {
	method  string
	path    string
	headers map[string]string
	cookies map[string]string
	body    []byte
}

func (r *csrfMockRequest) Method() string               { return r.method }
func (r *csrfMockRequest) Path() string                 { return r.path }
func (r *csrfMockRequest) Header(key string) string     { return r.headers[key] }
func (r *csrfMockRequest) Body() ([]byte, error)        { return r.body, nil }
func (r *csrfMockRequest) QueryParam(key string) string { return "" }
func (r *csrfMockRequest) RemoteAddr() string           { return "127.0.0.1" }
func (r *csrfMockRequest) RawRequest() any              { return nil }
func (r *csrfMockRequest) Context() context.Context     { return context.Background() }

func (r *csrfMockRequest) Cookie(name string) string {
	if r.cookies != nil {
		return r.cookies[name]
	}
	return ""
}

func (r *csrfMockRequest) MultipartForm(int64) (*multipart.Form, error) {
	return nil, fmt.Errorf("not supported")
}

// --- CSRF test helpers ---

func csrfServe(t *testing.T, mw kruda.HandlerFunc, handler kruda.HandlerFunc, req *csrfMockRequest) *mockResponseWriter {
	t.Helper()
	app := kruda.New()
	app.Use(mw)
	app.Get("/test", handler)
	app.Post("/test", handler)
	app.Put("/test", handler)
	app.Delete("/test", handler)

	resp := newMockResponse()
	app.ServeKruda(resp, req)
	return resp
}

func csrfReq(method, path string, headers, cookies map[string]string) *csrfMockRequest {
	if headers == nil {
		headers = make(map[string]string)
	}
	if cookies == nil {
		cookies = make(map[string]string)
	}
	return &csrfMockRequest{
		method:  method,
		path:    path,
		headers: headers,
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

func TestCSRF_GET_SetsTokenCookie(t *testing.T) {
	var gotToken any
	handler := func(c *kruda.Ctx) error {
		gotToken = c.Get("csrf_token")
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("GET", "/test", nil, nil)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	// Token should be set in context.
	if gotToken == nil || gotToken.(string) == "" {
		t.Fatal("expected csrf_token in context")
	}
	token := gotToken.(string)

	// Token should be 64 hex chars (32 bytes).
	if len(token) != 64 {
		t.Fatalf("expected 64 char token, got %d chars: %q", len(token), token)
	}

	// Cookie should be set.
	setCookie := resp.headers.Get("Set-Cookie")
	if setCookie == "" {
		t.Fatal("expected Set-Cookie header")
	}
	cookieToken := extractSetCookie(setCookie, "_csrf")
	if cookieToken == "" {
		t.Fatal("expected _csrf cookie in Set-Cookie header")
	}
	if cookieToken != token {
		t.Fatalf("cookie token %q != context token %q", cookieToken, token)
	}

	// Verify cookie attributes.
	if !strings.Contains(setCookie, "SameSite=Strict") {
		t.Fatal("expected SameSite=Strict in CSRF cookie")
	}
	// HttpOnly should NOT be present (JS needs to read cookie).
	if strings.Contains(setCookie, "HttpOnly") {
		t.Fatal("CSRF cookie should NOT have HttpOnly flag")
	}
}

func TestCSRF_GET_VaryHeader(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("GET", "/test", nil, nil)
	resp := csrfServe(t, CSRF(), handler, req)

	vary := resp.headers.Get("Vary")
	if !strings.Contains(vary, "Cookie") {
		t.Fatalf("expected Vary to contain Cookie, got %q", vary)
	}
}

func TestCSRF_POST_ValidToken(t *testing.T) {
	token := generateCSRFToken(32) // generate a valid token

	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		map[string]string{"X-CSRF-Token": token},
		map[string]string{"_csrf": token},
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d (body: %s)", resp.statusCode, string(resp.body))
	}
}

func TestCSRF_POST_MissingCookie(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		map[string]string{"X-CSRF-Token": "sometoken"},
		nil, // no cookies
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.statusCode)
	}

	var body map[string]string
	json.Unmarshal(resp.body, &body)
	if body["error"] != "csrf_token_invalid" {
		t.Fatalf("expected csrf_token_invalid error, got %q", body["error"])
	}
}

func TestCSRF_POST_MissingHeader(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		nil, // no header
		map[string]string{"_csrf": "sometoken"},
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.statusCode)
	}
}

func TestCSRF_POST_WrongToken(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		map[string]string{"X-CSRF-Token": "wrong-token"},
		map[string]string{"_csrf": "correct-token"},
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403, got %d", resp.statusCode)
	}
}

func TestCSRF_PUT_RequiresToken(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("PUT", "/test", nil, nil)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403 for PUT without token, got %d", resp.statusCode)
	}
}

func TestCSRF_DELETE_RequiresToken(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("DELETE", "/test", nil, nil)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403 for DELETE without token, got %d", resp.statusCode)
	}
}

func TestCSRF_HEAD_NoValidation(t *testing.T) {
	var gotToken any
	handler := func(c *kruda.Ctx) error {
		gotToken = c.Get("csrf_token")
		return c.NoContent()
	}

	app := kruda.New()
	app.Use(CSRF())
	app.Head("/test", handler)

	req := csrfReq("HEAD", "/test", nil, nil)
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.statusCode)
	}
	if gotToken == nil || gotToken.(string) == "" {
		t.Fatal("expected csrf_token set for HEAD request")
	}
}

func TestCSRF_OPTIONS_NoValidation(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.NoContent()
	}

	app := kruda.New()
	app.Use(CSRF())
	app.Options("/test", handler)

	req := csrfReq("OPTIONS", "/test", nil, nil)
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.statusCode)
	}
}

func TestCSRF_Skip(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	mw := CSRF(CSRFConfig{
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/test" // skip /test
		},
	})

	// POST without token should succeed because Skip returns true.
	req := csrfReq("POST", "/test", nil, nil)
	resp := csrfServe(t, mw, handler, req)

	if resp.statusCode != 200 {
		t.Fatalf("expected 200 when skipped, got %d", resp.statusCode)
	}
}

func TestCSRF_CustomErrorHandler(t *testing.T) {
	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	mw := CSRF(CSRFConfig{
		ErrorHandler: func(c *kruda.Ctx) error {
			return c.Status(418).JSON(map[string]string{"custom": "error"})
		},
	})

	req := csrfReq("POST", "/test", nil, nil)
	resp := csrfServe(t, mw, handler, req)

	if resp.statusCode != 418 {
		t.Fatalf("expected 418 from custom error handler, got %d", resp.statusCode)
	}

	var body map[string]string
	json.Unmarshal(resp.body, &body)
	if body["custom"] != "error" {
		t.Fatalf("expected custom error body, got %v", body)
	}
}

func TestCSRF_CustomConfig(t *testing.T) {
	var gotToken any
	handler := func(c *kruda.Ctx) error {
		gotToken = c.Get("csrf_token")
		return c.JSON(kruda.Map{"ok": true})
	}

	mw := CSRF(CSRFConfig{
		CookieName:  "my_csrf",
		HeaderName:  "X-My-CSRF",
		TokenLength: 48,
	})

	// GET should set custom cookie name.
	req := csrfReq("GET", "/test", nil, nil)
	resp := csrfServe(t, mw, handler, req)

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	token := gotToken.(string)
	// 48 bytes → 96 hex chars.
	if len(token) != 96 {
		t.Fatalf("expected 96 char token (48 bytes), got %d", len(token))
	}

	setCookie := resp.headers.Get("Set-Cookie")
	if !strings.Contains(setCookie, "my_csrf=") {
		t.Fatalf("expected my_csrf cookie, got %q", setCookie)
	}

	// POST with custom header name should work.
	postReq := csrfReq("POST", "/test",
		map[string]string{"X-My-CSRF": token},
		map[string]string{"my_csrf": token},
	)
	postResp := csrfServe(t, mw, handler, postReq)

	if postResp.statusCode != 200 {
		t.Fatalf("expected 200 with custom header, got %d", postResp.statusCode)
	}
}

func TestCSRF_TokenUniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateCSRFToken(32)
		if tokens[token] {
			t.Fatalf("duplicate token generated at iteration %d", i)
		}
		tokens[token] = true
	}
}

func TestCSRF_TokenLength(t *testing.T) {
	token := generateCSRFToken(32)
	if len(token) != 64 {
		t.Fatalf("expected 64 hex chars for 32 bytes, got %d", len(token))
	}

	token = generateCSRFToken(16)
	if len(token) != 32 {
		t.Fatalf("expected 32 hex chars for 16 bytes, got %d", len(token))
	}
}

func TestCSRF_PanicsOnShortToken(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for TokenLength < 16")
		}
		msg := fmt.Sprintf("%v", r)
		if !strings.Contains(msg, "TokenLength must be at least 16") {
			t.Fatalf("unexpected panic message: %v", r)
		}
	}()

	CSRF(CSRFConfig{TokenLength: 8})
}

func TestCSRF_POST_RefreshesToken(t *testing.T) {
	token := generateCSRFToken(32)
	var newToken any

	handler := func(c *kruda.Ctx) error {
		newToken = c.Get("csrf_token")
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		map[string]string{"X-CSRF-Token": token},
		map[string]string{"_csrf": token},
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.statusCode)
	}

	// After successful POST, a new token should be generated.
	if newToken == nil || newToken.(string) == "" {
		t.Fatal("expected new csrf_token after POST")
	}
	if newToken.(string) == token {
		t.Fatal("expected new token to be different from old token")
	}
}

func TestCSRF_ConstantTimeComparison(t *testing.T) {
	// This test verifies that even with a partially matching token, access is denied.
	token := generateCSRFToken(32)
	partialMatch := token[:len(token)-2] + "xx" // change last 2 chars

	handler := func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"ok": true})
	}

	req := csrfReq("POST", "/test",
		map[string]string{"X-CSRF-Token": partialMatch},
		map[string]string{"_csrf": token},
	)
	resp := csrfServe(t, CSRF(), handler, req)

	if resp.statusCode != 403 {
		t.Fatalf("expected 403 for partial token match, got %d", resp.statusCode)
	}
}
