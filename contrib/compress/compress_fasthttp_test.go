//go:build !windows

package compress

import (
	"github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
	"strings"
	"testing"
)

func TestCompressionPreservesVaryOnFastHTTP(t *testing.T) {
	app := kruda.New()
	app.Use(New(Config{MinSize: 1}))
	app.Get("/", func(c *kruda.Ctx) error { c.SetHeader("Vary", "Origin"); return CompressText(c, "compressed response") })
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	ctx.Request.Header.Set("Accept-Encoding", "gzip")
	app.ServeFast(&ctx)
	tokens := map[string]bool{}
	for _, v := range ctx.Response.Header.PeekAll("Vary") {
		for _, part := range strings.Split(string(v), ",") {
			tokens[strings.TrimSpace(part)] = true
		}
	}
	if !tokens["Origin"] || !tokens["Accept-Encoding"] {
		t.Fatalf("Vary=%q", ctx.Response.Header.PeekAll("Vary"))
	}
}
