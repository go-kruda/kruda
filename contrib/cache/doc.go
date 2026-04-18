// Package cache provides response caching middleware for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/cache"
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
//
// # What it does
//
// The middleware looks up cached responses on incoming requests. On a HIT it
// sets X-Cache: HIT and Age headers, replays the cached body, and short-
// circuits the handler chain. On a MISS it sets X-Cache: MISS and stores
// caching metadata in the request context — handlers must use [CacheJSON]
// or [CacheBytes] to send a response that should also be persisted.
//
// # Configuration
//
//   - TTL: time-to-live for cached responses (default 5 minutes)
//   - KeyFunc: cache key generator (default: METHOD + ":" + path)
//   - Methods: HTTP methods eligible for caching (default ["GET"])
//   - StatusCodes: response codes eligible for caching (default [200])
//   - Store: backing storage (default in-memory store, capacity 1000)
//   - Skip: per-request bypass function
//
// # See also
//
//   - [Store] — interface for plugging in custom backends (e.g. Redis)
//   - [NewMemoryStore] — in-memory LRU store shipped with the package
package cache
