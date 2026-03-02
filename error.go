package kruda

import (
	"errors"
	"fmt"
	"reflect"
)

// KrudaError is the standard error type for Kruda.
// It carries an HTTP status code and is auto-serialized as JSON.
type KrudaError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
	Err     error  `json:"-"`
	mapped  bool   // true when error was resolved via MapError/MapErrorFunc/MapErrorType
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

// errorTypeMapping maps an error type to HTTP status + message.
type errorTypeMapping struct {
	errType    reflect.Type
	statusCode int
	message    string
}

// errorFuncMapping maps an error to a custom transformation function.
type errorFuncMapping struct {
	target error
	fn     func(error) *KrudaError
}

// MapErrorType registers a type-based error mapping.
// Matches any error of type T using errors.As.
//
// This is a free function (not a method on *App) because Go does not
// support generic methods. Use: kruda.MapErrorType[*MyError](app, 422, "msg")
func MapErrorType[T error](app *App, statusCode int, message string) {
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t == reflect.TypeOf((*error)(nil)).Elem() {
		panic("kruda: MapErrorType[error] would match all errors — use a specific type")
	}
	app.errorTypes = append(app.errorTypes, errorTypeMapping{
		errType:    t,
		statusCode: statusCode,
		message:    message,
	})
}

// MapErrorFunc registers a custom error transformation function.
// When a handler returns an error matching target (via errors.Is),
// fn is called to produce the KrudaError response.
func MapErrorFunc(app *App, target error, fn func(error) *KrudaError) {
	app.errorFuncs = append(app.errorFuncs, errorFuncMapping{
		target: target,
		fn:     fn,
	})
}

// resolveError converts any error to a KrudaError using the error map.
func (app *App) resolveError(err error) *KrudaError {
	var ke *KrudaError
	if errors.As(err, &ke) {
		return ke
	}

	for target, mapping := range app.errorMap {
		if errors.Is(err, target) {
			return &KrudaError{
				Code:    mapping.Status,
				Message: mapping.Message,
				Detail:  err.Error(),
				Err:     err,
				mapped:  true,
			}
		}
	}

	for _, ef := range app.errorFuncs {
		if errors.Is(err, ef.target) {
			ke := ef.fn(err)
			ke.mapped = true
			return ke
		}
	}

	// reflect.New per type is acceptable here — only runs on error paths.
	// Typical apps have <10 type mappings.
	for _, et := range app.errorTypes {
		target := reflect.New(et.errType).Interface()
		if errors.As(err, target) {
			return &KrudaError{
				Code:    et.statusCode,
				Message: et.message,
				Detail:  err.Error(),
				Err:     err,
				mapped:  true,
			}
		}
	}

	return &KrudaError{
		Code:    500,
		Message: "internal server error",
		Err:     err,
	}
}
