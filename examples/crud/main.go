// Example: Phase 4 Ecosystem — DI, Modules, Resource, Health
package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain
// ---------------------------------------------------------------------------

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ---------------------------------------------------------------------------
// Service — implements ResourceService for auto CRUD + HealthChecker
// ---------------------------------------------------------------------------

type UserService struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int
}

func NewUserService() *UserService {
	return &UserService{users: make(map[string]User)}
}

func (s *UserService) List(_ context.Context, page, limit int) ([]User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out, len(out), nil
}

func (s *UserService) Create(_ context.Context, item User) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	item.ID = fmt.Sprintf("%d", s.seq)
	s.users[item.ID] = item
	return item, nil
}

func (s *UserService) Get(_ context.Context, id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, kruda.NotFound("user not found")
	}
	return u, nil
}

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

func (s *UserService) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return kruda.NotFound("user not found")
	}
	delete(s.users, id)
	return nil
}

// HealthChecker — auto-discovered by HealthHandler
func (s *UserService) Check(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// Module — groups DI registrations
// ---------------------------------------------------------------------------

type UserModule struct{}

func (UserModule) Install(c *kruda.Container) error {
	return c.Give(NewUserService())
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	// Create container + install modules
	c := kruda.NewContainer()
	c.Give(NewUserService())

	app := kruda.New(kruda.WithContainer(c))
	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	// Auto CRUD: GET/POST /users, GET/PUT/DELETE /users/:id
	svc := kruda.MustUse[*UserService](c)
	kruda.Resource(app, "/users", svc)

	// Health endpoint — auto-discovers UserService.Check()
	app.Get("/health", kruda.HealthHandler())

	// Custom handler using Resolve from request context
	app.Get("/stats", func(ctx *kruda.Ctx) error {
		svc := kruda.MustResolve[*UserService](ctx)
		_, total, _ := svc.List(ctx.Context(), 1, 100)
		return ctx.JSON(kruda.Map{"total": total})
	})

	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
