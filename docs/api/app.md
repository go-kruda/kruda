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
func (app *App) Get(path string, handler HandlerFunc) *App
```

Registers a GET route.

```go
app.Get("/users", listUsers)
app.Get("/users/:id", getUser)
```

### Post

```go
func (app *App) Post(path string, handler HandlerFunc) *App
```

Registers a POST route.

### Put

```go
func (app *App) Put(path string, handler HandlerFunc) *App
```

Registers a PUT route.

### Delete

```go
func (app *App) Delete(path string, handler HandlerFunc) *App
```

Registers a DELETE route.

### Patch

```go
func (app *App) Patch(path string, handler HandlerFunc) *App
```

Registers a PATCH route.

### Options

```go
func (app *App) Options(path string, handler HandlerFunc) *App
```

Registers an OPTIONS route.

### Head

```go
func (app *App) Head(path string, handler HandlerFunc) *App
```

Registers a HEAD route.

### All

```go
func (app *App) All(path string, handler HandlerFunc) *App
```

Registers a handler for all HTTP methods.

## Middleware

### Use

```go
func (app *App) Use(middleware ...HandlerFunc) *App
```

Registers global middleware that runs on every request.

```go
app.Use(middleware.Logger(), middleware.Recovery())
```

## Groups

### Group

```go
func (app *App) Group(prefix string, middleware ...HandlerFunc) *Group
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
func (app *App) Listen(addr string) error
```

Starts the HTTP server on the given address. Blocks until the server is shut down.

```go
app.Listen(":3000")
app.Listen("127.0.0.1:8080")
```

### Shutdown

```go
func (app *App) Shutdown(ctx context.Context) error
```

Gracefully shuts down the server. In-flight requests are allowed to complete before the given context deadline.

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
app.Shutdown(ctx)
```

### OnShutdown

```go
func (app *App) OnShutdown(fn func()) *App
```

Registers a shutdown hook.

## Resource (package-level function)

```go
func Resource[T any, ID comparable](app *App, path string, svc ResourceService[T, ID], opts ...ResourceOption) *App
```

Registers auto-generated CRUD routes for a resource service. This is a generic package-level function.

```go
kruda.Resource[User, string](app, "/users", &UserService{})
```

See [Resource API](/api/resource).

## Error Mapping

### MapError (method on App)

```go
func (app *App) MapError(err error, code int, message string) *App
```

Maps a specific error to an HTTP status code and message.

### MapErrorType (package-level generic function)

```go
func MapErrorType[T error](app *App, statusCode int, message string)
```

Maps all errors of a given type to an HTTP status code and message.

```go
kruda.MapErrorType[*ValidationError](app, 422, "Validation failed")
```

### MapErrorFunc (package-level function)

```go
func MapErrorFunc(app *App, target error, fn func(error) *KrudaError)
```

Registers a custom error mapping function for a specific target error.

```go
kruda.MapErrorFunc(app, ErrDB, func(err error) *kruda.KrudaError {
    return kruda.NewError(500, "database error")
})
```

## Module

### Module

```go
func (app *App) Module(m Module) *App
```

Installs a DI module. The module's `Install(*Container) error` method is called. See [Container API](/api/container).

## Hooks

### OnParse

```go
func (app *App) OnParse(fn func(c *Ctx, input any) error) *App
```

Registers a hook that runs after input parsing but before validation in typed handlers.

### Validator

```go
func (app *App) Validator() *Validator
```

Returns the app's validator for registering custom validation rules.

## Functional Options

See [Config API](/api/config) for all `WithXxx` options.
