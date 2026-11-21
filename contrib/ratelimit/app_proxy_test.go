package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// TestDefaultKeyFollowsAppTrustedProxies trusts a proxy on the app only. Two
// clients behind it must get their own buckets instead of sharing the proxy's.
func TestDefaultKeyFollowsAppTrustedProxies(t *testing.T) {
	app := kruda.New(kruda.NetHTTP(), kruda.WithTrustProxy(true), kruda.WithTrustedProxies("192.0.2.10"))
	app.Use(New(Config{Max: 1, Window: time.Minute}))
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	do := func(client string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		req.Header.Set("X-Forwarded-For", client)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := do("198.51.100.1"); got != 200 {
		t.Fatalf("first client: %d, want 200", got)
	}
	if got := do("198.51.100.2"); got != 200 {
		t.Fatalf("second client behind the same proxy: %d, want 200", got)
	}
	if got := do("198.51.100.1"); got != http.StatusTooManyRequests {
		t.Fatalf("first client again: %d, want 429", got)
	}
}
