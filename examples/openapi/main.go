// Example: Auto-Generated OpenAPI 3.1 Spec
//
// This example demonstrates Kruda's built-in OpenAPI spec generation.
// Typed handlers (C[T]) auto-generate the spec from your Go structs —
// no manual YAML/JSON, no code generation step, no external tools.
//
// Key concepts:
//   - WithOpenAPIInfo — enable spec generation with API metadata
//   - WithOpenAPIPath — customize the spec endpoint (default: /openapi.json)
//   - WithOpenAPITag — define tag descriptions for grouping
//   - WithDescription / WithTags — per-route metadata via RouteOption
//   - `validate` tags → auto-generate constraints in the schema
//   - `param` / `query` / `json` tags → auto-generate parameters & request bodies
//
// Endpoints:
//
//	GET  /openapi.json         — auto-generated OpenAPI 3.1 spec
//	POST /tasks                — create a task (JSON body)
//	GET  /tasks                — list tasks (query params: status, limit)
//	GET  /tasks/:id            — get task by ID
//	PUT  /tasks/:id            — update a task
//	POST /tasks/:id/comments   — add a comment to a task
//
// Run:
//
//	go run -tags kruda_stdjson ./examples/openapi/
//
// Test:
//
//	curl http://localhost:3000/openapi.json | jq .
//	curl -X POST http://localhost:3000/tasks -H 'Content-Type: application/json' \
//	     -d '{"title":"Buy milk","priority":"high"}'
//	curl "http://localhost:3000/tasks?status=pending&limit=5"
//	curl http://localhost:3000/tasks/1
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/middleware"
)

// ---------------------------------------------------------------------------
// Domain models — struct tags drive the OpenAPI schema
// ---------------------------------------------------------------------------

