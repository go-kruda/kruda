package ratelimit

import (
	"time"

	"github.com/go-kruda/kruda"
)

// Config holds rate limiter configuration.
type Config struct {
	// Max is the maximum number of requests allowed per Window. Required.
	Max int

	// Window is the time window for rate limiting. Required.
	Window time.Duration

	// Algorithm selects the rate limiting strategy.
	// "token_bucket" (default) or "sliding_window".
	Algorithm string

	// KeyFunc extracts the client identifier from the request.
	// Default: socket peer IP, or a forwarded IP from a trusted proxy.
	KeyFunc func(*kruda.Ctx) string

	// Skip returns true to bypass rate limiting for this request.
	// Default: nil (no skip).
	Skip func(*kruda.Ctx) bool

	// TrustedProxies is the list of trusted proxy IPs.
	// X-Forwarded-For is only used when the request comes from a trusted proxy.
	// Empty = do not trust forwarded headers (use the socket peer only).
	//
	// NOTE: Only exact IP addresses are supported (e.g., "10.0.0.1").
	// CIDR notation (e.g., "10.0.0.0/8") is NOT supported in v1.
	// If your proxy uses multiple IPs, list each one individually.
	TrustedProxies []string

	// CleanupInterval is how often expired entries are removed.
	// Default: 1 minute.
	CleanupInterval time.Duration

	// Store is the backing store for rate limit state.
	// Default: in-memory store.
	Store Store
}

func (c *Config) defaults() {
	if c.Algorithm == "" {
		c.Algorithm = "token_bucket"
	}
	if c.CleanupInterval <= 0 {
		c.CleanupInterval = time.Minute
	}
	if c.Window <= 0 {
		c.Window = time.Minute
	}
	if c.Max <= 0 {
		c.Max = 100
	}
}
