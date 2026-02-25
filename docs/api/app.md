# App

The `App` is the core of a Kruda application. It manages routing, middleware, DI container, and the server lifecycle.

## Constructor

### New

```go
func New(opts ...Option) *App
```

Creates a new Kruda application with the given options.

```go
app := kruda.New()

app := kruda.New(
    kruda.WithReadTimeout(30 * time.Second),
    kruda.WithBodyLimit(4 * 1024 * 1024),
    kruda.WithDevMode(true),
)
```

## Route Registration

### Get

```go
func (a *App) Get(path string, handler HandlerFunc, middleware ...MiddlewareFunc) *App
```

Registers a GET route.

```go
app.Get("/users", listUsers)
app.Get("/users/:id", getUser)
```

### Post

```go
func (a *App) Post(path string, handler HandlerFunc, middleware ...MiddlewareFunc) *App
```

Registers a POST route.

### Put

```go
func (a *App) Put(path string, handler HandlerFunc, middleware ...MiddlewareFunc) *App
```

Registers a PUT route.

### Delete

```go
func (a *App) Delete(path string, handler HandlerFunc, middleware ...MiddlewareFunc) *App
```

Registers a DELETE route.

### Patch

```go
func (a *App) Patch(path string, handler HandlerFunc, middleware ...MiddlewareFunc) *App
```

Registers a PATCH route.

## Middleware

### Use

```go
func (a *App) Use(middleware ...MiddlewareFunc) *App
```

Registers global middleware that runs on every request.

```go
app.Use(middleware.Logger(), middleware.Recovery())
```

## Groups

### Group

```go
func (a *App) Group(prefix string, middleware ...MiddlewareFunc) *Group
```

Creates a route group with a shared prefix and optional middleware.

```go
api := app.Group("/api/v1")
api.Get("/users", listUsers)

admin := app.Group("/admin", authMiddleware)
admin.Get("/stats", getStats)
```

## Server Lifecycle

### Listen

```go
func (a *App) Listen(addr string) error
```

Starts the HTTP server on the given address. Blocks until the server is shut down.

```go
app.Listen(":3000")
app.Listen("127.0.0.1:8080")
```

### Shutdown

```go
func (a *App) Shutdown(ctx context.Context) error
```

Gracefully shuts down the server. In-flight requests are allowed to complete before the given context deadline.

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
app.Shutdown(ctx)
```

## Resource

### Resource

```go
func (a *App) Resource(prefix string, svc ResourceService[T, ID], opts ...ResourceOption) *App
```

Registers auto-generated CRUD routes for a resource service. See [Resource API](/api/resource).

## Error Mapping

### MapError

```go
func (a *App) MapError(err error, code int, message string) *App
```

Maps a specific error to an HTTP status code and message.

### MapErrorType

```go
func (a *App) MapErrorType(t reflect.Type, code int, message string) *App
```

Maps all errors of a given type to an HTTP status code and message.

### MapErrorFunc

```go
func (a *App) MapErrorFunc(fn func(error) *KrudaError) *App
```

Registers a custom error mapping function.

## Module

### Module

```go
func (a *App) Module(m Module) *App
```

Registers a DI module. See [Container API](/api/container).

## Functional Options

See [Config API](/api/config) for all `WithXxx` options.
