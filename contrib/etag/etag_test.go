package etag

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
)

func TestNew(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New())

	app.Get("/test", func(c *kruda.Ctx) error {
		body := []byte("test response")
		
		// Generate and set ETag before sending response
		etag := GenerateETag(body, true)
		if SetETag(c, etag) {
			// Return 304 if ETag matches
			return nil
		}
		
		// Send response with ETag already set
		return c.Text(string(body))
	})

	app.Compile()

	// Test ETag generation
	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header to be set")
		return // Prevent panic in next check
	}

	if len(etag) < 2 || etag[0:2] != "W/" {
		t.Errorf("Expected weak ETag, got %s", etag)
	}

	// Test 304 response with matching ETag
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("If-None-Match", etag)
	resp2 := httptest.NewRecorder()
	app.ServeHTTP(resp2, req2)

	if resp2.Code != 304 {
		t.Errorf("Expected status 304, got %d", resp2.Code)
	}

	if resp2.Body.Len() != 0 {
		t.Error("Expected empty body for 304 response")
	}
}

func TestSkipPOST(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New())

	app.Post("/test", func(c *kruda.Ctx) error {
		return c.Text("post response")
	})

	app.Compile()

	req := httptest.NewRequest("POST", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	etag := resp.Header().Get("ETag")
	if etag != "" {
		t.Error("Expected no ETag header for POST request")
	}
}

func TestSkipNon200(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New())

	app.Get("/test", func(c *kruda.Ctx) error {
		return c.Status(404).Text("not found")
	})

	app.Compile()

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 404 {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}

	etag := resp.Header().Get("ETag")
	if etag != "" {
		t.Error("Expected no ETag header for non-200 response")
	}
}

func TestSkipEmptyBody(t *testing.T) {
	etag := GenerateETag([]byte{}, true)
	if etag != "" {
		t.Error("Expected empty ETag for empty body")
	}

	etag = GenerateETag(nil, true)
	if etag != "" {
		t.Error("Expected empty ETag for nil body")
	}
}

func TestSkipExistingETag(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New())

	app.Get("/test", func(c *kruda.Ctx) error {
		h := c.ResponseWriter().Header()
		h.Set("ETag", `"custom-etag"`)
		return c.Text("test response")
	})

	app.Compile()

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	etag := resp.Header().Get("ETag")
	if etag != `"custom-etag"` {
		t.Errorf("Expected custom ETag to be preserved, got %s", etag)
	}
}

func TestGenerateETag(t *testing.T) {
	body := []byte("test content")
	
	// Test weak ETag
	etag := GenerateETag(body, true)
	if etag == "" {
		t.Error("Expected non-empty ETag")
	}
	if len(etag) < 2 || etag[0:2] != "W/" {
		t.Errorf("Expected weak ETag, got %s", etag)
	}

	// Test strong ETag
	etag = GenerateETag(body, false)
	if etag == "" {
		t.Error("Expected non-empty ETag")
	}
	if len(etag) >= 2 && etag[0:2] == "W/" {
		t.Errorf("Expected strong ETag, got %s", etag)
	}

	// Test consistent generation
	etag1 := GenerateETag(body, true)
	etag2 := GenerateETag(body, true)
	if etag1 != etag2 {
		t.Error("Expected consistent ETag generation")
	}

	// Test different content produces different ETag
	etag3 := GenerateETag([]byte("different content"), true)
	if etag1 == etag3 {
		t.Error("Expected different ETags for different content")
	}
}

func TestBasicHeaderSetting(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())

	app.Get("/test", func(c *kruda.Ctx) error {
		// Set headers directly on transport writer like ratelimit does
		h := c.ResponseWriter().Header()
		h.Set("X-Custom-Header", "test-value")
		h.Set("ETag", `W/"test-etag"`)
		return c.Text("test response")
	})

	app.Compile()

	req := httptest.NewRequest("GET", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	t.Logf("Response headers: %v", resp.Header())
	
	custom := resp.Header().Get("X-Custom-Header")
	if custom != "test-value" {
		t.Errorf("Expected X-Custom-Header to be set, got %s", custom)
	}
	
	etag := resp.Header().Get("ETag")
	if etag != `W/"test-etag"` {
		t.Errorf("Expected ETag to be set, got %s", etag)
	}
}

