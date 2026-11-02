package ratelimit

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kruda/kruda"
)

// New creates a rate limiting middleware with the given config.
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()

	policy := newProxyPolicy(cfg.TrustedProxies)

	// Default key function: client IP
	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = func(c *kruda.Ctx) string {
			return extractIP(c, policy)
		}
	}

	// Create or use provided store
	store := cfg.Store
	if store == nil {
		store = NewMemoryStore(cfg.CleanupInterval, 2*cfg.Window)
	}

	// Select algorithm function
	var allowFn func(e *entry, limit int, window time.Duration) Result
	switch cfg.Algorithm {
	case "sliding_window":
		allowFn = slidingWindowAllow
	default:
		allowFn = tokenBucketAllow
	}

	limit := cfg.Max
	window := cfg.Window

	return func(c *kruda.Ctx) error {
		// Skip check
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		key := keyFunc(c)

		// Use internal path for MemoryStore to support algorithm selection
		var result Result
		if ms, ok := store.(*MemoryStore); ok {
			e := ms.getEntry(key)
			result = allowFn(e, limit, window)
		} else {
			result = store.Allow(key, limit, window)
		}

		// Set rate limit headers directly on the transport writer (R11.8).
		// Must use transport header map directly because Kruda's Text()/JSON()
		// fast paths bypass writeHeaders() for performance.
		h := c.ResponseWriter().Header()
		resetUnix := result.ResetAt.Unix()
		h.Set("X-RateLimit-Limit", strconv.Itoa(limit))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(resetUnix, 10))

		if !result.Allowed {
			retryAfterSec := int(result.RetryAt.Seconds())
			if retryAfterSec < 1 {
				retryAfterSec = 1
			}
			h.Set("Retry-After", strconv.Itoa(retryAfterSec))
			c.Status(http.StatusTooManyRequests)
			return c.JSON(kruda.Map{
				"error":       "rate limit exceeded",
				"retry_after": retryAfterSec,
			})
		}

		return c.Next()
	}
}

// extractIP uses the same trusted-hop policy as the framework.
func extractIP(c *kruda.Ctx, policy proxyPolicy) string {
	ip := policy.clientIP(requestWithPeer(c.Request()))
	if ip == "" {
		return "unknown"
	}
	return ip
}

// ForRoute creates a rate limiter scoped to specific routes with custom limits.
// Usage: app.Use(ratelimit.ForRoute("/api/login", 5, time.Minute))
func ForRoute(path string, max int, window time.Duration, opts ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(opts) > 0 {
		cfg = opts[0]
	}
	cfg.Max = max
	cfg.Window = window

	limiter := New(cfg)

	return func(c *kruda.Ctx) error {
		if c.Path() == path || strings.HasPrefix(c.Path(), path+"/") {
			return limiter(c)
		}
		return c.Next()
	}
}

// String returns a human-readable description of the rate limiter config.
func (c Config) String() string {
	return fmt.Sprintf("ratelimit(%d/%s, %s)", c.Max, c.Window, c.Algorithm)
}
