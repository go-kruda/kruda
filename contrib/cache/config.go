package cache

import (
	"time"

	"github.com/go-kruda/kruda"
)

// Config holds configuration for the cache middleware.
type Config struct {
	// TTL is the time-to-live for cached responses.
	// Default: 5 minutes
	TTL time.Duration

	// KeyFunc generates the cache key from the request context.
	// Default: method + ":" + path
	KeyFunc func(c *kruda.Ctx) string

	// Methods lists HTTP methods eligible for caching.
	// Default: ["GET"]
	Methods []string

	// StatusCodes lists response status codes eligible for caching.
	// Default: [200]
	StatusCodes []int

	// Store is the backing cache storage.
	// Default: NewMemoryStore(1000)
	Store Store

	// Skip is an optional function to skip caching for certain requests.
	// Return true to bypass the cache entirely (no lookup, no store).
	Skip func(c *kruda.Ctx) bool
}

// defaults applies default values to a Config.
func (c *Config) defaults() {
	if c.TTL == 0 {
		c.TTL = 5 * time.Minute
	}
	if c.KeyFunc == nil {
		c.KeyFunc = defaultKeyFunc
	}
	if c.Methods == nil {
		c.Methods = []string{"GET"}
	}
	if c.StatusCodes == nil {
		c.StatusCodes = []int{200}
	}
}

// defaultKeyFunc generates a cache key from method and path.
func defaultKeyFunc(c *kruda.Ctx) string {
	return c.Method() + ":" + c.Path()
}
