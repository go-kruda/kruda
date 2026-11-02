//go:build !windows

package cache

import (
	"bytes"
	"github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
	"testing"
)

func TestCacheJSONMatchesFastHTTPEncoder(t *testing.T) {
	app := kruda.New(kruda.WithJSONEncoder(func(any) ([]byte, error) { return []byte(`"encoder"`), nil }), kruda.WithJSONStreamEncoder(func(b *bytes.Buffer, _ any) error { b.WriteString(`"stream"`); return nil }))
	app.Use(New())
	app.Get("/plain", func(c *kruda.Ctx) error { return c.JSON(kruda.Map{"value": 1}) })
	app.Get("/cache", func(c *kruda.Ctx) error { return CacheJSON(c, kruda.Map{"value": 1}) })
	var plain fasthttp.RequestCtx
	plain.Request.SetRequestURI("/plain")
	plain.Request.Header.SetMethod("GET")
	app.ServeFast(&plain)
	for range 2 {
		var cached fasthttp.RequestCtx
		cached.Request.SetRequestURI("/cache")
		cached.Request.Header.SetMethod("GET")
		app.ServeFast(&cached)
		if !bytes.Equal(cached.Response.Body(), plain.Response.Body()) {
			t.Fatalf("plain=%s cached=%s", plain.Response.Body(), cached.Response.Body())
		}
	}
}
