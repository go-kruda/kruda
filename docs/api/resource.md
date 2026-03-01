# Resource

Auto-generate CRUD endpoints from a service interface.

## ResourceService Interface

```go
type ResourceService[T any, ID comparable] interface {
    List(ctx context.Context, page int, limit int) ([]T, int, error)
    Create(ctx context.Context, item T) (T, error)
    Get(ctx context.Context, id ID) (T, error)
    Update(ctx context.Context, id ID, item T) (T, error)
    Delete(ctx context.Context, id ID) error
}
```

Implement this interface to get automatic route generation.

## Resource (package-level function)

```go
func Resource[T any, ID comparable](app *App, path string, svc ResourceService[T, ID], opts ...ResourceOption) *App
```

Registers CRUD routes for the given service:

| Method | Path | Calls |
|--------|------|-------|
| GET | `{path}` | `svc.List(ctx, page, limit)` |
| GET | `{path}/:id` | `svc.Get(ctx, id)` |
| POST | `{path}` | `svc.Create(ctx, item)` |
| PUT | `{path}/:id` | `svc.Update(ctx, id, item)` |
| DELETE | `{path}/:id` | `svc.Delete(ctx, id)` |

```go
kruda.Resource[User, string](app, "/users", userService)
kruda.Resource[Post, int](app, "/posts", postService)
```

## GroupResource (package-level function)

```go
func GroupResource[T any, ID comparable](g *Group, path string, svc ResourceService[T, ID], opts ...ResourceOption) *Group
```

Registers CRUD routes on a route group.

```go
api := app.Group("/api/v1")
kruda.GroupResource[User, string](api, "/users", userService)
```

## ResourceOption

### WithResourceMiddleware

```go
func WithResourceMiddleware(mw ...HandlerFunc) ResourceOption
```

Applies middleware to all generated resource routes.

```go
kruda.Resource[User, string](app, "/users", userService,
    kruda.WithResourceMiddleware(authMiddleware),
)
```

### WithResourceOnly

```go
func WithResourceOnly(methods ...string) ResourceOption
```

Limits which CRUD actions are generated. Values are uppercased: `"LIST"`, `"GET"`, `"CREATE"`, `"UPDATE"`, `"DELETE"`.

```go
kruda.Resource[User, string](app, "/users", userService,
    kruda.WithResourceOnly("LIST", "GET"),
)
```

### WithResourceExcept

```go
func WithResourceExcept(methods ...string) ResourceOption
```

Generate all actions except the specified ones.

```go
kruda.Resource[User, string](app, "/users", userService,
    kruda.WithResourceExcept("DELETE"),
)
```

### WithResourceIDParam

```go
func WithResourceIDParam(param string) ResourceOption
```

Customize the ID route parameter name (default: `"id"`).

```go
kruda.Resource[User, string](app, "/users", userService,
    kruda.WithResourceIDParam("userId"),
)
// Routes use :userId instead of :id
```

## Example

```go
type Todo struct {
    ID   string `json:"id"`
    Text string `json:"text"`
    Done bool   `json:"done"`
}

type TodoService struct {
    mu    sync.RWMutex
    todos map[string]Todo
}

func (s *TodoService) List(ctx context.Context, page, limit int) ([]Todo, int, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]Todo, 0, len(s.todos))
    for _, t := range s.todos {
        result = append(result, t)
    }
    return result, len(result), nil
}

func (s *TodoService) Get(ctx context.Context, id string) (Todo, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    t, ok := s.todos[id]
    if !ok {
        return Todo{}, errors.New("not found")
    }
    return t, nil
}

func (s *TodoService) Create(ctx context.Context, item Todo) (Todo, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    item.ID = generateID()
    s.todos[item.ID] = item
    return item, nil
}

func (s *TodoService) Update(ctx context.Context, id string, item Todo) (Todo, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.todos[id]; !ok {
        return Todo{}, errors.New("not found")
    }
    item.ID = id
    s.todos[id] = item
    return item, nil
}

func (s *TodoService) Delete(ctx context.Context, id string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.todos, id)
    return nil
}

// Register
kruda.Resource[Todo, string](app, "/todos", &TodoService{todos: make(map[string]Todo)})
```
