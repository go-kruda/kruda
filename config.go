package kruda

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	krudajson "github.com/go-kruda/kruda/json"
	"github.com/go-kruda/kruda/transport"
)

// Config holds the server configuration.
type Config struct {
	// Server timeouts
	ReadTimeout  time.Duration // default: 30s
	WriteTimeout time.Duration // default: 30s
	IdleTimeout  time.Duration // default: 120s

	// Limits
	BodyLimit   int // default: 4MB (4 * 1024 * 1024)
	HeaderLimit int // default: 8KB (8 * 1024)

	// Shutdown
	ShutdownTimeout time.Duration // default: 10s

	// Transport
	Transport transport.Transport // default: net/http (Phase 1), Netpoll (Phase 3)

	// Proxy trust (C6 fix: default false — only trust X-Forwarded-For/X-Real-IP when true)
	TrustProxy bool

	// JSON engine
	JSONEncoder func(v any) ([]byte, error)    // default: encoding/json
	JSONDecoder func(data []byte, v any) error // default: encoding/json

	// Security (all enabled by default)
	Security SecurityConfig

	// Logging
	Logger *slog.Logger // default: slog.Default()

	// Error handler — receives *KrudaError with HTTP status code and message.
	// Use ke.Unwrap() to access the original error if needed.
	ErrorHandler func(c *Ctx, err *KrudaError)

	// Validator holds the validation engine. nil = no validation (zero overhead).
	// Set via WithValidator() option or lazy-created via app.Validator().
	Validator *Validator

	// OpenAPI configuration (zero value = disabled, zero overhead)
	openAPIInfo openAPIInfo
	openAPIPath string
	openAPITags []openAPITagDef
}

// SecurityConfig controls security headers and behavior.
type SecurityConfig struct {
	// Security headers (all enabled by default)
	XSSProtection         string // default: "1; mode=block"
	ContentTypeNosniff    string // default: "nosniff"
	XFrameOptions         string // default: "SAMEORIGIN"
	HSTSMaxAge            int    // default: 0 (disabled), recommended: 31536000
	ContentSecurityPolicy string // default: ""
	ReferrerPolicy        string // default: "no-referrer"
}

// defaultConfig returns the default configuration.
func defaultConfig() Config {
	return Config{
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     120 * time.Second,
		BodyLimit:       4 * 1024 * 1024, // 4MB
		HeaderLimit:     8 * 1024,        // 8KB
		ShutdownTimeout: 10 * time.Second,
		JSONEncoder:     krudajson.Marshal,
		JSONDecoder:     krudajson.Unmarshal,
		Logger:          slog.Default(),
		Security: SecurityConfig{
			XSSProtection:      "1; mode=block",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "SAMEORIGIN",
			ReferrerPolicy:     "no-referrer",
		},
	}
}

// Option is a functional option for configuring the App.
type Option func(*App)

// WithReadTimeout sets the read timeout.
func WithReadTimeout(d time.Duration) Option {
	return func(a *App) { a.config.ReadTimeout = d }
}

// WithWriteTimeout sets the write timeout.
func WithWriteTimeout(d time.Duration) Option {
	return func(a *App) { a.config.WriteTimeout = d }
}

// WithIdleTimeout sets the idle timeout.
func WithIdleTimeout(d time.Duration) Option {
	return func(a *App) { a.config.IdleTimeout = d }
}

// WithBodyLimit sets the maximum request body size.
func WithBodyLimit(limit int) Option {
	return func(a *App) { a.config.BodyLimit = limit }
}

// WithTransport sets a custom transport.
func WithTransport(t transport.Transport) Option {
	return func(a *App) { a.config.Transport = t }
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) Option {
	return func(a *App) { a.config.Logger = l }
}

// WithErrorHandler sets a custom error handler.
func WithErrorHandler(h func(c *Ctx, err *KrudaError)) Option {
	return func(a *App) { a.config.ErrorHandler = h }
}

// WithJSONEncoder sets a custom JSON encoder.
func WithJSONEncoder(enc func(v any) ([]byte, error)) Option {
	return func(a *App) { a.config.JSONEncoder = enc }
}

