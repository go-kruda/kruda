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
}

// Recovery returns middleware that recovers from panics in handlers,
// logs the panic value and stack trace, and returns a 500 Internal Server Error.
// M5 fix: returns an InternalError so that OnError hooks fire properly.
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
	}

	return func(c *kruda.Ctx) (retErr error) {
		defer func() {
			if r := recover(); r != nil {
				// Capture stack trace.
				buf := make([]byte, 4096)
				n := runtime.Stack(buf, false)
				stack := string(buf[:n])

				if cfg.PanicHandler != nil {
					// F2 fix: always log stack trace before calling custom handler,
					// so panic context is never silently lost.
					cfg.Logger.Error("panic recovered (custom handler)",
						"panic", r,
						"stack", stack,
					)
					cfg.PanicHandler(c, r)
					// NEW-1 fix: only propagate error if custom handler didn't
					// write a response, to avoid double-write risk.
					// If it did respond, OnError hooks won't fire — this is
					// documented: custom PanicHandler takes full ownership.
					if c.StatusCode() == 200 && !c.Responded() {
						retErr = kruda.InternalError("panic recovered")
					}
					return
				}

				cfg.Logger.Error("panic recovered",
					"panic", r,
					"stack", stack,
				)

				// M5: return error instead of writing response directly,
				// so OnError hooks can fire. The error handler will write the response.
				retErr = kruda.InternalError("internal server error")
			}
		}()

		return c.Next()
	}
}
