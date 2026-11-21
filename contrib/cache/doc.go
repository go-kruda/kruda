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
//	    if hit, err := cache.Replay(c); hit { return err }
//	    data := fetchExpensiveData()
//	    return cache.CacheJSON(c, data)
//	})
//
// With custom config:
//
//	app.Use(cache.New(cache.Config{
//	    TTL:     10 * time.Minute,
//	    Methods: []string{"GET"},
//	    Store:   cache.NewMemoryStore(5000).SetMaxBytes(256 << 20),
//	}))
//
// # What it does
//
// The middleware looks up cached responses but always runs middleware and the
// handler. After authorization, call [Replay] before expensive work to send a
// hit. [CacheJSON] and [CacheBytes] also replay matching cached responses, or
// store and send the response on a miss.
//
// CacheJSON requires core EncodeJSON support to store responses. On older cores
// it sends JSON without caching; CacheBytes remains available.
//
// # Configuration
//
//   - TTL: time-to-live for cached responses (default 5 minutes)
//   - KeyFunc: cache key generator (default: method, path, raw query and a
//     hash of the Authorization and Cookie headers; write your own when users
//     are identified any other way)
//   - Methods: HTTP methods eligible for caching (default ["GET"])
//   - StatusCodes: response codes eligible for caching (default [200])
//   - Store: backing storage (default in-memory store, capacity 1000)
//   - Skip: per-request bypass function
//
// # See also
//
//   - [Store] — interface for plugging in custom backends (e.g. Redis)
//   - [NewMemoryStore] — in-memory store shipped with the package; it evicts
//     oldest entries past its entry limit or its byte budget
//     ([DefaultMaxBytes], change with [MemoryStore.SetMaxBytes])
package cache
