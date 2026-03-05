// Example: Auto CRUD — ResourceService with app.Resource()
//
// Demonstrates Kruda's auto-wired CRUD endpoints using the ResourceService
// interface. A single call to kruda.Resource() registers 5 REST endpoints:
//
//	GET    /users          — List all users (paginated)
//	POST   /users          — Create a new user
//	GET    /users/:id      — Get user by ID
//	PUT    /users/:id      — Update user by ID
//	DELETE /users/:id      — Delete user by ID
//
// Also shows DI container integration, health checks, and custom endpoints.
//
// Run: go run -tags kruda_stdjson ./examples/auto-crud/
// Test: curl http://localhost:3000/users
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain model
// ---------------------------------------------------------------------------

// User is the domain entity for this example.
type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ---------------------------------------------------------------------------
// UserService — implements ResourceService[User, string] for auto CRUD
// and HealthChecker for health endpoint auto-discovery.
// ---------------------------------------------------------------------------

// UserService is an in-memory user store that satisfies both
// kruda.ResourceService[User, string] and kruda.HealthChecker.
type UserService struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int
}

// NewUserService creates a new in-memory user service.
func NewUserService() *UserService {
	return &UserService{users: make(map[string]User)}
}

// List returns all users with pagination info.
func (s *UserService) List(_ context.Context, page, limit int) ([]User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out, len(out), nil
}

// Create adds a new user and returns it with an auto-generated ID.
func (s *UserService) Create(_ context.Context, item User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	item.ID = fmt.Sprintf("%d", s.seq)
	s.users[item.ID] = item
	return item, nil
}

// Get retrieves a user by ID. Returns 404 if not found.
func (s *UserService) Get(_ context.Context, id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, kruda.NotFound("user not found")
	}
	return u, nil
}

// Update replaces a user by ID. Returns 404 if not found.
func (s *UserService) Update(_ context.Context, id string, item User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return User{}, kruda.NotFound("user not found")
	}
	item.ID = id
	s.users[id] = item
	return item, nil
}

// Delete removes a user by ID. Returns 404 if not found.
func (s *UserService) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return kruda.NotFound("user not found")
	}
	delete(s.users, id)
	return nil
}

// Check implements kruda.HealthChecker — auto-discovered by HealthHandler.
func (s *UserService) Check(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// Module — groups DI registrations for clean organization
// ---------------------------------------------------------------------------

// UserModule installs user-related services into the DI container.
type UserModule struct{}

// Install registers the UserService singleton in the container.
func (UserModule) Install(c *kruda.Container) error {
	return c.Give(NewUserService())
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// 1. Create DI container and register services
	c := kruda.NewContainer()
	c.Give(NewUserService())

	// 2. Create app with container
	app := kruda.New(kruda.NetHTTP(), kruda.WithContainer(c))
	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// 3. Auto CRUD — one line registers 5 REST endpoints
	//    GET /users, POST /users, GET /users/:id, PUT /users/:id, DELETE /users/:id
	svc := kruda.MustUse[*UserService](c)
	kruda.Resource(app, "/users", svc)

	// 4. Health endpoint — auto-discovers all HealthChecker implementations
	app.Get("/health", kruda.HealthHandler())

	// 5. Custom endpoint using DI resolution from request context
	app.Get("/stats", func(ctx *kruda.Ctx) error {
		svc := kruda.MustResolve[*UserService](ctx)
		_, total, _ := svc.List(ctx.Context(), 1, 100)
		return ctx.JSON(kruda.Map{"total": total})
	})

	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
