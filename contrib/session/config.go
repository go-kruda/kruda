package session

import (
	"net/http"
	"time"
)

// Config holds configuration for the Session middleware.
type Config struct {
	// CookieName is the name of the session cookie.
	// Default: "_session"
	CookieName string

	// CookiePath sets the Path attribute of the session cookie.
	// Default: "/"
	CookiePath string

	// CookieDomain sets the Domain attribute of the session cookie.
	// Default: "" (current domain)
	CookieDomain string

	// CookieSecure sets the Secure flag on the session cookie.
	// Default: false
	CookieSecure bool

	// CookieHTTPOnly sets the HttpOnly flag on the session cookie.
	// Default: true (prevents JavaScript access).
	// Set DisableHTTPOnly to true to explicitly disable HttpOnly.
	CookieHTTPOnly bool

	// DisableHTTPOnly explicitly disables the HttpOnly flag on the session cookie.
	// This is needed because the zero value of CookieHTTPOnly (false) is
	// indistinguishable from "not set". Use this when JavaScript access is required.
	DisableHTTPOnly bool

	// CookieSameSite sets the SameSite attribute of the session cookie.
	// Default: http.SameSiteLaxMode
	CookieSameSite http.SameSite

	// MaxAge is the cookie max-age in seconds.
	// Default: 86400 (24 hours)
	MaxAge int

	// IdleTimeout is the maximum time a session can be idle before expiring.
	// The session's expiration is refreshed on each request.
	// Default: 30 minutes
	IdleTimeout time.Duration

	// Store is the backing session storage.
	// Default: NewMemoryStore() (in-memory, single instance only)
	Store Store

	// Skip is an optional function to skip session management for certain requests.
	// Return true to skip creating/loading a session.
	Skip func(method, path string) bool
}

// defaults applies default values to a Config.
func (c *Config) defaults() {
	if c.CookieName == "" {
		c.CookieName = "_session"
	}
	if c.CookiePath == "" {
		c.CookiePath = "/"
	}
	if c.CookieSameSite == 0 {
		c.CookieSameSite = http.SameSiteLaxMode
	}
	if c.MaxAge == 0 {
		c.MaxAge = 86400 // 24 hours
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 30 * time.Minute
	}
	// HTTPOnly defaults to true for security.
	if !c.DisableHTTPOnly {
		c.CookieHTTPOnly = true
	}
}
