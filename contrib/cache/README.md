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
    data := map[string]interface{}{"result": "expensive computation"}
    return cache.CacheJSON(c, data)
})

app.Get("/file", func(c *kruda.Ctx) error {
    content := []byte("file content")
    return cache.CacheBytes(c, "text/plain", content)
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| TTL | time.Duration | 5*time.Minute | Cache expiration time |
| KeyFunc | func(*kruda.Ctx) string | method + path + query + auth/cookie hash | Cache key function |
| Methods | []string | ["GET"] | HTTP methods eligible for caching |
| StatusCodes | []int | [200] | Status codes eligible for caching |
| Store | Store | MemoryStore | Cache storage backend |
| Skip | func(*kruda.Ctx) bool | nil | Skip caching condition |
