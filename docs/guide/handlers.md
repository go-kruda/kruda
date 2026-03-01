# Handlers

## Basic Handlers

A handler receives a `*Ctx` and returns an error:

```go
app.Get("/hello", func(c *kruda.Ctx) error {
    return c.Text("Hello, World!")
})
```

The `HandlerFunc` type:

```go
type HandlerFunc func(c *Ctx) error
```

## Typed Handlers with C[T]

Typed handlers use Go generics to auto-parse request data into a struct:

```go
type CreateUserInput struct {
    Name  string `json:"name"  validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}

type User struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

kruda.Post[CreateUserInput, User](app, "/users", func(c *kruda.C[CreateUserInput]) (*User, error) {
    return &User{ID: "1", Name: c.In.Name, Email: c.In.Email}, nil
})
```

The `C[T]` type extends `Ctx` with a typed `In` field:

```go
type C[T any] struct {
    *Ctx
    In T // parsed input from request
}
```

## Typed Handler Registration

Package-level generic functions register typed handlers:

```go
kruda.Get[In, Out](app, path, handler, opts...)
kruda.Post[In, Out](app, path, handler, opts...)
kruda.Put[In, Out](app, path, handler, opts...)
kruda.Delete[In, Out](app, path, handler, opts...)
kruda.Patch[In, Out](app, path, handler, opts...)
```

Short variants (no error return, for prototyping):

```go
kruda.GetX[In, Out](app, path, handler, opts...)
kruda.PostX[In, Out](app, path, handler, opts...)
```

Group-level typed handlers:

```go
kruda.GroupGet[In, Out](g, path, handler, opts...)
kruda.GroupPost[In, Out](g, path, handler, opts...)
```

## Struct Tags

Control how request data is bound to your struct:

```go
type GetUserInput struct {
    ID     string `param:"id"`              // from route parameter :id
    Fields string `query:"fields"`          // from query string ?fields=name,email
}

type UpdateUserInput struct {
    ID   string `param:"id"`               // from route parameter
    Name string `json:"name"`              // from JSON body
    Age  int    `json:"age"`               // from JSON body
}
```

Supported tag types:

| Tag | Source | Example |
|-----|--------|---------|
| `param` | Route parameters (`:id`) | `param:"id"` |
| `query` | Query string (`?key=val`) | `query:"fields"` |
| `json` | JSON request body | `json:"name"` |
| `header` | Request headers | `header:"X-Request-ID"` |

## Validation

Validation runs automatically after binding. Use `validate` struct tags:

```go
type Input struct {
    Name  string `json:"name"  validate:"required,min=2,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"min=0,max=150"`
}
```

When validation fails, Kruda returns a structured error response:

```json
{
  "error": "Validation failed",
  "code": 422,
  "details": [
    { "field": "email", "message": "must be a valid email" }
  ]
}
```

## Response Methods

Handlers use `Ctx` methods to send responses:

```go
// JSON response (status set via Status())
c.Status(200).JSON(user)
c.JSON(user) // default status 200

// Text response
c.Status(200).Text("OK")
c.Text("OK")

// HTML response
c.HTML("<h1>Hello</h1>")

// No content (204)
c.NoContent()

// With headers (method chaining)
c.Status(200).SetHeader("X-Custom", "value").JSON(data)

// Redirect (default 302)
c.Redirect("/new-location")
c.Redirect("/new-location", 301)
```

See [Context API](/api/context) for all response methods.

## Route Options

Add metadata for OpenAPI generation:

```go
kruda.Post[CreateUserInput, User](app, "/users", handler,
    kruda.WithDescription("Create a new user"),
    kruda.WithTags("users"),
)
```
