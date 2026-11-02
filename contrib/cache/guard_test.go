package cache

import (
	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
	"net/http/httptest"
	"testing"
)

func TestCacheDoesNotSkipLaterGuard(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	allowed := true
	app.Group("/private").Guard(func(c *kruda.Ctx) error {
		if !allowed {
			return c.Status(401).Text("denied")
		}
		return c.Next()
	}).Get("/data", func(c *kruda.Ctx) error { return CacheBytes(c, "text/plain", []byte("secret")) })
	for _, want := range []int{200, 401} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/private/data", nil))
		if rec.Code != want {
			t.Fatalf("status=%d want=%d", rec.Code, want)
		}
		allowed = false
	}
}

func TestCacheAfterGuardStillHits(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	guards, calls := 0, 0
	app.Use(func(c *kruda.Ctx) error { guards++; return c.Next() }, New(Config{Store: store}))
	app.Get("/data", func(c *kruda.Ctx) error { calls++; return CacheBytes(c, "text/plain", []byte("ok")) })
	for range 2 {
		app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/data", nil))
	}
	if guards != 2 || calls != 2 {
		t.Fatalf("guards=%d handlers=%d", guards, calls)
	}
}

func TestCacheDoesNotSkipHandlerAuthorization(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	allowed := true
	app.Get("/data", func(c *kruda.Ctx) error {
		if !allowed {
			return c.Status(401).Text("denied")
		}
		return CacheBytes(c, "text/plain", []byte("secret"))
	})
	for _, want := range []int{200, 401} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/data", nil))
		if rec.Code != want {
			t.Fatalf("status=%d want=%d", rec.Code, want)
		}
		allowed = false
	}
}

func TestCacheSeparatesEncodedPathFromQuery(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	app.Get("/users/:id", func(c *kruda.Ctx) error { return CacheBytes(c, "text/plain", []byte(c.Param("id"))) })
	for _, tc := range []struct{ uri, want string }{{"/users/alice%3Fx", "alice?x"}, {"/users/alice?x", "alice"}} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", tc.uri, nil))
		if rec.Body.String() != tc.want {
			t.Fatalf("%s got %q", tc.uri, rec.Body.String())
		}
	}
}

func TestCacheReplayDoesNotReplaceDenialStatus(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	allowed := true
	app.Get("/data", func(c *kruda.Ctx) error {
		if !allowed {
			return CacheJSON(c.Status(403), kruda.Map{"error": "denied"})
		}
		return CacheJSON(c, kruda.Map{"secret": true})
	})
	for _, want := range []int{200, 403} {
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest("GET", "/data", nil))
		if rec.Code != want {
			t.Fatalf("status=%d want=%d", rec.Code, want)
		}
		allowed = false
	}
}

func TestCacheReplaySkipsWorkAfterAuthorization(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	guards, queries := 0, 0
	app.Get("/data", func(c *kruda.Ctx) error {
		guards++
		if hit, err := Replay(c); hit {
			return err
		}
		queries++
		return CacheBytes(c, "text/plain", []byte("ok"))
	})
	for range 2 {
		app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/data", nil))
	}
	if guards != 2 || queries != 1 {
		t.Fatalf("guards=%d queries=%d", guards, queries)
	}
}

type flattenedQueryRequest struct{ transport.Request }

func (flattenedQueryRequest) AllQuery() map[string]string {
	return map[string]string{"id": "flattened"}
}
func TestCacheBypassesRequestsWithoutLosslessQuery(t *testing.T) {
	store := NewMemoryStore(100)
	defer store.Close()
	app := kruda.New()
	app.Use(New(Config{Store: store}))
	app.Get("/data", func(c *kruda.Ctx) error { return CacheBytes(c, "text/plain", []byte(c.Query("id"))) })
	for _, id := range []string{"alpha", "beta"} {
		rec := newMockResponse()
		app.ServeKruda(rec, flattenedQueryRequest{newReqWithQuery("GET", "/data", map[string]string{"id": id})})
		if string(rec.body) != id || rec.headers.Get("X-Cache") != "" {
			t.Fatalf("id=%s body=%q cache=%s", id, rec.body, rec.headers.Get("X-Cache"))
		}
	}
	if store.Len() != 0 {
		t.Fatal("stored response using lossy query")
	}
}
