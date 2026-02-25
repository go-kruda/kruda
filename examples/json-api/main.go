// Example: JSON API with Typed Handlers
//
// This example demonstrates building a JSON REST API using Kruda's typed
// handler system (C[T]). Typed handlers auto-parse request bodies and path
// parameters — no manual binding needed.
//
// Endpoints:
//
//	POST   /users      — create a user (JSON body auto-parsed via C[T])
//	GET    /users      — list all users
//	GET    /users/:id  — get a single user by ID
//	PUT    /users/:id  — update a user
//	DELETE /users/:id  — delete a user
//
// Run:
//
//	go run -tags kruda_stdjson ./examples/json-api/
//
// Test:
//
//	curl -X POST http://localhost:3000/users -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com"}'
//	curl http://localhost:3000/users
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

// User represents a user in our API. JSON tags control serialization,
// and the `json` tag also tells Kruda's typed handler to parse the
// request body into these fields automatically.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ---------------------------------------------------------------------------
// In-memory store (thread-safe)
// ---------------------------------------------------------------------------

type UserStore struct {
	mu    sync.RWMutex
	users map[string]User
	seq   int
}

func NewUserStore() *UserStore {
	return &UserStore{users: make(map[string]User)}
}

func (s *UserStore) List(_ context.Context) []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, u)
	}
	return out
}

func (s *UserStore) Create(_ context.Context, u User) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	u.ID = fmt.Sprintf("%d", s.seq)
	s.users[u.ID] = u
	return u
}

func (s *UserStore) Get(_ context.Context, id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	return u, ok
}

func (s *UserStore) Update(_ context.Context, id string, u User) (User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return User{}, false
	}
	u.ID = id
	s.users[id] = u
	return u, true
}

func (s *UserStore) Delete(_ context.Context, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

// ---------------------------------------------------------------------------
// Request/response types for typed handlers
// ---------------------------------------------------------------------------

// CreateUserInput is the typed handler input for POST /users.
// The `json` tag tells Kruda to parse these fields from the request body.
type CreateUserInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserByID is the typed handler input for routes with :id parameter.
// The `param` tag tells Kruda to extract the value from the URL path.
type UserByID struct {
	ID string `param:"id"`
}

// UpdateUserInput combines path parameter and JSON body.
// Kruda parses `param` from the URL and `json` from the request body.
type UpdateUserInput struct {
	ID    string `param:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	store := NewUserStore()

	app := kruda.New()

	// Global middleware: recovery from panics, request IDs, logging
	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())

	// POST /users — Create a user using a typed handler.
	// C[CreateUserInput] auto-parses the JSON body into c.In.
	kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
		user := store.Create(c.Context(), User{
			Name:  c.In.Name,
			Email: c.In.Email,
		})
		c.Status(201) // Created
		return &user, nil
	})

	// GET /users — List all users (no typed input needed, use plain handler).
	app.Get("/users", func(c *kruda.Ctx) error {
		users := store.List(c.Context())
		return c.JSON(users)
	})

	// GET /users/:id — Get a single user using a typed handler.
	// C[UserByID] auto-extracts the :id path parameter into c.In.ID.
	kruda.Get[UserByID, User](app, "/users/:id", func(c *kruda.C[UserByID]) (*User, error) {
		user, ok := store.Get(c.Context(), c.In.ID)
		if !ok {
			return nil, kruda.NotFound("user not found")
		}
		return &user, nil
	})

	// PUT /users/:id — Update a user.
	// C[UpdateUserInput] combines param (ID from URL) and json (body fields).
	kruda.Put[UpdateUserInput, User](app, "/users/:id", func(c *kruda.C[UpdateUserInput]) (*User, error) {
		user, ok := store.Update(c.Context(), c.In.ID, User{
			Name:  c.In.Name,
			Email: c.In.Email,
		})
		if !ok {
			return nil, kruda.NotFound("user not found")
		}
		return &user, nil
	})

	// DELETE /users/:id — Delete a user.
	kruda.Delete[UserByID, struct{}](app, "/users/:id", func(c *kruda.C[UserByID]) (*struct{}, error) {
		if !store.Delete(c.Context(), c.In.ID) {
			return nil, kruda.NotFound("user not found")
		}
		return nil, c.NoContent()
	})

	// Start the server
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
