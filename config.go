package kruda

import (
	"bytes"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	krudajson "github.com/go-kruda/kruda/json"
	"github.com/go-kruda/kruda/transport"
)

// Config holds the server configuration.
type Config struct {
	ReadTimeout  time.Duration // default: 30s
	WriteTimeout time.Duration // default: 30s
	IdleTimeout  time.Duration // default: 120s

	BodyLimit   int // default: 4MB
	HeaderLimit int // default: 8KB

	ShutdownTimeout time.Duration // default: 10s

	Transport     transport.Transport
	TransportName string // "wing" (default on Linux/macOS), "fasthttp", "nethttp"
	TLSCertFile   string
	TLSKeyFile    string
	HTTP3         bool

	// TrustProxy enables trusting X-Forwarded-For/X-Real-IP headers. Default: false.
	TrustProxy bool

	JSONEncoder func(v any) ([]byte, error)
	JSONDecoder func(data []byte, v any) error

	// JSONStreamEncoder encodes v as JSON into the provided buffer.
	// When non-nil, c.JSON() uses this with a sync.Pool'd bytes.Buffer
	// instead of JSONEncoder, eliminating one allocation per response.
	// Set automatically when using the default encoder; cleared by WithJSONEncoder.
	JSONStreamEncoder func(buf *bytes.Buffer, v any) error

	Security        SecurityConfig
	SecurityHeaders bool
	PathTraversal   bool
	DevMode         bool
	devModeSet      bool

	Logger       *slog.Logger
	ErrorHandler func(c *Ctx, err *KrudaError)
	Validator    *Validator

	// Views is the template engine for c.Render(). Nil = c.Render() returns error.
	Views ViewEngine

	openAPIInfo openAPIInfo
	openAPIPath string
	openAPITags []openAPITagDef
}

// SecurityConfig controls security headers and behavior.
type SecurityConfig struct {
	// Security headers (all enabled by default)
	XSSProtection         string // default: "0" (disabled per modern best practice)
	ContentTypeNosniff    string // default: "nosniff"
	XFrameOptions         string // default: "DENY"
	HSTSMaxAge            int    // default: 0 (disabled), recommended: 31536000
	ContentSecurityPolicy string // default: ""
	ReferrerPolicy        string // default: "strict-origin-when-cross-origin"
}

