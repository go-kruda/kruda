package kruda

import (
	"context"
	"reflect"
	"time"
)

// HealthChecker is implemented by services that can report their health status.
// Implementations MUST respect ctx.Done() to avoid goroutine leaks on timeout.
type HealthChecker interface {
	Check(ctx context.Context) error
}

// HealthOption configures the health check handler.
type HealthOption func(*healthConfig)

type healthConfig struct {
	Timeout time.Duration
}

func defaultHealthConfig() healthConfig {
	return healthConfig{Timeout: 5 * time.Second}
}

// WithHealthTimeout sets the timeout for health checks.
func WithHealthTimeout(d time.Duration) HealthOption {
	return func(cfg *healthConfig) { cfg.Timeout = d }
}

type namedChecker struct {
	name    string
	checker HealthChecker
}

// HealthHandler returns a HandlerFunc that discovers all HealthChecker
// implementations from the DI container and runs them in parallel.
func HealthHandler(opts ...HealthOption) HandlerFunc {
	cfg := defaultHealthConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(c *Ctx) error {
		checkers := discoverHealthCheckers(c.app.container)
		if len(checkers) == 0 {
			return c.JSON(Map{"status": "ok", "checks": Map{}})
		}
		results := runHealthChecks(c.Context(), checkers, cfg.Timeout)
		return renderHealthResponse(c, results)
	}
}

// discoverHealthCheckers scans all singletons, resolved lazy singletons,
// and named instances for HealthChecker implementations.
func discoverHealthCheckers(c *Container) []namedChecker {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	var checkers []namedChecker
	seen := make(map[any]bool)
	addChecker := func(t reflect.Type, inst any) {
		if seen[inst] {
			return
		}
		seen[inst] = true
		if hc, ok := inst.(HealthChecker); ok {
			name := t.String()
			if t.Kind() == reflect.Ptr {
				name = t.Elem().Name()
			}
			if name == "" {
				name = t.String()
			}
			checkers = append(checkers, namedChecker{name: name, checker: hc})
		}
	}
	for t, inst := range c.singletons {
		addChecker(t, inst)
	}
	for t, entry := range c.lazies {
		if entry.done.Load() {
			addChecker(t, entry.instance)
		}
	}
	for name, inst := range c.named {
		if seen[inst] {
			continue
		}
		seen[inst] = true
		if hc, ok := inst.(HealthChecker); ok {
			checkers = append(checkers, namedChecker{name: name, checker: hc})
		}
	}
	return checkers
}

// runHealthChecks executes all checks in parallel with timeout.
func runHealthChecks(ctx context.Context, checkers []namedChecker, timeout time.Duration) map[string]string {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type result struct {
		name string
		err  error
	}
	ch := make(chan result, len(checkers))
	for _, nc := range checkers {
		go func(nc namedChecker) {
			err := nc.checker.Check(ctx)
			ch <- result{name: nc.name, err: err}
		}(nc)
	}

	results := make(map[string]string, len(checkers))
loop:
	for range checkers {
		select {
		case r := <-ch:
			if r.err != nil {
				results[r.name] = r.err.Error()
			} else {
				results[r.name] = "ok"
			}
		case <-ctx.Done():
			// Drain any results that arrived before we label them timed out.
			for {
				select {
				case r := <-ch:
					if r.err != nil {
						results[r.name] = r.err.Error()
					} else {
						results[r.name] = "ok"
					}
				default:
					break loop
				}
			}
		}
	}
	for _, nc := range checkers {
		if _, ok := results[nc.name]; !ok {
			results[nc.name] = "health check timed out"
		}
	}
	return results
}

// renderHealthResponse writes the health check JSON response.
func renderHealthResponse(c *Ctx, checks map[string]string) error {
	healthy := true
	for _, v := range checks {
		if v != "ok" {
			healthy = false
			break
		}
	}
	status := "ok"
	httpStatus := 200
	if !healthy {
		status = "unhealthy"
		httpStatus = 503
	}
	return c.Status(httpStatus).JSON(Map{
		"status": status,
		"checks": checks,
	})
}
