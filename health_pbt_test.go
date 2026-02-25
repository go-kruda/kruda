package kruda

import (
	"context"
	"errors"
	"testing"
	"testing/quick"
)

// Feature: phase4-ecosystem, Property 11: Health Check Discovery and Status

// ---------------------------------------------------------------------------
// PBT healthy checker types — each is a distinct type to avoid Give collisions.
// ---------------------------------------------------------------------------

type pbtHealthy1 struct{}

func (h *pbtHealthy1) Check(_ context.Context) error { return nil }

type pbtHealthy2 struct{}

func (h *pbtHealthy2) Check(_ context.Context) error { return nil }

type pbtHealthy3 struct{}

func (h *pbtHealthy3) Check(_ context.Context) error { return nil }

type pbtHealthy4 struct{}

func (h *pbtHealthy4) Check(_ context.Context) error { return nil }

type pbtHealthy5 struct{}

func (h *pbtHealthy5) Check(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// PBT unhealthy checker types — each returns a fixed error message.
// ---------------------------------------------------------------------------

type pbtUnhealthy1 struct{ msg string }

func (h *pbtUnhealthy1) Check(_ context.Context) error { return errors.New(h.msg) }

type pbtUnhealthy2 struct{ msg string }

func (h *pbtUnhealthy2) Check(_ context.Context) error { return errors.New(h.msg) }

type pbtUnhealthy3 struct{ msg string }

func (h *pbtUnhealthy3) Check(_ context.Context) error { return errors.New(h.msg) }

// ---------------------------------------------------------------------------
// TestPropertyHealthCheckDiscovery verifies that for a random number of
// healthy (0-5) and unhealthy (0-3) services registered in a container,
// the HealthHandler returns the correct HTTP status and check entries.
// ---------------------------------------------------------------------------

func TestPropertyHealthCheckDiscovery(t *testing.T) {
	// Pre-built pools of healthy and unhealthy instances.
	healthyPool := []any{
		&pbtHealthy1{},
		&pbtHealthy2{},
		&pbtHealthy3{},
		&pbtHealthy4{},
		&pbtHealthy5{},
	}
	unhealthyPool := []any{
		&pbtUnhealthy1{msg: "service1 down"},
		&pbtUnhealthy2{msg: "service2 down"},
		&pbtUnhealthy3{msg: "service3 down"},
	}

	f := func(nHealthy, nUnhealthy uint8) bool {
		healthy := int(nHealthy) % 6     // 0-5
		unhealthy := int(nUnhealthy) % 4 // 0-3

		c := NewContainer()
		for i := 0; i < healthy; i++ {
			_ = c.Give(healthyPool[i])
		}
		for i := 0; i < unhealthy; i++ {
			_ = c.Give(unhealthyPool[i])
		}

		app := New(WithContainer(c))
		app.Get("/health", HealthHandler())
		app.Compile()

		tc := NewTestClient(app)
		resp, err := tc.Get("/health")
		if err != nil {
			return false
		}

		// Property 1: correct HTTP status
		expectedStatus := 200
		if unhealthy > 0 {
			expectedStatus = 503
		}
		if resp.StatusCode() != expectedStatus {
			return false
		}

		// Parse body
		var body map[string]any
		if err := resp.JSON(&body); err != nil {
			return false
		}

		// Property 2: checks map has exactly the right number of entries
		checksRaw, ok := body["checks"]
		if !ok {
			return false
		}
		checks, ok := checksRaw.(map[string]any)
		if !ok {
			return false
		}
		if len(checks) != healthy+unhealthy {
			return false
		}

		// Property 3: count "ok" entries matches healthy count
		okCount := 0
		for _, v := range checks {
			if v == "ok" {
				okCount++
			}
		}
		return okCount == healthy
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
