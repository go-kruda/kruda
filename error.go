package kruda

import (
	"errors"
	"fmt"
)

// KrudaError is the standard error type for Kruda.
// It carries an HTTP status code and is auto-serialized as JSON.
type KrudaError struct {
	Code    int    `json:"code"`              // HTTP status code
	Message string `json:"message"`           // human-readable message
	Detail  string `json:"detail,omitempty"`  // optional detail
	Err     error  `json:"-"`                 // wrapped error (not exposed in JSON)
}

// Error implements the error interface.
func (e *KrudaError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error for errors.Is/As support.
func (e *KrudaError) Unwrap() error { return e.Err }

// NewError creates a new KrudaError with optional wrapped error.
func NewError(code int, message string, err ...error) *KrudaError {
	ke := &KrudaError{Code: code, Message: message}
	if len(err) > 0 {
		ke.Err = err[0]
	}
	return ke
}

// --- Convenience constructors ---

// BadRequest returns a 400 error.
func BadRequest(message string) *KrudaError {
	return &KrudaError{Code: 400, Message: message}
}

// Unauthorized returns a 401 error.
func Unauthorized(message string) *KrudaError {
	return &KrudaError{Code: 401, Message: message}
}

// Forbidden returns a 403 error.
func Forbidden(message string) *KrudaError {
	return &KrudaError{Code: 403, Message: message}
}

// NotFound returns a 404 error.
func NotFound(message string) *KrudaError {
	return &KrudaError{Code: 404, Message: message}
}

// Conflict returns a 409 error.
func Conflict(message string) *KrudaError {
	return &KrudaError{Code: 409, Message: message}
}

// UnprocessableEntity returns a 422 error.
func UnprocessableEntity(message string) *KrudaError {
	return &KrudaError{Code: 422, Message: message}
}

// TooManyRequests returns a 429 error.
func TooManyRequests(message string) *KrudaError {
	return &KrudaError{Code: 429, Message: message}
}

// InternalError returns a 500 error.
func InternalError(message string) *KrudaError {
	return &KrudaError{Code: 500, Message: message}
}

// --- Error Mapping ---

// ErrorMapping maps a Go error to an HTTP status code and message.
type ErrorMapping struct {
	Status  int
	Message string
}

// defaultErrorMap returns the default error mappings.
func defaultErrorMap() map[error]ErrorMapping {
	return map[error]ErrorMapping{}
}

// MapError registers an error-to-HTTP mapping.
// When a handler returns this error, Kruda auto-responds with the mapped status and message.
func (app *App) MapError(target error, status int, message string) *App {
	app.errorMap[target] = ErrorMapping{Status: status, Message: message}
	return app
}

// resolveError converts any error to a KrudaError using the error map.
func (app *App) resolveError(err error) *KrudaError {
	// 1. Already a KrudaError?
	var ke *KrudaError
	if errors.As(err, &ke) {
		return ke
	}

	// 2. Check error map
	for target, mapping := range app.errorMap {
		if errors.Is(err, target) {
			return &KrudaError{
				Code:    mapping.Status,
				Message: mapping.Message,
				Err:     err,
			}
		}
	}

	// 3. Default: 500 Internal Server Error
	return &KrudaError{
		Code:    500,
		Message: "internal server error",
		Err:     err,
	}
}
