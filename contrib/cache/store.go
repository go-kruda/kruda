package cache

import "time"

// Store is the interface for cache backends.
// Implementations must be safe for concurrent use.
type Store interface {
	// Get retrieves a cached response by key.
	// Returns nil, nil if the key does not exist or has expired.
	Get(key string) (*CachedResponse, error)

	// Set stores a cached response with the given TTL.
	// If the key already exists, it is overwritten.
	Set(key string, resp *CachedResponse, ttl time.Duration) error

	// Delete removes a cached response from the store.
	Delete(key string) error
}

// CachedResponse holds a cached HTTP response.
type CachedResponse struct {
	// StatusCode is the HTTP status code of the cached response.
	StatusCode int

	// Headers contains the response headers to replay on cache hit.
	Headers map[string]string

	// Body is the raw response body bytes.
	Body []byte

	// CachedAt is the time the response was cached.
	// Used to compute the Age header on cache hits.
	CachedAt time.Time
}
