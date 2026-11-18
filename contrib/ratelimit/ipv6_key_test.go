package ratelimit

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

func TestIPv6ClientsShareTheirSlash64Limit(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{Max: 1, Window: time.Minute}))
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	do := func(peer string) int {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = net.JoinHostPort(peer, "1234")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		return rec.Code
	}
	if got := do("2001:db8:1:2::1"); got != 200 {
		t.Fatalf("first request: %d, want 200", got)
	}
	if got := do("2001:db8:1:2::ffff"); got != http.StatusTooManyRequests {
		t.Fatalf("second address in the same /64: %d, want 429", got)
	}
	if got := do("2001:db8:1:3::1"); got != 200 {
		t.Fatalf("address in another /64: %d, want 200", got)
	}
	if got := do("192.0.2.1"); got != 200 {
		t.Fatalf("IPv4 client: %d, want 200", got)
	}
	if got := do("192.0.2.2"); got != 200 {
		t.Fatalf("second IPv4 client: %d, want 200 (IPv4 keys stay per address)", got)
	}
}

func TestClientKey(t *testing.T) {
	for in, want := range map[string]string{
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:2::/64",
		"192.0.2.7":            "192.0.2.7",
		"unknown":              "unknown",
	} {
		if got := clientKey(in); got != want {
			t.Errorf("clientKey(%q) = %q, want %q", in, got, want)
		}
	}
}
