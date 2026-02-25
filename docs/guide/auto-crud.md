# Auto CRUD

Kruda can generate a full set of CRUD endpoints from a service interface using `app.Resource()`.

## ResourceService Interface

Implement the `ResourceService[T, ID]` interface:

```go
type ResourceService[T any, ID comparable] interface {
    List() ([]T, error)
    Get(id ID) (T, error)
    Create(item T) (T, error)
    Update(id ID, item T) (T, error)
    Delete(id ID) error
}
```

## Registration

Register a resource to auto-generate routes:

```go
type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

type UserService struct {
    // your storage implementation
}

func (s *UserService) List() ([]User, error)                  { /* ... */ }
func (s *UserService) Get(id string) (User, error)            { /* ... */ }
func (s *UserService) Create(item User) (User, error)         { /* ... */ }
func (s *UserService) Update(id string, item User) (User, error) { /* ... */ }
func (s *UserService) Delete(id string) error                  { /* ... */ }

// Register — generates 5 routes automatically
app.Resource("/users", &UserService{})
```

This generates:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/users` | `List()` |
| GET | `/users/:id` | `Get(id)` |
| POST | `/users` | `Create(item)` |
| PUT | `/users/:id` | `Update(id, item)` |
| DELETE | `/users/:id` | `Delete(id)` |

## Custom Options

Customize the generated routes with `ResourceOption`:

```go
app.Resource("/users", &UserService{},
    kruda.WithResourceMiddleware(authMiddleware),
    kruda.WithResourceOnly("list", "get"), // only List and Get
)
```

## Error Mapping Integration

Combine with error mapping for clean error responses:

```go
var ErrUserNotFound = errors.New("user not found")

app.MapError(ErrUserNotFound, 404, "User not found")

// Now Get() returning ErrUserNotFound automatically responds with 404
```

## In-Memory Example

```go
type InMemoryUserService struct {
    mu    sync.RWMutex
    users map[string]User
}

func NewInMemoryUserService() *InMemoryUserService {
    return &InMemoryUserService{users: make(map[string]User)}
}

func (s *InMemoryUserService) List() ([]User, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]User, 0, len(s.users))
    for _, u := range s.users {
        result = append(result, u)
    }
    return result, nil
}

func (s *InMemoryUserService) Get(id string) (User, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    u, ok := s.users[id]
    if !ok {
        return User{}, ErrUserNotFound
    }
    return u, nil
}

// ... Create, Update, Delete implementations
```

See [Resource API](/api/resource) for the full reference.
