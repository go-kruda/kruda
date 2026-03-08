// Package cache provides response caching middleware for Kruda.
//
// The middleware checks for cached responses on incoming requests and serves
// them directly on cache hits. On cache misses, it stores caching metadata
// in the request context. Handlers use the CacheJSON or CacheBytes helper
// functions to send the response and store it in the cache simultaneously.
//
// Usage:
//
//	app := kruda.New()
//	app.Use(cache.New())
//
//	app.Get("/api/data", func(c *kruda.Ctx) error {
//	    data := fetchExpensiveData()
//	    return cache.CacheJSON(c, data)
//	})
//
// With custom config:
//
//	app.Use(cache.New(cache.Config{
//	    TTL:     10 * time.Minute,
//	    Methods: []string{"GET"},
//	    Store:   cache.NewMemoryStore(5000),
//	}))
package cache

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/go-kruda/kruda"
)

// Context keys for cache metadata stored in request locals.
const (
	cacheKeyKey   = "_cache_key"
	cacheStoreKey = "_cache_store"
	cacheTTLKey   = "_cache_ttl"
	cacheCodesKey = "_cache_codes"
)

// New creates cache middleware with the given configuration.
//
// On cache HIT: sets X-Cache: HIT and Age headers, replays the cached response,
// and does NOT call c.Next().
//
// On cache MISS: sets X-Cache: MISS, stores caching metadata in the request
// context, and calls c.Next(). Handlers should use CacheJSON or CacheBytes
// to send and cache their response.
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg.defaults()

	// Use default memory store if none provided.
	if cfg.Store == nil {
		cfg.Store = NewMemoryStore(1000)
	}

	// Pre-build method set for O(1) lookup.
	methodSet := make(map[string]struct{}, len(cfg.Methods))
	for _, m := range cfg.Methods {
		methodSet[m] = struct{}{}
	}

	return func(c *kruda.Ctx) error {
		// Skip if custom skip function returns true.
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		// Skip non-cacheable methods.
		if _, ok := methodSet[c.Method()]; !ok {
			return c.Next()
		}

		key := cfg.KeyFunc(c)

		// Check cache for a hit.
		cached, err := cfg.Store.Get(key)
		if err == nil && cached != nil {
			// HIT -- replay cached response.
			c.SetHeader("X-Cache", "HIT")
			age := int(time.Since(cached.CachedAt).Seconds())
			c.SetHeader("Age", strconv.Itoa(age))
			for k, v := range cached.Headers {
				c.SetHeader(k, v)
			}
			return c.Status(cached.StatusCode).SendBytes(cached.Body)
		}

		// MISS -- store caching metadata in context for helper functions.
		c.SetHeader("X-Cache", "MISS")
		c.Set(cacheKeyKey, key)
		c.Set(cacheStoreKey, cfg.Store)
		c.Set(cacheTTLKey, cfg.TTL)
		c.Set(cacheCodesKey, cfg.StatusCodes)

		return c.Next()
	}
}

// CacheJSON marshals the data as JSON, stores it in the cache (if the cache
// middleware is active and the status code is cacheable), then sends the JSON
// response to the client.
//
// This is the recommended way to send cacheable JSON responses.
func CacheJSON(c *kruda.Ctx, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Store in cache if middleware is active.
	storeCachedResponse(c, "application/json; charset=utf-8", body)

	return c.JSON(data)
}

// CacheBytes stores raw bytes in the cache (if the cache middleware is active
// and the status code is cacheable), then sends the bytes to the client with
// the given content type.
func CacheBytes(c *kruda.Ctx, contentType string, body []byte) error {
	// Store in cache if middleware is active.
	storeCachedResponse(c, contentType, body)

	return c.SetContentType(contentType).SendBytes(body)
}

// storeCachedResponse stores a response in the cache store if:
// 1. The cache middleware set a key in the context (i.e., this was a cache MISS).
// 2. The response status code is in the allowed list.
func storeCachedResponse(c *kruda.Ctx, contentType string, body []byte) {
	key, ok := c.Get(cacheKeyKey).(string)
	if !ok || key == "" {
		return
	}

	store, ok := c.Get(cacheStoreKey).(Store)
	if !ok || store == nil {
		return
	}

	ttl, ok := c.Get(cacheTTLKey).(time.Duration)
	if !ok {
		ttl = 5 * time.Minute
	}

	codes, _ := c.Get(cacheCodesKey).([]int)

	// Determine effective status code (0 means 200 -- handler hasn't set status yet).
	status := c.StatusCode()
	if status == 0 {
		status = 200
	}

	if !isAllowedStatus(status, codes) {
		return
	}

	// Make a copy of the body for the cache so it is independent of any pooled buffers.
	bodyCopy := make([]byte, len(body))
	copy(bodyCopy, body)

	resp := &CachedResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": contentType},
		Body:       bodyCopy,
		CachedAt:   time.Now(),
	}

	_ = store.Set(key, resp, ttl)
}

// isAllowedStatus checks if the status code is in the allowed list.
func isAllowedStatus(status int, codes []int) bool {
	if len(codes) == 0 {
		return status == 200
	}
	for _, code := range codes {
		if code == status {
			return true
		}
	}
	return false
}
