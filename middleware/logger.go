package middleware

import (
	"errors"
	"log/slog"

	"github.com/go-kruda/kruda"
)

// LoggerConfig holds configuration for the Logger middleware.
type LoggerConfig struct {
	// Logger is the slog.Logger to use for logging.
	// Default: slog.Default()
	Logger *slog.Logger

	// SkipPaths is a list of paths to skip logging (e.g. "/health", "/metrics").
	SkipPaths []string
}

// Logger returns middleware that logs request information using slog.
// It logs method, path, status code, latency, and client IP.
// Log level is determined by status code: 5xx=Error, 4xx=Warn, 2xx/3xx=Info.
func Logger(config ...LoggerConfig) kruda.HandlerFunc {
	cfg := LoggerConfig{
		Logger: slog.Default(),
	}
	if len(config) > 0 {
		if config[0].Logger != nil {
			cfg.Logger = config[0].Logger
		}
		cfg.SkipPaths = config[0].SkipPaths
	}

	// Build skip map for O(1) lookup.
	skip := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = true
	}

	return func(c *kruda.Ctx) error {
		// Skip logging for configured paths.
		if skip[c.Path()] {
			return c.Next()
		}

		// Execute next handler.
		err := c.Next()

		// N1 fix: resolve status from error if handler returned one,
		// since handleError hasn't set the status on Ctx yet when Logger reads it.
		status := c.StatusCode()
		if err != nil {
			var ke *kruda.KrudaError
			if errors.As(err, &ke) {
				status = ke.Code
			} else {
				status = 500
			}
		}
		attrs := []any{
			"method", c.Method(),
			"path", c.Path(),
			"status", status,
			"latency", c.Latency().String(),
			"ip", c.IP(),
		}

		switch {
		case status >= 500:
			cfg.Logger.Error("request", attrs...)
		case status >= 400:
			cfg.Logger.Warn("request", attrs...)
		default:
			cfg.Logger.Info("request", attrs...)
		}

		return err
	}
}
