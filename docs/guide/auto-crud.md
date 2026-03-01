# Auto CRUD

Kruda can generate a full set of CRUD endpoints from a service interface using the `Resource` package-level function.

## ResourceService Interface

Implement the `ResourceService[T, ID]` interface:

```go
type ResourceService[T any, ID comparable] interface {
    List(ctx context.Context, page int, limit int) ([]T, int, error)
    Create(ctx context.Context, item T) (T, error)
    Get(ctx context.Context, id ID) (T, error)
    Update(ctx context.Context, id ID, item T) (T, error)
    Delete(ctx context.Context, id ID) error
}
```

Note: `List` returns items, total count, and error. It receives `page` and `limit` for pagination.

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

func (s *UserService) List(ctx context.Context, page, limit int) ([]User, int, error) { /* ... */ }
func (s *UserService) Get(ctx context.Context, id string) (User, error)               { /* ... */ }
func (s *UserService) Create(ctx context.Context, item User) (User, error)            { /* ... */ }
func (s *UserService) Update(ctx context.Context, id string, item User) (User, error) { /* ... */ }
func (s *UserService) Delete(ctx context.Context, id string) error                    { /* ... */ }

// Register — generates 5 routes automatically
kruda.Resource[User, string](app, "/users", &UserService{})
```

This generates:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/users` | `List(ctx, page, limit)` |
| GET | `/users/:id` | `Get(ctx, id)` |
| POST | `/users` | `Create(ctx, item)` |
| PUT | `/users/:id` | `Update(ctx, id, item)` |
| DELETE | `/users/:id` | `Delete(ctx, id)` |

## Group Resource

Register resources on a route group:

```go
func GroupResource[T any, ID comparable](g *Group, path string, svc ResourceService[T, ID], opts ...ResourceOption) *Group
```

```go
api := app.Group("/api/v1")
kruda.GroupResource[User, string](api, "/users", &UserService{})
```

## Custom Options

Customize the generated routes with `ResourceOption`:

```go
kruda.Resource[User, string](app, "/users", &UserService{},
    kruda.WithResourceMiddleware(authMiddleware),
    kruda.WithResourceOnly("LIST", "GET"),
    kruda.WithResourceIDParam("userId"),
)
```

### WithResourceMiddleware

```go
func WithResourceMiddleware(mw ...HandlerFunc) ResourceOption
```

### WithResourceOnly

```go
func WithResourceOnly(methods ...string) ResourceOption
```

Only generate specified actions (uppercased: `"LIST"`, `"GET"`, `"CREATE"`, `"UPDATE"`, `"DELETE"`).

### WithResourceExcept

```go
func WithResourceExcept(methods ...string) ResourceOption
```

Generate all actions except the specified ones.

### WithResourceIDParam

```go
func WithResourceIDParam(param string) ResourceOption
```

Customize the ID parameter name (default: `"id"`).

## Error Mapping Integration

Combine with error mapping for clean error responses:

```go
var ErrUserNotFound = errors.New("user not found")

app.MapError(ErrUserNotFound, 404, "User not found")

// Now Get() returning ErrUserNotFound automatically responds with 404
```

See [Resource API](/api/resource) for the full reference.