// defaultConfig returns the default configuration.
func defaultConfig() Config {
	return Config{
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		BodyLimit:         4 * 1024 * 1024, // 4MB
		HeaderLimit:       8 * 1024,        // 8KB
		ShutdownTimeout:   10 * time.Second,
		JSONEncoder:       krudajson.Marshal,
		JSONDecoder:       krudajson.Unmarshal,
		JSONStreamEncoder: krudajson.MarshalToBuffer,
		Logger:            slog.Default(),
		SecurityHeaders:   false,
		Security: SecurityConfig{
			XSSProtection:      "0",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "DENY",
			ReferrerPolicy:     "strict-origin-when-cross-origin",
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

// WithMaxBodySize sets the maximum request body size in bytes.
// Alias for WithBodyLimit. When a request body exceeds this limit, the framework responds with HTTP 413.
func WithMaxBodySize(size int) Option {
	return WithBodyLimit(size)
}

// WithDevMode enables or disables development mode.
// When enabled, the framework relaxes X-Frame-Options to SAMEORIGIN
// and activates the dev error page (when implemented).
// Default: false (production mode). Also auto-detected via KRUDA_ENV=development.
func WithDevMode(enabled bool) Option {
	return func(a *App) {
		a.config.DevMode = enabled
		a.config.devModeSet = true
	}
}

// WithSecurity enables all security features: security headers and path traversal prevention.
// Equivalent to using both WithSecureHeaders() and WithPathTraversal().
// Recommended for production deployments.
func WithSecurity() Option {
	return func(a *App) {
		a.config.SecurityHeaders = true
		a.config.PathTraversal = true
	}
}

// WithSecureHeaders enables default security headers on all responses.
// Headers include X-Content-Type-Options, X-Frame-Options, X-XSS-Protection,
// and Referrer-Policy.
func WithSecureHeaders() Option {
	return func(a *App) { a.config.SecurityHeaders = true }
}

// WithPathTraversal enables path traversal prevention.
// Requests with path traversal patterns (../, encoded dots) are rejected.
func WithPathTraversal() Option {
	return func(a *App) { a.config.PathTraversal = true }
}

// WithLegacySecurityHeaders restores Phase 1-4 security header defaults
// for backward compatibility. Sets:
//   - X-XSS-Protection: "1; mode=block"
//   - X-Frame-Options: "SAMEORIGIN"
//   - Referrer-Policy: "no-referrer"
//   - X-Content-Type-Options: "nosniff" (unchanged)
func WithLegacySecurityHeaders() Option {
	return func(a *App) {
		a.config.SecurityHeaders = true
		a.config.Security = SecurityConfig{
			XSSProtection:      "1; mode=block",
			ContentTypeNosniff: "nosniff",
			XFrameOptions:      "SAMEORIGIN",
			ReferrerPolicy:     "no-referrer",
		}
	}
}

// WithTransport sets a custom transport.
func WithTransport(t transport.Transport) Option {
	return func(a *App) { a.config.Transport = t }
}

// FastHTTP selects the fasthttp transport.
// Use when you need fasthttp's battle-tested HTTP/1.1 handling.
func FastHTTP() Option {
	return func(a *App) { a.config.TransportName = "fasthttp" }
}

// NetHTTP selects the net/http transport for HTTP/2, TLS, and Windows compatibility.
// Use this when you need HTTP/2 auto-negotiation, TLS, or are running on Windows.
func NetHTTP() Option {
	return func(a *App) { a.config.TransportName = "nethttp" }
}

// Wing selects the Wing transport (epoll/kqueue).
// This is the default on Linux and macOS — calling Wing() is optional but explicit.
// Falls back to fasthttp on unsupported platforms.
func Wing() Option {
	return func(a *App) { a.config.TransportName = "wing" }
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
// This disables the default streaming buffer pool optimization.
// To re-enable pooling with a custom encoder, also call WithJSONStreamEncoder.
func WithJSONEncoder(enc func(v any) ([]byte, error)) Option {
	return func(a *App) {
		a.config.JSONEncoder = enc
		a.config.JSONStreamEncoder = nil // custom encoder — disable stream path
	}
}

// WithJSONStreamEncoder sets a streaming JSON encoder that writes into a
// provided bytes.Buffer. When set, c.JSON() uses a sync.Pool'd buffer with
// this encoder instead of JSONEncoder, avoiding one allocation per response.
func WithJSONStreamEncoder(enc func(buf *bytes.Buffer, v any) error) Option {
	return func(a *App) { a.config.JSONStreamEncoder = enc }
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

// WithTLS configures TLS for HTTPS and HTTP/2 auto-negotiation.
func WithTLS(certFile, keyFile string) Option {
	return func(a *App) {
		a.config.TLSCertFile = certFile
		a.config.TLSKeyFile = keyFile
	}
}

// WithHTTP3 enables HTTP/3 dual-stack serving (QUIC on UDP + HTTP/2 on TCP).
// Requires TLS certificate and key since QUIC mandates TLS 1.3.
// When enabled, the Alt-Svc header is auto-injected to advertise HTTP/3.
func WithHTTP3(certFile, keyFile string) Option {
	return func(a *App) {
		a.config.TLSCertFile = certFile
		a.config.TLSKeyFile = keyFile
		a.config.HTTP3 = true
	}
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
			cfg.BodyLimit = int(n)
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
// Decimal values are not supported ("1.5MB" → error, use "1536KB" instead).
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)
	if strings.HasSuffix(upper, "GB") {
		n, err := strconv.ParseInt(strings.TrimSpace(s[:len(s)-2]), 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024 * 1024, nil
	}
	if strings.HasSuffix(upper, "MB") {
		n, err := strconv.ParseInt(strings.TrimSpace(s[:len(s)-2]), 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024 * 1024, nil
	}
	if strings.HasSuffix(upper, "KB") {
		n, err := strconv.ParseInt(strings.TrimSpace(s[:len(s)-2]), 10, 64)
		if err != nil {
			return 0, err
		}
		return n * 1024, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err
}

// WithValidator sets a pre-configured Validator on the App.
func WithValidator(v *Validator) Option {
	return func(a *App) { a.config.Validator = v }
}

// WithContainer attaches a DI container to the App.
// The container is optional — apps without it work identically to Phase 1-3.
// When configured, the container is accessible to handlers via Resolve[T]()
// and is automatically shut down when the App shuts down.
func WithContainer(c *Container) Option {
	return func(a *App) { a.container = c }
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

// selectTransport chooses the transport based on config, env, and OS.
// Priority: explicit WithTransport() > Wing()/FastHTTP()/NetHTTP() > KRUDA_TRANSPORT env > default.
// Default is Wing on Linux/macOS for maximum performance. Falls back to net/http on Windows or when TLS is needed.
func selectTransport(cfg Config, logger *slog.Logger) (transport.Transport, string) {
	// If user provided a concrete Transport instance, use it directly
	if cfg.Transport != nil {
		return cfg.Transport, cfg.TransportName
	}

	name := cfg.TransportName
	if name == "" {
		if env := os.Getenv("KRUDA_TRANSPORT"); env != "" {
			name = env
		} else if runtime.GOOS == "windows" {
			name = "nethttp"
		} else {
			name = "wing"
		}
	}

	netHTTPCfg := transport.NetHTTPConfig{
		ReadTimeout:    cfg.ReadTimeout,
		WriteTimeout:   cfg.WriteTimeout,
		IdleTimeout:    cfg.IdleTimeout,
		MaxBodySize:    cfg.BodyLimit,
		MaxHeaderBytes: cfg.HeaderLimit,
		TrustProxy:     cfg.TrustProxy,
		TLSCertFile:    cfg.TLSCertFile,
		TLSKeyFile:     cfg.TLSKeyFile,
	}

	switch name {
	case "nethttp":
		logger.Debug("transport selected", "name", "nethttp")
		return transport.NewNetHTTP(netHTTPCfg), "nethttp"
	case "fasthttp":
		if cfg.TLSCertFile != "" {
			logger.Debug("transport selected", "name", "nethttp", "reason", "tls")
			return transport.NewNetHTTP(netHTTPCfg), "nethttp"
		}
		if runtime.GOOS == "windows" {
			logger.Debug("transport selected", "name", "nethttp", "reason", "windows")
			return transport.NewNetHTTP(netHTTPCfg), "nethttp"
		}
		return newFastHTTPTransport(cfg, logger), "fasthttp"
	default: // "wing" or any other value
		if cfg.TLSCertFile != "" {
			logger.Warn("Wing transport does not support TLS; falling back to net/http", "reason", "tls_override_wing")
			return transport.NewNetHTTP(netHTTPCfg), "nethttp"
		}
		return newWingTransport(cfg, logger), "wing"
	}
}
