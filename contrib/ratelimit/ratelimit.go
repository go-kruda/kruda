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

	// Build trusted proxy set for O(1) lookup
	trustedSet := make(map[string]struct{}, len(cfg.TrustedProxies))
	for _, ip := range cfg.TrustedProxies {
		trustedSet[ip] = struct{}{}
	}

	// Default key function: client IP
	keyFunc := cfg.KeyFunc
	if keyFunc == nil {
		keyFunc = func(c *kruda.Ctx) string {
			return extractIP(c, trustedSet)
		}
	}

	// Create or use provided store
	store := cfg.Store
	if store == nil {
		store = NewMemoryStore(cfg.CleanupInterval)
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

// extractIP extracts the client IP, respecting trusted proxies.
func extractIP(c *kruda.Ctx, trusted map[string]struct{}) string {
	remoteIP := c.IP()

	// Only trust forwarded headers from trusted proxies (R11.14)
	if len(trusted) > 0 {
		if _, ok := trusted[remoteIP]; ok {
			// X-Forwarded-For: client, proxy1, proxy2
			if xff := c.Header("X-Forwarded-For"); xff != "" {
				if ip := firstIP(xff); ip != "" {
					return ip
				}
			}
			if xri := c.Header("X-Real-IP"); xri != "" {
				return strings.TrimSpace(xri)
			}
		}
	}

	return remoteIP
}

// firstIP extracts the first IP from a comma-separated X-Forwarded-For value.
func firstIP(xff string) string {
	if idx := strings.IndexByte(xff, ','); idx >= 0 {
		return strings.TrimSpace(xff[:idx])
	}
	return strings.TrimSpace(xff)
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
