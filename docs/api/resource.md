# Resource

Auto-generate CRUD endpoints from a service interface.

## ResourceService Interface

```go
type ResourceService[T any, ID comparable] interface {
    List() ([]T, error)
    Get(id ID) (T, error)
    Create(item T) (T, error)
    Update(id ID, item T) (T, error)
    Delete(id ID) error
}
```

Implement this interface to get automatic route generation.

## App.Resource

```go
func (a *App) Resource(prefix string, svc ResourceService[T, ID], opts ...ResourceOption) *App
```

Registers CRUD routes for the given service:

| Method | Path | Calls |
|--------|------|-------|
| GET | `{prefix}` | `svc.List()` |
| GET | `{prefix}/:id` | `svc.Get(id)` |
| POST | `{prefix}` | `svc.Create(item)` |
| PUT | `{prefix}/:id` | `svc.Update(id, item)` |
| DELETE | `{prefix}/:id` | `svc.Delete(id)` |

```go
app.Resource("/users", userService)
app.Resource("/posts", postService)
```

## ResourceOption

### WithResourceMiddleware

```go
func WithResourceMiddleware(mw ...MiddlewareFunc) ResourceOption
```

Applies middleware to all generated resource routes.

```go
app.Resource("/users", userService,
    kruda.WithResourceMiddleware(authMiddleware),
)
```

### WithResourceOnly

```go
func WithResourceOnly(actions ...string) ResourceOption
```

Limits which CRUD actions are generated. Valid actions: `"list"`, `"get"`, `"create"`, `"update"`, `"delete"`.

```go
// Only generate List and Get routes
app.Resource("/users", userService,
    kruda.WithResourceOnly("list", "get"),
)
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

func (s *TodoService) List() ([]Todo, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]Todo, 0, len(s.todos))
    for _, t := range s.todos {
        result = append(result, t)
    }
    return result, nil
}

func (s *TodoService) Get(id string) (Todo, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    t, ok := s.todos[id]
    if !ok {
        return Todo{}, errors.New("not found")
    }
    return t, nil
}

func (s *TodoService) Create(item Todo) (Todo, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    item.ID = generateID()
    s.todos[item.ID] = item
    return item, nil
}

func (s *TodoService) Update(id string, item Todo) (Todo, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if _, ok := s.todos[id]; !ok {
        return Todo{}, errors.New("not found")
    }
    item.ID = id
    s.todos[id] = item
    return item, nil
}

func (s *TodoService) Delete(id string) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    delete(s.todos, id)
    return nil
}

// Register
app.Resource("/todos", &TodoService{todos: make(map[string]Todo)})
```
