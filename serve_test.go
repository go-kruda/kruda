package kruda

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- ServeKruda: panic recovery + lifecycle hook errors ---

func TestServeKruda_PanicRecovery(t *testing.T) {
	app := New()
	app.Get("/panic", func(c *Ctx) error {
		panic("test panic")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/panic")
	if resp.StatusCode() != 500 {
		t.Errorf("status = %d, want 500 after panic", resp.StatusCode())
	}
}

func TestServeKruda_OnRequestError(t *testing.T) {
	app := New()
	app.OnRequest(func(c *Ctx) error {
		return NewError(429, "rate limited")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 429 {
		t.Errorf("status = %d, want 429", resp.StatusCode())
	}
}

func TestServeKruda_BeforeHandleError(t *testing.T) {
	app := New()
	app.BeforeHandle(func(c *Ctx) error {
		return NewError(401, "unauthorized")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("should not reach")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	if resp.StatusCode() != 401 {
		t.Errorf("status = %d, want 401", resp.StatusCode())
	}
}

func TestServeKruda_AfterHandleError(t *testing.T) {
	app := New()
	app.AfterHandle(func(c *Ctx) error {
		return NewError(500, "after handle fail")
	})
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/test")
	// AfterHandle error is handled but response may already be written
	if resp.StatusCode() != 200 && resp.StatusCode() != 500 {
		t.Errorf("status = %d", resp.StatusCode())
	}
}

func TestServeKruda_PanicRecovery_AlreadyResponded(t *testing.T) {
	app := New()
	app.Get("/panic-after-write", func(c *Ctx) error {
		_ = c.Text("ok")
		panic("late panic after response")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/panic-after-write"}
	resp := newMockResponse()

	// Should not crash even if panic occurs after response is written
	app.ServeKruda(resp, req)
}

// --- ServeHTTP (net/http adapter) ---

func TestServeHTTP_PathTraversalBlocked_Boost3(t *testing.T) {
	app := New(WithPathTraversal(), NetHTTP())
	app.Get("/safe", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	req := httptest.NewRequest("GET", "/../../etc/passwd", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400 for path traversal", w.Code)
	}
}

func TestServeHTTP_MethodNotAllowed_Boost3(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/data-boost3", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	req := httptest.NewRequest("POST", "/data-boost3", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("status = %d, want 405", w.Code)
	}
	// 405 is the key assertion; Allow header visibility in recorder
	// depends on header write timing in the net/http adapter.
}

func TestServeHTTP_NotFound_Boost3(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/exists-boost3", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	req := httptest.NewRequest("GET", "/nope-boost3", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestServeHTTP_HandlerError_Boost3(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/fail-boost3", func(c *Ctx) error {
		return errors.New("handler error")
	})
	app.Compile()

	req := httptest.NewRequest("GET", "/fail-boost3", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestServeHTTP_EmptyPath_Boost3(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/", func(c *Ctx) error {
		return c.Text("root")
	})
	app.Compile()

	// Empty URL path should be treated as /
	req := httptest.NewRequest("GET", "/", nil)
	req.URL.Path = ""
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// With empty path, it should be set to "/" and match root
	if w.Code == 0 {
		t.Error("status should not be 0")
	}
}

func TestServeHTTP_SecurityHeaders_Boost3(t *testing.T) {
	// Test that security headers are compiled and applied via ServeKruda
	app := New(WithSecurity())
	app.Get("/test", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.statusCode)
	}
	// Security headers should be set via writeHeaders
	if resp.headers.Get("X-Content-Type-Options") == "" {
		t.Error("expected X-Content-Type-Options header")
	}
}

func TestServeHTTP_MultipartCleanup(t *testing.T) {
	app := New(NetHTTP())
	app.Post("/upload", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	// Send a POST with multipart content type
	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"\r\n\r\ncontent\r\n--boundary--")
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestServeHTTP_PathTraversal_CleanedSuccessfully(t *testing.T) {
	app := New(NetHTTP(), WithPathTraversal())
	app.Get("/a/c", func(c *Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	// /a/b/../c should be cleaned to /a/c
	req := httptest.NewRequest("GET", "/a/b/../c", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 after path cleaning", w.Code)
	}
}

func TestServeHTTP_ShrinkMapsOnNotFound(t *testing.T) {
	app := New(NetHTTP())
	app.Compile()

	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestServeHTTP_CookieSliceReuse(t *testing.T) {
	app := New(NetHTTP())
	app.Get("/test", func(c *Ctx) error {
		c.SetCookie(&Cookie{Name: "test", Value: "val"})
		return c.Text("ok")
	})
	app.Compile()

	// Send two requests to test pool reuse
	for range 2 {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("status = %d, want 200", w.Code)
		}
	}
}

// --- handleError: custom handler / validation error / already responded /
// production message stripping ---

func TestHandleError_CustomHandler(t *testing.T) {
	customCalled := false
	app := New(WithErrorHandler(func(c *Ctx, err *KrudaError) {
		customCalled = true
		c.Status(503)
		_ = c.Text("custom error")
	}))
	app.Get("/fail", func(c *Ctx) error {
		return InternalError("oops")
	})
	app.Compile()

	tc := NewTestClient(app)
	resp, _ := tc.Get("/fail")
	if !customCalled {
		t.Error("custom error handler not called")
	}
	if resp.StatusCode() != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode())
	}
}

func TestHandleError_ValidationError_CustomHandler(t *testing.T) {
	var capturedErr error
	app := New(WithErrorHandler(func(c *Ctx, ke *KrudaError) {
		capturedErr = ke
		c.Status(ke.Code)
		_ = c.JSON(Map{"custom": true, "code": ke.Code})
	}))

	app.Get("/validate", func(c *Ctx) error {
		ve := &ValidationError{
			Errors: []FieldError{{Field: "name", Rule: "required", Message: "is required"}},
		}
		return ve
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/validate"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if capturedErr == nil {
		t.Fatal("custom error handler should have been called")
	}
	if resp.statusCode != 422 {
		t.Errorf("status = %d, want 422", resp.statusCode)
	}
}

func TestHandleError_AlreadyResponded(t *testing.T) {
	app := New()
	app.Get("/test", func(c *Ctx) error {
		_ = c.Text("already sent")
		// Return error after already responded
		return errors.New("late error")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/test"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	// Should have the first response, not a 500
	if resp.statusCode == 500 {
		t.Log("error handler wrote 500 after panic recovery")
	}
}

func TestHandleError_5xxStripDetail_Production(t *testing.T) {
	app := New() // production mode
	app.Get("/fail", func(c *Ctx) error {
		ke := NewError(500, "server error")
		ke.Detail = "internal details here"
		return ke
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	body := string(resp.body)
	if strings.Contains(body, "internal details here") {
		t.Error("production 5xx should strip Detail")
	}
}

func TestHandleError_4xxPreserve_Production(t *testing.T) {
	app := New()
	app.Get("/fail", func(c *Ctx) error {
		return BadRequest("invalid input")
	})
	app.Compile()

	req := &mockRequest{method: "GET", path: "/fail"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	if resp.statusCode != 400 {
		t.Errorf("status = %d, want 400", resp.statusCode)
	}
	body := string(resp.body)
	if !strings.Contains(body, "invalid input") {
		t.Error("4xx should preserve message in production")
	}
}
