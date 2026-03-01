# Error

Structured error handling with error mapping and custom error types.

## KrudaError

```go
type KrudaError struct {
    Code    int    `json:"code"`             // HTTP status code
    Message string `json:"message"`          // human-readable message
    Detail  string `json:"detail,omitempty"` // optional detail
    Err     error  `json:"-"`                // wrapped error (not exposed in JSON)
}
```

### NewError

```go
func NewError(code int, message string, err ...error) *KrudaError
```

Creates a new HTTP error. Optionally wraps an underlying error.

```go
return kruda.NewError(404, "User not found")
return kruda.NewError(400, "Invalid request", err)
```

### Error Response Format

```json
{
  "code": 404,
  "message": "User not found"
}
```

With detail:

```json
{
  "code": 422,
  "message": "Validation failed",
  "detail": "field 'email' is required"
}
```

## Error Mapping

### MapError (method on App)

```go
func (app *App) MapError(target error, status int, message string) *App
```

Maps a specific error value to an HTTP response.

```go
var ErrNotFound = errors.New("not found")
app.MapError(ErrNotFound, 404, "Resource not found")
```

### MapErrorType (package-level generic function)

```go
func MapErrorType[T error](app *App, statusCode int, message string)
```

Maps all errors of a given type.

```go
kruda.MapErrorType[*ValidationError](app, 422, "Validation failed")
```

### MapErrorFunc (package-level function)

```go
func MapErrorFunc(app *App, target error, fn func(error) *KrudaError)
```

Registers a custom mapping function for a specific target error. Return `nil` to pass to the next mapper.

```go
kruda.MapErrorFunc(app, ErrDB, func(err error) *kruda.KrudaError {
    if errors.Is(err, sql.ErrNoRows) {
        return kruda.NewError(404, "Not found")
    }
    return nil
})
```

## Error Flow

When a handler returns an error:

1. Check if it's already a `*KrudaError` → use directly
2. Check `MapError` exact matches → use mapped response
3. Check `MapErrorType` type matches → use mapped response
4. Check `MapErrorFunc` functions in order → use first non-nil result
5. Fall back to HTTP 500 Internal Server Error

In dev mode (`WithDevMode(true)`), unhandled errors render a rich HTML error page instead of JSON.
