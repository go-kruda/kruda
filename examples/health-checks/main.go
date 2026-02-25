// Example: Health Checks — HealthChecker Services and HealthHandler
//
// Demonstrates Kruda's health check system:
//   - HealthChecker interface: services report their own health
//   - HealthHandler(): auto-discovers checkers from DI container
//   - Healthy and unhealthy states with proper HTTP status codes
//   - WithHealthTimeout: configure check timeout
//
// Run: go run -tags kruda_stdjson ./examples/health-checks/
// Test:
//
//	curl http://localhost:3000/health     → 200 when all healthy
//	curl http://localhost:3000/break      → makes DB unhealthy
//	curl http://localhost:3000/health     → 503 when DB is down
//	curl http://localhost:3000/fix        → restores DB health
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// DatabaseService — implements HealthChecker
// ---------------------------------------------------------------------------

// DatabaseService simulates a database connection that can be healthy or not.
// It implements kruda.HealthChecker so the health endpoint discovers it
// automatically from the DI container.
type DatabaseService struct {
	mu      sync.RWMutex
	healthy bool
}

func NewDatabaseService() *DatabaseService {
	return &DatabaseService{healthy: true}
}

// Check implements kruda.HealthChecker.
// The context should be respected to avoid goroutine leaks on timeout.
func (db *DatabaseService) Check(ctx context.Context) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Simulate a quick ping — respect context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !db.healthy {
		return errors.New("database connection lost")
	}
	return nil
}

// SetHealthy toggles the database health state (for demo purposes).
func (db *DatabaseService) SetHealthy(healthy bool) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.healthy = healthy
}

// ---------------------------------------------------------------------------
// CacheService — another HealthChecker
// ---------------------------------------------------------------------------

// CacheService simulates a cache layer. Always healthy in this example,
// but demonstrates multiple health checkers running in parallel.
type CacheService struct{}

func NewCacheService() *CacheService { return &CacheService{} }

// Check implements kruda.HealthChecker.
func (c *CacheService) Check(ctx context.Context) error {
	// Simulate a fast cache ping
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1 * time.Millisecond):
		return nil // cache is always healthy
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Create services
	db := NewDatabaseService()
	cache := NewCacheService()

	// Create DI container and register services.
	// HealthHandler auto-discovers any service implementing HealthChecker.
	container := kruda.NewContainer()
	container.Give(db)
	container.Give(cache)

	app := kruda.New(
		kruda.WithContainer(container),
	)
	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// Inject the container into request context so HealthHandler can discover checkers
	app.Use(container.InjectMiddleware())

	// -----------------------------------------------------------------------
	// Health endpoint — auto-discovers all HealthChecker implementations
	// from the DI container and runs them in parallel.
	//
	// Response format:
	//   200: {"status":"ok","checks":{"DatabaseService":"ok","CacheService":"ok"}}
	//   503: {"status":"unhealthy","checks":{"DatabaseService":"database connection lost",...}}
	// -----------------------------------------------------------------------
	app.Get("/health", kruda.HealthHandler(
		kruda.WithHealthTimeout(3*time.Second), // timeout for all checks
	))

	// Demo endpoints to toggle database health
	app.Get("/break", func(c *kruda.Ctx) error {
		db.SetHealthy(false)
		return c.JSON(kruda.Map{"message": "database marked as unhealthy"})
	})

	app.Get("/fix", func(c *kruda.Ctx) error {
		db.SetHealthy(true)
		return c.JSON(kruda.Map{"message": "database restored to healthy"})
	})

	app.Get("/", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{
			"message": "Health checks example",
			"try": kruda.Map{
				"GET /health": "check all services (200 or 503)",
				"GET /break":  "make database unhealthy",
				"GET /fix":    "restore database health",
			},
		})
	})

	fmt.Println("Health checks example listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