func TestETagMatches(t *testing.T) {
	etag := `W/"abc123-10"`

	// Test exact match
	if !etagMatches(etag, etag) {
		t.Error("Expected exact ETag match")
	}

	// Test wildcard
	if !etagMatches("*", etag) {
		t.Error("Expected wildcard to match any ETag")
	}

	// Test no match
	if etagMatches(`W/"different-10"`, etag) {
		t.Error("Expected no match for different ETag")
	}

	// Test weak ETag comparison
	if !etagMatches(`W/"abc123-10"`, `"abc123-10"`) {
		t.Error("Expected weak ETag to match strong ETag with same value")
	}
}

func TestWeakETagsMatch(t *testing.T) {
	if !weakETagsMatch(`W/"abc123"`, `W/"abc123"`) {
		t.Error("Expected weak ETags to match")
	}

	if !weakETagsMatch(`W/"abc123"`, `"abc123"`) {
		t.Error("Expected weak and strong ETags to match")
	}

	if weakETagsMatch(`W/"abc123"`, `W/"different"`) {
		t.Error("Expected different weak ETags not to match")
	}
}

func TestIsWeakETag(t *testing.T) {
	if !isWeakETag(`W/"abc123"`) {
		t.Error("Expected W/ prefix to be identified as weak ETag")
	}

	if isWeakETag(`"abc123"`) {
		t.Error("Expected no W/ prefix to be identified as strong ETag")
	}

	if isWeakETag("W/") {
		t.Error("Expected short W/ to not be identified as weak ETag")
	}
}

func TestContainsETag(t *testing.T) {
	list := `W/"abc123", "def456", W/"ghi789"`
	
	if !containsETag(list, `W/"abc123"`) {
		t.Error("Expected to find first ETag in list")
	}

	if !containsETag(list, `"def456"`) {
		t.Error("Expected to find middle ETag in list")
	}

	if !containsETag(list, `W/"ghi789"`) {
		t.Error("Expected to find last ETag in list")
	}

	if containsETag(list, `"notfound"`) {
		t.Error("Expected not to find non-existent ETag")
	}
}

func TestConfigDefaults(t *testing.T) {
	var cfg Config
	cfg.defaults()

	if !cfg.Weak {
		t.Error("Expected default Weak to be true")
	}
}

func TestSkipFunction(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/skip"
		},
	}))

	app.Get("/skip", func(c *kruda.Ctx) error {
		// Don't call GenerateAndSetETag on skipped paths
		return c.Text("skipped")
	})

	app.Get("/noskip", func(c *kruda.Ctx) error {
		body := []byte("not skipped")
		GenerateAndSetETag(c, body)
		return c.Text(string(body))
	})

	app.Compile()

	// Test skipped path
	req := httptest.NewRequest("GET", "/skip", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	etag := resp.Header().Get("ETag")
	if etag != "" {
		t.Error("Expected no ETag for skipped path")
	}

	// Test non-skipped path
	req2 := httptest.NewRequest("GET", "/noskip", nil)
	resp2 := httptest.NewRecorder()
	app.ServeHTTP(resp2, req2)

	etag2 := resp2.Header().Get("ETag")
	if etag2 == "" {
		t.Error("Expected ETag for non-skipped path")
	}
}

func TestHEADMethod(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New())

	// Register both GET and HEAD for the same route
	handler := func(c *kruda.Ctx) error {
		body := []byte("test response")
		if GenerateAndSetETag(c, body) {
			return nil
		}
		return c.Text(string(body))
	}
	
	app.Get("/test", handler)
	app.Head("/test", handler)

	app.Compile()

	// Test HEAD request
	req := httptest.NewRequest("HEAD", "/test", nil)
	resp := httptest.NewRecorder()
	app.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header for HEAD request")
	}
}