// WithJSONDecoder sets a custom JSON decoder.
func WithJSONDecoder(dec func(data []byte, v any) error) Option {
	return func(a *App) { a.config.JSONDecoder = dec }
}

// WithShutdownTimeout sets the graceful shutdown timeout.
func WithShutdownTimeout(d time.Duration) Option {
	return func(a *App) { a.config.ShutdownTimeout = d }
}

// WithTrustProxy enables trusting proxy headers (X-Forwarded-For, X-Real-IP).
// Default is false — only the direct connection's remote address is used.
func WithTrustProxy(trust bool) Option {
	return func(a *App) { a.config.TrustProxy = trust }
}

// WithEnvPrefix reads config from environment variables with the given prefix.
// Maps CamelCase field names to SCREAMING_SNAKE_CASE env vars.
// Example: prefix="APP" reads APP_READ_TIMEOUT, APP_BODY_LIMIT, etc.
// Missing or unparseable env vars are silently ignored (defaults are kept).
func WithEnvPrefix(prefix string) Option {
	return func(a *App) {
		applyEnvConfig(prefix, &a.config)
	}
}

// applyEnvConfig reads environment variables with the given prefix and overrides
// the corresponding Config fields. If an env var is missing or cannot be parsed,
// the existing value is kept.
func applyEnvConfig(prefix string, cfg *Config) {
	if v := os.Getenv(prefix + "_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadTimeout = d
		}
	}
	if v := os.Getenv(prefix + "_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.WriteTimeout = d
		}
	}
	if v := os.Getenv(prefix + "_IDLE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.IdleTimeout = d
		}
	}
	if v := os.Getenv(prefix + "_BODY_LIMIT"); v != "" {
		if n, err := parseSize(v); err == nil {
			cfg.BodyLimit = n
		}
	}
	if v := os.Getenv(prefix + "_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ShutdownTimeout = d
		}
	}
}

// parseSize parses a human-readable size string into bytes.
// Supported suffixes: KB, MB, GB (case insensitive).
// Plain number strings are treated as bytes.
//
// F5 limitations (by design, sufficient for Phase 1):
//   - Decimal values not supported ("1.5MB" → error). Use "1536KB" instead.
//   - "B" suffix not recognized. Plain numbers are already treated as bytes.
func parseSize(s string) (int, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)
	if strings.HasSuffix(upper, "GB") {
		n, err := strconv.Atoi(strings.TrimSpace(s[:len(s)-2]))
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024 * 1024, nil
	}
	if strings.HasSuffix(upper, "MB") {
		n, err := strconv.Atoi(strings.TrimSpace(s[:len(s)-2]))
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024, nil
	}
	if strings.HasSuffix(upper, "KB") {
		n, err := strconv.Atoi(strings.TrimSpace(s[:len(s)-2]))
		if err != nil {
			return 0, err
		}
		return n * 1024, nil
	}
	return strconv.Atoi(s)
}

// WithValidator sets a pre-configured Validator on the App.
func WithValidator(v *Validator) Option {
	return func(a *App) { a.config.Validator = v }
}

// WithOpenAPIInfo enables OpenAPI spec generation with the given metadata.
// When configured, a GET handler is auto-registered at the OpenAPI path (default: /openapi.json).
func WithOpenAPIInfo(title, version, description string) Option {
	return func(a *App) {
		a.config.openAPIInfo = openAPIInfo{
			Title:       title,
			Version:     version,
			Description: description,
		}
		if a.config.openAPIPath == "" {
			a.config.openAPIPath = "/openapi.json"
		}
	}
}

// WithOpenAPIPath sets the path where the OpenAPI spec is served.
func WithOpenAPIPath(path string) Option {
	return func(a *App) { a.config.openAPIPath = path }
}

// WithOpenAPITag adds a tag definition to the OpenAPI spec.
func WithOpenAPITag(name, description string) Option {
	return func(a *App) {
		a.config.openAPITags = append(a.config.openAPITags, openAPITagDef{
			Name: name, Description: description,
		})
	}
}
