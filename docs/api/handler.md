# Handler

## HandlerFunc

```go
type HandlerFunc func(c *Ctx) error
```

The base handler type. Every route handler and middleware is a `HandlerFunc`.

```go
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.Text("Hello!")
})
```

## MiddlewareFunc

```go
type MiddlewareFunc = HandlerFunc
```

`MiddlewareFunc` is a type alias for `HandlerFunc`. They are fully interchangeable.

```go
func MyMiddleware(c *kruda.Ctx) error {
    // before handler
    err := c.Next()
    // after handler
    return err
}
```

## C[T] — Typed Context

```go
type C[T any] struct {
    *Ctx
    In T // parsed input from request
}
```

`C[T]` extends `Ctx` with a typed `In` field. The request body, route parameters, and query parameters are automatically parsed and validated into `T` before the handler runs.

## Typed Handler Registration

Package-level generic functions register typed handlers with pre-compiled binding and validation:

```go
func Get[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func Post[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func Put[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func Delete[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func Patch[In any, Out any](app *App, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
```

```go
type CreateUserInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
}

kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
    return &User{ID: "1", Name: c.In.Name}, nil
})
```

### Short Handlers (no error return)

For prototyping — panics are caught by Recovery middleware:

```go
func GetX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption)
func PostX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption)
func PutX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption)
func DeleteX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption)
func PatchX[In any, Out any](app *App, path string, handler func(*C[In]) *Out, opts ...RouteOption)
```

### Group Typed Handlers

```go
func GroupGet[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func GroupPost[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func GroupPut[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func GroupDelete[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
func GroupPatch[In any, Out any](g *Group, path string, handler func(*C[In]) (*Out, error), opts ...RouteOption)
```

## RouteOption

```go
type RouteOption func(*routeConfig)
```

### WithDescription

```go
func WithDescription(desc string) RouteOption
```

Sets a route description (used by OpenAPI).

### WithTags

```go
func WithTags(tags ...string) RouteOption
```

Sets route tags (used by OpenAPI).

## Struct Tag Binding

The typed handler binds request data based on struct tags:

```go
type GetUserInput struct {
    ID     string `param:"id"`         // route parameter :id
    Fields string `query:"fields"`     // query string ?fields=...
}

type UpdateUserInput struct {
    ID   string `param:"id"`           // route parameter
    Name string `json:"name"`          // JSON body field
    Age  int    `json:"age"`           // JSON body field
}
```

Binding order:
1. `param` tags — from route parameters
2. `query` tags — from query string
3. `json` tags — from request body
4. `header` tags — from request headers

## Validation

After binding, validation runs automatically using `validate` struct tags:

```go
type Input struct {
    Name string `json:"name" validate:"required,min=2,max=100"`
    Age  int    `json:"age"  validate:"min=0,max=150"`
}
```

Validation failures return HTTP 422 with structured error details.
