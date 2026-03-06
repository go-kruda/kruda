// Example: Testing — TestClient Usage Patterns
//
// Demonstrates Kruda's built-in test client for writing tests without
// starting a real HTTP server:
//   - NewTestClient: create an in-memory test client
//   - tc.Get/Post/Put/Delete: shorthand request methods
//   - tc.Request(): fluent request builder with headers, cookies, query params
//   - TestResponse: status code, headers, body, JSON parsing
//
// This file sets up the app. See app_test.go for actual test functions.
//
// Run: go run -tags kruda_stdjson ./examples/testing/
// Test: go test -tags kruda_stdjson ./examples/testing/
package main

import (
	"fmt"
	"sync"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// In-memory store — simple thread-safe map for the example
// ---------------------------------------------------------------------------

type Todo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type TodoStore struct {
	mu    sync.RWMutex
	items map[string]Todo
	seq   int
}

func NewTodoStore() *TodoStore {
	return &TodoStore{items: make(map[string]Todo)}
}

func (s *TodoStore) List() []Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	todos := make([]Todo, 0, len(s.items))
	for _, t := range s.items {
		todos = append(todos, t)
	}
	return todos
}

func (s *TodoStore) Get(id string) (Todo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	return t, ok
}

func (s *TodoStore) Create(title string) Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	id := fmt.Sprintf("%d", s.seq)
	t := Todo{ID: id, Title: title, Done: false}
	s.items[id] = t
	return t
}

// ---------------------------------------------------------------------------
// App factory — shared between main() and tests
// ---------------------------------------------------------------------------

// NewApp creates the Kruda app with all routes registered.
// Extracting this into a function makes it easy to test.
func NewApp() *kruda.App {
	app := kruda.New()
	store := NewTodoStore()

	app.Use(middleware.Recovery())

	// List all todos
	app.Get("/todos", func(c *kruda.Ctx) error {
		return c.JSON(store.List())
	})

	// Get a single todo by ID
	app.Get("/todos/:id", func(c *kruda.Ctx) error {
		id := c.Param("id")
		todo, ok := store.Get(id)
		if !ok {
			return kruda.NotFound("todo not found")
		}
		return c.JSON(todo)
	})

	// Create a new todo (expects JSON body with "title" field)
	app.Post("/todos", func(c *kruda.Ctx) error {
		var input struct {
			Title string `json:"title"`
		}
		if err := c.Bind(&input); err != nil {
			return kruda.BadRequest("invalid JSON body")
		}
		if input.Title == "" {
			return kruda.UnprocessableEntity("title is required")
		}
		todo := store.Create(input.Title)
		return c.Status(201).JSON(todo)
	})

	// Health check
	app.Get("/health", func(c *kruda.Ctx) error {
		return c.JSON(kruda.Map{"status": "ok"})
	})

	return app
}

func main() {
	app := NewApp()
	fmt.Println("Testing example — run: go test -tags kruda_stdjson ./examples/testing/")
	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
