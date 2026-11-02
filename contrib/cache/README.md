# Cache

Response caching middleware with cache hit serving and helper functions.

## Install

```bash
go get github.com/go-kruda/kruda/contrib/cache
```

## Usage

```go
import "github.com/go-kruda/kruda/contrib/cache"

app.Use(cache.New(cache.Config{TTL: 5*time.Minute}))

app.Get("/data", func(c *kruda.Ctx) error {
    if hit, err := cache.Replay(c); hit {
        return err
    }
    data := map[string]interface{}{"result": "expensive computation"}
    return cache.CacheJSON(c, data)
})

app.Get("/file", func(c *kruda.Ctx) error {
    content := []byte("file content")
    return cache.CacheBytes(c, "text/plain", content)
})
```

Cache hits still run middleware and the route handler, so authorization checks
run on every request. Place `Replay` after authorization and before expensive
work to avoid recomputing a cached result. `CacheJSON` and `CacheBytes` replay cached bytes at the point
where the handler calls them; code before those helpers runs on every request.

The default key preserves the complete raw query on built-in transports,
including repeated keys. Reordering the query can produce a separate entry.
On older Wing cores without raw-query access, default caching is bypassed; use
the updated core or a lossless custom `KeyFunc`.

Custom transports must expose `RawQuery() string` or a supported raw HTTP request;
otherwise default caching is bypassed because query data cannot be recovered
without loss. A custom `KeyFunc` can define a key for other transports.

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| TTL | time.Duration | 5*time.Minute | Cache expiration time |
| KeyFunc | func(*kruda.Ctx) string | method + path + query + auth/cookie hash | Cache key function |
| Methods | []string | ["GET"] | HTTP methods eligible for caching |
| StatusCodes | []int | [200] | Status codes eligible for caching |
| Store | Store | MemoryStore | Cache storage backend |
| Skip | func(*kruda.Ctx) bool | nil | Skip caching condition |

`CacheJSON` encodes once using `Ctx.EncodeJSON`, then stores and sends the same
bytes. Update core alongside this module to enable JSON caching. On older cores
without `EncodeJSON` (including v1.7.2), it sends through `c.JSON` without storing
a response, preserving custom encoder behavior. `CacheBytes` still caches on
those cores when a lossless query source is available.
