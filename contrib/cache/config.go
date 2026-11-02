package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
	"net/http"
)

// Config holds configuration for the cache middleware.
type Config struct {
	// TTL is the time-to-live for cached responses.
	// Default: 5 minutes
	TTL time.Duration

	// KeyFunc generates the cache key from the request context.
	// Default: method + path + raw query + auth/cookie hashes
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

// defaultKeyFunc generates a cache key from method, path, query, and
// authenticated request identity headers.
func defaultKeyFunc(c *kruda.Ctx) string {
	var b strings.Builder
	b.WriteString(c.Method())
	b.WriteByte(':')
	b.WriteString(url.QueryEscape(c.Path()))

	raw, ok := requestRawQuery(c)
	if !ok {
		return ""
	}
	if raw != "" {
		b.WriteByte('?')
		b.WriteString(url.QueryEscape(raw))
	}

	appendHeaderHash(&b, "auth", c.Header("Authorization"))
	appendHeaderHash(&b, "cookie", c.Header("Cookie"))
	return b.String()
}

func appendHeaderHash(b *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	sum := sha256.Sum256([]byte(value))
	b.WriteByte('|')
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(hex.EncodeToString(sum[:8]))
}

func requestRawQuery(c *kruda.Ctx) (string, bool) {
	r := c.Request()
	if r == nil {
		return "", false
	}
	if q, ok := r.(interface{ RawQuery() string }); ok {
		return q.RawQuery(), true
	}
	switch raw := r.RawRequest().(type) {
	case *http.Request:
		return raw.URL.RawQuery, true
	case *fasthttp.RequestCtx:
		return string(raw.URI().QueryString()), true
	}
	return "", false
}
