package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// newTestApp creates a Kruda app with rate limiter middleware and a simple handler.
func newTestApp(cfg Config) (*kruda.App, *httptest.Server) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(cfg))
	app.Get("/test", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	app.Compile()
	srv := httptest.NewServer(app)
	return app, srv
}

func TestTokenBucket_WithinLimit(t *testing.T) {
	_, srv := newTestApp(Config{Max: 5, Window: time.Minute})
	defer srv.Close()

	for i := 0; i < 5; i++ {
		resp, err := http.Get(srv.URL + "/test")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestTokenBucket_ExceedLimit(t *testing.T) {
	_, srv := newTestApp(Config{Max: 3, Window: time.Minute})
	defer srv.Close()

	// Exhaust tokens
	for i := 0; i < 3; i++ {
		resp, err := http.Get(srv.URL + "/test")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	// Next request should be rejected
	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}

	// Check error body
	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["error"] != "rate limit exceeded" {
		t.Errorf("expected 'rate limit exceeded', got %v", body["error"])
	}
}

func TestRateLimitHeaders(t *testing.T) {
	_, srv := newTestApp(Config{Max: 10, Window: time.Minute})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Check X-RateLimit-Limit
	limit := resp.Header.Get("X-RateLimit-Limit")
	if limit != "10" {
		t.Errorf("X-RateLimit-Limit: expected '10', got %q", limit)
	}

	// Check X-RateLimit-Remaining
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	rem, _ := strconv.Atoi(remaining)
	if rem != 9 {
		t.Errorf("X-RateLimit-Remaining: expected 9, got %d", rem)
	}

	// Check X-RateLimit-Reset exists and is a valid unix timestamp
	reset := resp.Header.Get("X-RateLimit-Reset")
	if reset == "" {
		t.Error("X-RateLimit-Reset header missing")
	}
	resetUnix, err := strconv.ParseInt(reset, 10, 64)
	if err != nil {
		t.Errorf("X-RateLimit-Reset not a valid unix timestamp: %v", err)
	}
	if resetUnix <= time.Now().Unix() {
		t.Error("X-RateLimit-Reset should be in the future")
	}
}

func TestRetryAfterHeader(t *testing.T) {
	_, srv := newTestApp(Config{Max: 1, Window: time.Minute})
	defer srv.Close()

	// First request — allowed
	resp, _ := http.Get(srv.URL + "/test")
	resp.Body.Close()

	// Second request — rejected
	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != 429 {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header missing on 429 response")
	}
	sec, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Errorf("Retry-After not a valid integer: %v", err)
	}
	if sec < 1 {
		t.Errorf("Retry-After should be >= 1, got %d", sec)
	}
}

func TestSkipFunction(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Max:    1,
		Window: time.Minute,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/health"
		},
	}))
	app.Get("/test", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Get("/health", func(c *kruda.Ctx) error { return c.Text("healthy") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Exhaust limit on /test
	resp, _ := http.Get(srv.URL + "/test")
	resp.Body.Close()

	// /test should be rate limited
	resp, _ = http.Get(srv.URL + "/test")
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("expected 429 on /test, got %d", resp.StatusCode)
	}

	// /health should bypass rate limiting
	for i := 0; i < 5; i++ {
		resp, _ = http.Get(srv.URL + "/health")
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("/health request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestCustomKeyFunc(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Max:    2,
		Window: time.Minute,
		KeyFunc: func(c *kruda.Ctx) string {
			return c.Header("X-API-Key")
		},
	}))
	app.Get("/test", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Client A: 2 requests → allowed
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
		req.Header.Set("X-API-Key", "client-a")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("client-a request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// Client A: 3rd request → rejected
	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-API-Key", "client-a")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("client-a 3rd request: expected 429, got %d", resp.StatusCode)
	}

	// Client B: still has quota
	req, _ = http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-API-Key", "client-b")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("client-b: expected 200, got %d", resp.StatusCode)
	}
}

func TestSlidingWindow_WithinLimit(t *testing.T) {
	_, srv := newTestApp(Config{Max: 5, Window: time.Minute, Algorithm: "sliding_window"})
	defer srv.Close()

	for i := 0; i < 5; i++ {
		resp, err := http.Get(srv.URL + "/test")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestSlidingWindow_ExceedLimit(t *testing.T) {
	_, srv := newTestApp(Config{Max: 3, Window: time.Minute, Algorithm: "sliding_window"})
	defer srv.Close()

	for i := 0; i < 3; i++ {
		resp, _ := http.Get(srv.URL + "/test")
		resp.Body.Close()
	}

	resp, _ := http.Get(srv.URL + "/test")
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestTrustedProxy(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Max:            2,
		Window:         time.Minute,
		TrustedProxies: []string{"127.0.0.1"},
	}))
	app.Get("/test", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// Requests with X-Forwarded-For from trusted proxy
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 3rd request from same forwarded IP → rejected
	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}

	// Different forwarded IP → allowed
	req, _ = http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.2")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("different IP: expected 200, got %d", resp.StatusCode)
	}
}

func TestUntrustedProxy_IgnoresXFF(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Max:    2,
		Window: time.Minute,
		// No TrustedProxies — X-Forwarded-For should be ignored
	}))
	app.Get("/test", func(c *kruda.Ctx) error { return c.Text("ok") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// All requests come from same RemoteAddr regardless of XFF
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
		req.Header.Set("X-Forwarded-For", "different-ip-"+strconv.Itoa(i))
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 3rd request — should be limited by RemoteAddr, not XFF
	req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req.Header.Set("X-Forwarded-For", "yet-another-ip")
	resp, _ := http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("expected 429 (XFF ignored), got %d", resp.StatusCode)
	}
}

func TestForRoute(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(ForRoute("/api/login", 1, time.Minute))
	app.Get("/api/login", func(c *kruda.Ctx) error { return c.Text("login") })
	app.Get("/api/data", func(c *kruda.Ctx) error { return c.Text("data") })
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	// First login request — allowed
	resp, _ := http.Get(srv.URL + "/api/login")
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("first login: expected 200, got %d", resp.StatusCode)
	}

	// Second login request — rejected
	resp, _ = http.Get(srv.URL + "/api/login")
	resp.Body.Close()
	if resp.StatusCode != 429 {
		t.Errorf("second login: expected 429, got %d", resp.StatusCode)
	}

	// /api/data — not rate limited by this middleware
	for i := 0; i < 5; i++ {
		resp, _ = http.Get(srv.URL + "/api/data")
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("/api/data request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore(50 * time.Millisecond)
	defer store.Stop()

	// Add an entry
	e := store.getEntry("test-key")
	e.mu.Lock()
	e.last = time.Now().Add(-5 * time.Minute) // make it old
	e.mu.Unlock()

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	// Entry should be cleaned up
	_, exists := store.entries.Load("test-key")
	if exists {
		t.Error("expected expired entry to be cleaned up")
	}
}

func TestMemoryStore_Stop(t *testing.T) {
	store := NewMemoryStore(time.Hour)
	store.Stop()
	// Double stop should not panic
	store.Stop()
}
