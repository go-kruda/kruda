// Example: Database — Repository Pattern with In-Memory Store
//
// Demonstrates how to structure database access in a Kruda application:
//   - Repository interface: abstracts data access
//   - In-memory implementation: for testing and prototyping
//   - DI integration: register repo in container, resolve in handlers
//   - Module pattern: group related DI registrations
//
// In a real app, swap InMemoryUserRepo for a PostgresUserRepo, etc.
//
// Run: go run -tags kruda_stdjson ./examples/database/
// Test:
//
//	curl http://localhost:3000/users
//	curl -X POST http://localhost:3000/users -d '{"name":"Alice","email":"alice@example.com"}'
//	curl http://localhost:3000/users/1
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

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ---------------------------------------------------------------------------
// Repository interface — abstracts data access
// ---------------------------------------------------------------------------

// UserRepository defines the contract for user data access.
// In production, implement this with your database driver (pgx, sqlx, etc.).
type UserRepository interface {
	FindAll(ctx context.Context) ([]User, error)
	FindByID(ctx context.Context, id string) (User, error)
	Create(ctx context.Context, u User) (User, error)
}

// ---------------------------------------------------------------------------
// In-memory implementation — swap this for a real DB in production
// ---------------------------------------------------------------------------

// InMemoryUserRepo implements UserRepository with a thread-safe map.
// Perfect for testing, prototyping, and examples.
type InMemoryUserRepo struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int
}

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{
		users: map[string]User{
			"1": {ID: "1", Name: "Alice", Email: "alice@example.com"},
			"2": {ID: "2", Name: "Bob", Email: "bob@example.com"},
		},
		seq: 2,
	}
}

func (r *InMemoryUserRepo) FindAll(_ context.Context) ([]User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	users := make([]User, 0, len(r.users))
	for _, u := range r.users {
		users = append(users, u)
	}
	return users, nil
}

func (r *InMemoryUserRepo) FindByID(_ context.Context, id string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return User{}, fmt.Errorf("user %s not found", id)
	}
	return u, nil
}

func (r *InMemoryUserRepo) Create(_ context.Context, u User) (User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	u.ID = fmt.Sprintf("%d", r.seq)
	r.users[u.ID] = u
	return u, nil
}

// ---------------------------------------------------------------------------
// DI Module — groups related registrations
// ---------------------------------------------------------------------------

// DatabaseModule registers all database-related services into the DI container.
// Using a Module keeps main() clean and makes registrations reusable.
type DatabaseModule struct{}

func (DatabaseModule) Install(c *kruda.Container) error {
	// Register the concrete type — handlers resolve via the interface
	repo := NewInMemoryUserRepo()
	// GiveAs registers the instance as the UserRepository interface
	return c.GiveAs(repo, (*UserRepository)(nil))
}

// ---------------------------------------------------------------------------
// Handlers — resolve repository from DI
// ---------------------------------------------------------------------------

func listUsersHandler(c *kruda.Ctx) error {
	// Resolve the UserRepository from the DI container.
	// MustResolve panics if not found — appropriate for required services.
	repo := kruda.MustResolve[UserRepository](c)

	users, err := repo.FindAll(c.Context())
	if err != nil {
		return kruda.InternalError("failed to list users")
	}
	return c.JSON(users)
}

func getUserHandler(c *kruda.Ctx) error {
	repo := kruda.MustResolve[UserRepository](c)

	user, err := repo.FindByID(c.Context(), c.Param("id"))
	if err != nil {
		return kruda.NotFound("user not found")
	}
	return c.JSON(user)
}

func createUserHandler(c *kruda.Ctx) error {
	repo := kruda.MustResolve[UserRepository](c)

	var input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := c.Bind(&input); err != nil {
		return kruda.BadRequest("invalid JSON body")
	}
	if input.Name == "" || input.Email == "" {
		return kruda.UnprocessableEntity("name and email are required")
	}

	user, err := repo.Create(c.Context(), User{Name: input.Name, Email: input.Email})
	if err != nil {
		return kruda.InternalError("failed to create user")
	}
	return c.Status(201).JSON(user)
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	app := kruda.New(kruda.NetHTTP())

	// Install the database module — registers UserRepository in DI.
	// app.Module() creates a container if none exists, then calls Install().
	app.Module(DatabaseModule{})

	app.Use(middleware.Recovery())
	app.Use(middleware.Logger())

	app.Get("/", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{
			"message": "Database integration example",
			"try": kruda.Map{
				"GET /users":     "list all users",
				"GET /users/:id": "get user by ID",
				"POST /users":    "create user (JSON: name, email)",
			},
		})
	})

	app.Get("/users", listUsersHandler)
	app.Get("/users/:id", getUserHandler)
	app.Post("/users", createUserHandler)

	fmt.Println("Database example listening on :3000")
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
