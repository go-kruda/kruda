package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/go-kruda/kruda"
)

// CSRFConfig holds configuration for the CSRF middleware.
type CSRFConfig struct {
	// CookieName is the name of the cookie that stores the CSRF token.
	// Default: "_csrf"
	CookieName string

	// HeaderName is the HTTP header to check for the CSRF token.
	// Default: "X-CSRF-Token"
	HeaderName string

	// CookiePath sets the Path attribute of the CSRF cookie.
	// Default: "/"
	CookiePath string

	// CookieDomain sets the Domain attribute of the CSRF cookie.
	// Default: "" (current domain)
	CookieDomain string

	// CookieSecure sets the Secure flag on the CSRF cookie.
	// Default: false
	CookieSecure bool

	// SameSite sets the SameSite attribute of the CSRF cookie.
	// Default: http.SameSiteStrictMode
	SameSite http.SameSite

	// MaxAge is the cookie max-age in seconds.
	// Default: 3600 (1 hour)
	MaxAge int

	// TokenLength is the number of random bytes in the token.
	// The cookie value will be hex-encoded (2× this length).
	// Default: 32 (64 hex characters)
	TokenLength int

	// Skip is an optional function to skip CSRF protection for certain requests.
	// Return true to skip validation entirely.
	Skip func(*kruda.Ctx) bool

	// ErrorHandler is an optional custom error handler for CSRF failures.
	// Default: 403 JSON response {"error": "csrf_token_invalid"}
	ErrorHandler func(*kruda.Ctx) error
}

// csrfTokenKey is the context key used to expose the CSRF token to handlers.
const csrfTokenKey = "csrf_token"

// CSRF returns middleware that provides Cross-Site Request Forgery protection
// using the double-submit cookie pattern.
//
// For safe methods (GET, HEAD, OPTIONS, TRACE), it generates a new token,
// sets it as a cookie, and stores it in the request context via c.Set("csrf_token", token).
//
// For unsafe methods (POST, PUT, DELETE, PATCH), it validates the token from
// the X-CSRF-Token header (or custom header) against the cookie value using
// constant-time comparison.
//
// Usage:
//
//	app.Use(middleware.CSRF())
//
//	// In handler — get token for template rendering:
//	token := c.Get("csrf_token").(string)
func CSRF(config ...CSRFConfig) kruda.HandlerFunc {
	cfg := CSRFConfig{
		CookieName:  "_csrf",
		HeaderName:  "X-CSRF-Token",
		CookiePath:  "/",
		SameSite:    http.SameSiteStrictMode,
		MaxAge:      3600,
		TokenLength: 32,
	}
	if len(config) > 0 {
		c := config[0]
		if c.CookieName != "" {
			cfg.CookieName = c.CookieName
		}
		if c.HeaderName != "" {
			cfg.HeaderName = c.HeaderName
		}
		if c.CookiePath != "" {
			cfg.CookiePath = c.CookiePath
		}
		if c.CookieDomain != "" {
			cfg.CookieDomain = c.CookieDomain
		}
		cfg.CookieSecure = c.CookieSecure
		if c.SameSite != 0 {
			cfg.SameSite = c.SameSite
		}
		if c.MaxAge > 0 {
			cfg.MaxAge = c.MaxAge
		}
		if c.TokenLength > 0 {
			cfg.TokenLength = c.TokenLength
		}
		cfg.Skip = c.Skip
		cfg.ErrorHandler = c.ErrorHandler
	}

	// Validate config at init time.
	if cfg.TokenLength < 16 {
		panic("kruda: CSRF TokenLength must be at least 16 bytes")
	}

	return func(c *kruda.Ctx) error {
		// Skip if custom skip function returns true.
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		method := c.Method()

		// Safe methods: generate/refresh token, no validation required.
		if isSafeMethod(method) {
			token := generateCSRFToken(cfg.TokenLength)
			setCSRFCookie(c, &cfg, token)
			c.Set(csrfTokenKey, token)
			// Vary by Cookie so CDNs don't cache responses with different CSRF tokens.
			c.AddHeader("Vary", "Cookie")
			return c.Next()
		}

		// Unsafe methods: validate token.
		cookieToken := c.Cookie(cfg.CookieName)
		if cookieToken == "" {
			return csrfError(c, &cfg)
		}

		// Check header first (SPA/AJAX pattern).
		headerToken := c.Header(cfg.HeaderName)
		if headerToken == "" {
			return csrfError(c, &cfg)
		}

		// Constant-time comparison to prevent timing attacks.
		if subtle.ConstantTimeCompare([]byte(cookieToken), []byte(headerToken)) != 1 {
			return csrfError(c, &cfg)
		}

		// Token valid — refresh with new token for next request.
		newToken := generateCSRFToken(cfg.TokenLength)
		setCSRFCookie(c, &cfg, newToken)
		c.Set(csrfTokenKey, newToken)
		c.AddHeader("Vary", "Cookie")

		return c.Next()
	}
}

// isSafeMethod returns true for HTTP methods that don't modify server state.
func isSafeMethod(method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
}

// setCSRFCookie sets the CSRF cookie on the response.
// HttpOnly is false because JavaScript needs to read the cookie to send the token
// in a custom header (double-submit cookie pattern).
func setCSRFCookie(c *kruda.Ctx, cfg *CSRFConfig, token string) {
	c.SetCookie(&kruda.Cookie{
		Name:     cfg.CookieName,
		Value:    token,
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   cfg.MaxAge,
		Secure:   cfg.CookieSecure,
		HTTPOnly: false, // JS must read this for double-submit
		SameSite: cfg.SameSite,
	})
}

// csrfError returns the CSRF error response.
func csrfError(c *kruda.Ctx, cfg *CSRFConfig) error {
	if cfg.ErrorHandler != nil {
		return cfg.ErrorHandler(c)
	}
	return c.Status(403).JSON(map[string]string{"error": "csrf_token_invalid"})
}

// generateCSRFToken generates a cryptographically random token.
// Panics if crypto/rand fails — an empty token would defeat CSRF protection.
func generateCSRFToken(length int) string {
	b := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("kruda: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
