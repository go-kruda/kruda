package kruda

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type healthyDB struct{}

func (h *healthyDB) Check(_ context.Context) error { return nil }

type healthyCache struct{}

func (h *healthyCache) Check(_ context.Context) error { return nil }

type unhealthyDB struct{ err error }

func (h *unhealthyDB) Check(_ context.Context) error { return h.err }

type slowChecker struct{ delay time.Duration }

func (h *slowChecker) Check(ctx context.Context) error {
	select {
	case <-time.After(h.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func execHealth(t *testing.T, app *App) (int, map[string]any) {
	t.Helper()
	app.Compile()
	req := &mockRequest{method: "GET", path: "/health"}
	resp := newMockResponse()
	app.ServeKruda(resp, req)

	var body map[string]any
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatalf("failed to parse response JSON: %v\nbody: %s", err, resp.body)
	}
	return resp.statusCode, body
}

func TestHealthHandlerNoContainer(t *testing.T) {
	app := New()
	app.Get("/health", HealthHandler())

	code, body := execHealth(t, app)

	if code != 200 {
		t.Fatalf("expected 200, got %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("expected checks to be object, got %T", body["checks"])
	}
	if len(checks) != 0 {
		t.Fatalf("expected empty checks, got %v", checks)
	}
}

func TestHealthHandlerAllHealthy(t *testing.T) {
	c := NewContainer()
	_ = c.Give(&healthyDB{})
	_ = c.Give(&healthyCache{})

	app := New(WithContainer(c))
	app.Get("/health", HealthHandler())

	code, body := execHealth(t, app)

	if code != 200 {
		t.Fatalf("expected 200, got %d", code)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok {
		t.Fatalf("expected checks to be object, got %T", body["checks"])
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d: %v", len(checks), checks)
	}
	for name, v := range checks {
		if v != "ok" {
			t.Errorf("checker %q: expected ok, got %v", name, v)
		}
	}
}

func TestHealthHandlerOneUnhealthy(t *testing.T) {
	c := NewContainer()
	_ = c.Give(&healthyDB{})
	_ = c.Give(&unhealthyDB{err: errors.New("db connection lost")})

	app := New(WithContainer(c))
	app.Get("/health", HealthHandler())

	code, body := execHealth(t, app)

	if code != 503 {
		t.Fatalf("expected 503, got %d", code)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected status unhealthy, got %v", body["status"])
	}
	checks := body["checks"].(map[string]any)
	foundUnhealthy := false
	for _, v := range checks {
		if v == "db connection lost" {
			foundUnhealthy = true
		}
	}
	if !foundUnhealthy {
		t.Fatalf("expected to find unhealthy checker with error message, checks: %v", checks)
	}
}

func TestHealthHandlerTimeout(t *testing.T) {
	c := NewContainer()
	_ = c.Give(&slowChecker{delay: 2 * time.Second})

	app := New(WithContainer(c))
	app.Get("/health", HealthHandler(WithHealthTimeout(100*time.Millisecond)))

	code, body := execHealth(t, app)

	if code != 503 {
		t.Fatalf("expected 503, got %d", code)
	}
	if body["status"] != "unhealthy" {
		t.Fatalf("expected status unhealthy, got %v", body["status"])
	}
	checks := body["checks"].(map[string]any)
	for name, v := range checks {
		vs, _ := v.(string)
		if vs != "health check timed out" && vs != "context deadline exceeded" {
			t.Errorf("checker %q: expected timeout message, got %v", name, v)
		}
	}
}

func TestHealthHandlerCustomTimeout(t *testing.T) {
	cfg := defaultHealthConfig()
	if cfg.Timeout != 5*time.Second {
		t.Fatalf("expected default timeout 5s, got %v", cfg.Timeout)
	}

	opt := WithHealthTimeout(10 * time.Second)
	opt(&cfg)
	if cfg.Timeout != 10*time.Second {
		t.Fatalf("expected custom timeout 10s, got %v", cfg.Timeout)
	}
}
