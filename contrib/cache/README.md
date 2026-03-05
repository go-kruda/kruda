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
    return cache.CacheBytes(c, content, "text/plain")
})
```

## Config

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| TTL | time.Duration | 1*time.Hour | Cache expiration time |
| Skipper | func(*kruda.Ctx) bool | nil | Skip caching condition |
| KeyGenerator | func(*kruda.Ctx) string | URL+Method | Cache key function |
| Store | Store | MemoryStore | Cache storage backend |