// Task is the API response model. JSON tags become property names in the spec.
type Task struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Priority  string `json:"priority"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// Comment is a sub-resource of Task.
type Comment struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
	Author string `json:"author"`
	Body   string `json:"body"`
}

// ---------------------------------------------------------------------------
// Typed handler inputs — these generate parameters & request bodies
// ---------------------------------------------------------------------------

// CreateTaskInput → POST /tasks request body.
// `validate` tags auto-generate schema constraints (required, min/max length).
type CreateTaskInput struct {
	Title    string `json:"title" validate:"required,min=1,max=200"`
	Priority string `json:"priority" validate:"required"`
}

// ListTasksInput → GET /tasks query parameters.
// `query` tags generate OpenAPI query parameters automatically.
type ListTasksInput struct {
	Status string `query:"status"`
	Limit  int    `query:"limit" validate:"min=1"`
}

// TaskByID → path parameter for single-task routes.
// `param` tag generates an OpenAPI path parameter.
type TaskByID struct {
	ID string `param:"id" validate:"required"`
}

// UpdateTaskInput → PUT /tasks/:id combines path param + JSON body.
type UpdateTaskInput struct {
	ID       string `param:"id" validate:"required"`
	Title    string `json:"title" validate:"required,min=1,max=200"`
	Priority string `json:"priority"`
	Status   string `json:"status"`
}

// AddCommentInput → POST /tasks/:id/comments.
type AddCommentInput struct {
	TaskID string `param:"id" validate:"required"`
	Author string `json:"author" validate:"required,min=1,max=50"`
	Body   string `json:"body" validate:"required,min=1"`
}

// TaskListResponse wraps a list with metadata.
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
	Total int    `json:"total"`
}

// ---------------------------------------------------------------------------
// In-memory store
// ---------------------------------------------------------------------------

type Store struct {
	mu       sync.RWMutex
	tasks    map[string]Task
	comments map[string][]Comment
	seq      int
}

func NewStore() *Store {
	return &Store{
		tasks:    make(map[string]Task),
		comments: make(map[string][]Comment),
	}
}

func (s *Store) Create(_ context.Context, title, priority string) Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	t := Task{
		ID:        fmt.Sprintf("%d", s.seq),
		Title:     title,
		Priority:  priority,
		Status:    "pending",
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	s.tasks[t.ID] = t
	return t
}

func (s *Store) Get(_ context.Context, id string) (Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[id]
	return t, ok
}

func (s *Store) List(_ context.Context, status string, limit int) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		if status != "" && t.Status != status {
			continue
		}
		out = append(out, t)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (s *Store) Update(_ context.Context, id string, title, priority, status string) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[id]
	if !ok {
		return Task{}, false
	}
	if title != "" {
		t.Title = title
	}
	if priority != "" {
		t.Priority = priority
	}
	if status != "" {
		t.Status = status
	}
	s.tasks[id] = t
	return t, true
}

func (s *Store) AddComment(_ context.Context, taskID, author, body string) (Comment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[taskID]; !ok {
		return Comment{}, false
	}
	s.seq++
	c := Comment{
		ID:     fmt.Sprintf("c%d", s.seq),
		TaskID: taskID,
		Author: author,
		Body:   body,
	}
	s.comments[taskID] = append(s.comments[taskID], c)
	return c, true
}

// ---------------------------------------------------------------------------
// Main — the OpenAPI spec is generated from the typed handlers below
// ---------------------------------------------------------------------------

func main() {
	store := NewStore()

	app := kruda.New(
		kruda.WithValidator(kruda.NewValidator()),

		// ✅ Enable OpenAPI spec generation — this is all you need.
		// The spec is auto-served at /openapi.json (default path).
		kruda.WithOpenAPIInfo("Task Manager API", "1.0.0", "A simple task management API with auto-generated OpenAPI 3.1 spec"),

		// Optional: customize the spec endpoint path.
		// kruda.WithOpenAPIPath("/api/spec.json"),

		// Optional: define tag descriptions for grouping.
		kruda.WithOpenAPITag("tasks", "Task management operations"),
		kruda.WithOpenAPITag("comments", "Task comment operations"),
	)

	app.Use(middleware.Recovery())
	app.Use(middleware.RequestID())
	app.Use(middleware.Logger())

	// POST /tasks — Create a task.
	// WithDescription and WithTags add per-route OpenAPI metadata.
	kruda.Post[CreateTaskInput, Task](app, "/tasks", func(c *kruda.C[CreateTaskInput]) (*Task, error) {
		t := store.Create(c.Context(), c.In.Title, c.In.Priority)
		c.Status(201)
		return &t, nil
	}, kruda.WithDescription("Create a new task"), kruda.WithTags("tasks"))

	// GET /tasks — List tasks with optional filters.
	// Query params (status, limit) auto-appear as OpenAPI parameters.
	kruda.Get[ListTasksInput, TaskListResponse](app, "/tasks", func(c *kruda.C[ListTasksInput]) (*TaskListResponse, error) {
		limit := c.In.Limit
		if limit == 0 {
			limit = 20
		}
		tasks := store.List(c.Context(), c.In.Status, limit)
		return &TaskListResponse{Tasks: tasks, Total: len(tasks)}, nil
	}, kruda.WithDescription("List tasks with optional status filter"), kruda.WithTags("tasks"))

	// GET /tasks/:id — Get a single task.
	// Path param :id auto-appears as an OpenAPI path parameter.
	kruda.Get[TaskByID, Task](app, "/tasks/:id", func(c *kruda.C[TaskByID]) (*Task, error) {
		t, ok := store.Get(c.Context(), c.In.ID)
		if !ok {
			return nil, kruda.NotFound("task not found")
		}
		return &t, nil
	}, kruda.WithDescription("Get a task by ID"), kruda.WithTags("tasks"))

	// PUT /tasks/:id — Update a task.
	// Combines path param (ID) + JSON body (title, priority, status).
	kruda.Put[UpdateTaskInput, Task](app, "/tasks/:id", func(c *kruda.C[UpdateTaskInput]) (*Task, error) {
		t, ok := store.Update(c.Context(), c.In.ID, c.In.Title, c.In.Priority, c.In.Status)
		if !ok {
			return nil, kruda.NotFound("task not found")
		}
		return &t, nil
	}, kruda.WithDescription("Update an existing task"), kruda.WithTags("tasks"))

	// POST /tasks/:id/comments — Add a comment to a task.
	// Shows sub-resource with path param + JSON body.
	kruda.Post[AddCommentInput, Comment](app, "/tasks/:id/comments", func(c *kruda.C[AddCommentInput]) (*Comment, error) {
		cm, ok := store.AddComment(c.Context(), c.In.TaskID, c.In.Author, c.In.Body)
		if !ok {
			return nil, kruda.NotFound("task not found")
		}
		c.Status(201)
		return &cm, nil
	}, kruda.WithDescription("Add a comment to a task"), kruda.WithTags("comments"))

	// The OpenAPI spec is auto-served at GET /openapi.json — no extra code needed.
	// Try: curl http://localhost:3000/openapi.json | jq .

	if err := app.Listen(":3000"); err != nil {
		panic(err)
	}
}
