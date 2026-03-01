# Error Handling

Kruda provides structured error handling with error mapping, custom error types, and dev mode error pages.

## KrudaError

The built-in error type for HTTP errors:

```go
err := kruda.NewError(404, "User not found")
err := kruda.NewError(422, "Validation failed", wrappedErr)
```

Return errors from handlers:

```go
app.Get("/users/:id", func(c *kruda.Ctx) error {
    user, err := findUser(c.Param("id"))
    if err != nil {
        return kruda.NewError(404, "User not found")
    }
    return c.JSON(user)
})
```

Error response format:

```json
{
  "message": "User not found",
  "code": 404
}
```

## Error Mapping

Map domain errors to HTTP responses without cluttering handler logic:

### MapError — Map Specific Errors (method on App)

```go
var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

app.MapError(ErrNotFound, 404, "Resource not found")
app.MapError(ErrForbidden, 403, "Access denied")
```

Now handlers can return domain errors directly:

```go
app.Get("/users/:id", func(c *kruda.Ctx) error {
    user, err := repo.FindByID(c.Param("id"))
    if err != nil {
        return err // ErrNotFound → 404 automatically
    }
    return c.JSON(user)
})
```

### MapErrorType — Map Error Types (package-level generic function)

```go
kruda.MapErrorType[*ValidationError](app, 422, "Validation failed")
```

Note: This is a generic package-level function, not a method on App.

### MapErrorFunc — Custom Error Mapping (package-level function)

```go
kruda.MapErrorFunc(app, ErrDatabase, func(err error) *kruda.KrudaError {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        if pgErr.Code == "23505" { // unique violation
            return kruda.NewError(409, "Resource already exists")
        }
    }
    return nil // not handled, pass to next mapper
})
```

## Custom Error Handler

Override the default error handler:

```go
app := kruda.New(kruda.WithErrorHandler(func(c *kruda.Ctx, err *kruda.KrudaError) {
    c.Status(err.Code).JSON(map[string]any{
        "error":     err.Message,
        "requestId": c.Header("X-Request-ID"),
    })
}))
```

Note: `WithErrorHandler` receives `*KrudaError` (not `error`).

## Dev Mode Error Page

In development mode, errors render as a rich HTML page with source code context, stack trace, and request details:

```go
app := kruda.New(kruda.WithDevMode(true))
```

The dev error page includes:
- Error message and type
- Source code ±10 lines around the error
- Stack trace with file paths
- Request details (method, path, headers, query)
- Available routes table
- Filtered environment variables

Dev mode is never active in production. Enable it explicitly or via `KRUDA_ENV=development`.

See [Error API](/api/error) for the full reference.
