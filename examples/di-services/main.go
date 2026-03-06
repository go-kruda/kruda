// Example: DI Container with Services and Modules
//
// This example demonstrates Kruda's built-in dependency injection (DI)
// container. DI lets you register services once and resolve them anywhere
// in your handlers — no global variables needed.
//
// Key concepts:
//   - Container — holds registered services
//   - Give()    — register a service instance in the container
//   - Use[T]()  — resolve a service by type from the container
//   - Resolve[T]() — resolve from request context (in handlers)
//   - MustResolve[T]() — like Resolve but panics on error
//   - Module    — groups related service registrations
//   - app.Module() — install a module into the app
//
// Architecture:
//
//	┌─────────────┐     ┌──────────────┐     ┌──────────────┐
//	│  Container   │────▶│ UserService  │────▶│  UserRepo    │
//	│  (Give/Use)  │     │ (business)   │     │  (storage)   │
//	│              │────▶│ EmailService │     └──────────────┘
//	└─────────────┘     └──────────────┘
//	       │
//	       ▼
//	┌─────────────┐
//	│   Handler    │  ← MustResolve[*UserService](c)
//	└─────────────┘
//
// Run:
//
//	go run -tags kruda_stdjson ./examples/di-services/
//
// Test:
//
//	curl http://localhost:3000/users
//	curl -X POST http://localhost:3000/users -H 'Content-Type: application/json' \
//	     -d '{"name":"Alice","email":"alice@example.com"}'
//	curl http://localhost:3000/users/1
//	curl http://localhost:3000/stats
package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain model
// ---------------------------------------------------------------------------

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ---------------------------------------------------------------------------
// Repository layer — data access
// ---------------------------------------------------------------------------

// UserRepo handles data storage. In a real app this would talk to a database.
type UserRepo struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int
}

func NewUserRepo() *UserRepo {
	return &UserRepo{users: make(map[string]User)}
}

func (r *UserRepo) FindAll(_ context.Context) []User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	return out
}

func (r *UserRepo) FindByID(_ context.Context, id string) (User, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	return u, ok
}

func (r *UserRepo) Save(_ context.Context, u User) User {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	u.ID = fmt.Sprintf("%d", r.seq)
	r.users[u.ID] = u
	return u
}

func (r *UserRepo) Count(_ context.Context) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.users)
}

// ---------------------------------------------------------------------------
// Service layer — business logic
// ---------------------------------------------------------------------------

// EmailService is a simple notification service.
// In a real app this would send actual emails.
type EmailService struct {
	logger *slog.Logger
}

func NewEmailService() *EmailService {
	return &EmailService{logger: slog.Default()}
}

func (e *EmailService) SendWelcome(email, name string) {
	e.logger.Info("sending welcome email", "to", email, "name", name)
}

// UserService orchestrates user operations using injected dependencies.
// It depends on UserRepo (storage) and EmailService (notifications).
type UserService struct {
	repo  *UserRepo
	email *EmailService
}

// NewUserService creates a UserService with its dependencies.
// In DI terms, this is the "constructor" — dependencies are passed in.
func NewUserService(repo *UserRepo, email *EmailService) *UserService {
	return &UserService{repo: repo, email: email}
}

func (s *UserService) ListUsers(ctx context.Context) []User {
	return s.repo.FindAll(ctx)
}

func (s *UserService) GetUser(ctx context.Context, id string) (User, bool) {
	return s.repo.FindByID(ctx, id)
}

func (s *UserService) CreateUser(ctx context.Context, name, email string) User {
	user := s.repo.Save(ctx, User{Name: name, Email: email})
	// Send welcome email after creating user
	s.email.SendWelcome(email, name)
	return user
}

func (s *UserService) UserCount(ctx context.Context) int {
	return s.repo.Count(ctx)
}

// ---------------------------------------------------------------------------
// Module — groups related DI registrations
//
// Modules are reusable units of DI configuration. They implement the
// kruda.Module interface with an Install(c *Container) method.
// This keeps service registration organized and testable.
// ---------------------------------------------------------------------------

// UserModule registers all user-related services into the container.
type UserModule struct{}

// Install registers UserRepo, EmailService, and UserService.
// Services are registered in dependency order — repo and email first,
// then the service that depends on them.
func (UserModule) Install(c *kruda.Container) error {
	// 1. Register the repository (no dependencies)
	repo := NewUserRepo()
	if err := c.Give(repo); err != nil {
		return err
	}

	// 2. Register the email service (no dependencies)
	email := NewEmailService()
	if err := c.Give(email); err != nil {
		return err
	}

	// 3. Register the user service (depends on repo + email)
	//    We resolve dependencies from the container, then register the service.
	svc := NewUserService(repo, email)
	return c.Give(svc)
}

