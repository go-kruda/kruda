package middleware

import (
	"log/slog"
	"runtime"

	"github.com/go-kruda/kruda"
)

// RecoveryConfig holds configuration for the Recovery middleware.
type RecoveryConfig struct {
	// Logger is the slog.Logger for logging panics.
	// Default: slog.Default()
	Logger *slog.Logger

	// PanicHandler is an optional custom handler called when a panic is recovered.
	// If set, it replaces the default behavior (log + 500 response).
	PanicHandler func(c *kruda.Ctx, v any)

	// DisableStackTrace skips capturing and logging stack traces on panic.
	// Enable in production to avoid leaking internal paths in logs.
	// Default: false
	DisableStackTrace bool
}

// Recovery returns middleware that recovers from panics in handlers,
// logs the panic value and stack trace, and returns a 500 Internal Server Error.
// Returns an InternalError so that OnError hooks fire properly.
// It accepts an optional RecoveryConfig for customization.
func Recovery(config ...RecoveryConfig) kruda.HandlerFunc {
	cfg := RecoveryConfig{
		Logger: slog.Default(),
	}
	if len(config) > 0 {
		if config[0].Logger != nil {
			cfg.Logger = config[0].Logger
		}
		if config[0].PanicHandler != nil {
			cfg.PanicHandler = config[0].PanicHandler
		}
		cfg.DisableStackTrace = config[0].DisableStackTrace
	}

	return func(c *kruda.Ctx) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				var logArgs []any
				logArgs = append(logArgs, "panic", r)

				if !cfg.DisableStackTrace {
					buf := make([]byte, 4096)
					n := runtime.Stack(buf, false)
					logArgs = append(logArgs, "stack", string(buf[:n]))
				}

				if cfg.PanicHandler != nil {
					cfg.Logger.Error("panic recovered (custom handler)", logArgs...)
					cfg.PanicHandler(c, r)
					if c.StatusCode() == 200 && !c.Responded() {
						retErr = kruda.InternalError("panic recovered")
					}
					return
				}

				cfg.Logger.Error("panic recovered", logArgs...)

				// Return error so OnError hooks fire; the error handler writes the response.
				retErr = kruda.InternalError("internal server error")
			}
		}()

		return c.Next()
	}
}