// NotificationModule is another module showing how to organize services.
// In a real app, you might have separate modules for auth, billing, etc.
type NotificationModule struct{}

func (NotificationModule) Install(c *kruda.Container) error {
	// This module reuses EmailService from UserModule (already registered).
	// Modules can share services through the container.
	return nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// -----------------------------------------------------------------------
	// Option A: Manual container setup
	//
	// Create a container, register services manually, pass to app.
	// This gives you full control over registration order.
	// -----------------------------------------------------------------------

	// Create a new DI container
	c := kruda.NewContainer()

	// Register services using Give() — the container stores them by type.
	// Give() accepts any value and registers it keyed by its concrete type.
	c.Give(NewUserRepo())
	c.Give(NewEmailService())

	// Resolve dependencies from the container to wire up the service.
	// Use[T]() retrieves a service by type — returns (T, error).
	// MustUse[T]() is the same but panics on error (for init-time wiring).
	repo := kruda.MustUse[*UserRepo](c)
	email := kruda.MustUse[*EmailService](c)
	c.Give(NewUserService(repo, email))

	// Create the app with the container
	app := kruda.New(kruda.WithContainer(c))

	// -----------------------------------------------------------------------
	// Option B: Module-based setup (commented out — use one or the other)
	//
	// Modules group related registrations. Install via app.Module().
	// This is cleaner for larger apps with many services.
	//
	//   app := kruda.New()
	//   app.Module(UserModule{})
	//
	// The Module's Install() method receives the container and registers
	// all services. If no container exists, app.Module() creates one.
	// -----------------------------------------------------------------------

	// Global middleware
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())

	// -----------------------------------------------------------------------
	// Handler 1: GET /users — Resolve service from request context
	//
	// MustResolve[T](c) resolves a service from the DI container
	// attached to the current request. This is the primary way to
	// access services in handlers.
	// -----------------------------------------------------------------------
	app.Get("/users", func(c *kruda.Ctx) error {
		// Resolve UserService from the request's DI container.
		// The container is automatically available via WithContainer().
		svc := kruda.MustResolve[*UserService](c)
		users := svc.ListUsers(c.Context())
		return c.JSON(users)
	})

	// -----------------------------------------------------------------------
	// Handler 2: GET /users/:id — Resolve + use path param
	// -----------------------------------------------------------------------
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		svc := kruda.MustResolve[*UserService](c)
		user, ok := svc.GetUser(c.Context(), c.Param("id"))
		if !ok {
			return kruda.NotFound("user not found")
		}
		return c.JSON(user)
	})

	// -----------------------------------------------------------------------
	// Handler 3: POST /users — Create user via service
	// -----------------------------------------------------------------------
	app.Post("/users", func(c *kruda.Ctx) error {
		var input struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		if err := c.Bind(&input); err != nil {
			return err
		}

		svc := kruda.MustResolve[*UserService](c)
		user := svc.CreateUser(c.Context(), input.Name, input.Email)
		return c.Status(201).JSON(user)
	})

	// -----------------------------------------------------------------------
	// Handler 4: GET /stats — Demonstrate resolving multiple services
	//
	// You can resolve any registered service type. Here we resolve both
	// UserService and UserRepo to show different access patterns.
	// -----------------------------------------------------------------------
	app.Get("/stats", func(c *kruda.Ctx) error {
		svc := kruda.MustResolve[*UserService](c)
		return c.JSON(kruda.Map{
			"total_users": svc.UserCount(c.Context()),
		})
	})

	// -----------------------------------------------------------------------
	// Handler 5: GET /health — Direct repo access
	//
	// You can resolve any registered type, not just "services".
	// Here we resolve the repo directly for a health check.
	// -----------------------------------------------------------------------
	app.Get("/health", func(c *kruda.Ctx) error {
		// Resolve[T]() returns (T, error) — use when you want to handle
		// the error gracefully instead of panicking.
		_, err := kruda.Resolve[*UserRepo](c)
		if err != nil {
			return c.Status(503).JSON(kruda.Map{"status": "unhealthy", "error": err.Error()})
		}
		return c.JSON(kruda.Map{"status": "healthy"})
	})

	// Start the server
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
