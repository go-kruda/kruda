//go:build !windows

package bench

import (
	"testing"

	kruda "github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
)

func BenchmarkKrudaFH_StaticGET(b *testing.B) {
	app := kruda.New(kruda.FastHTTP())
	app.Get("/", func(c *kruda.Ctx) error {
		return c.Text("Hello, World!")
	})
	app.Compile()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.SetMethod("GET")
		for pb.Next() {
			ctx.Response.Reset()
			app.ServeFast(ctx)
		}
	})
}

func BenchmarkKrudaFH_ParamGET(b *testing.B) {
	app := kruda.New(kruda.FastHTTP())
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return c.Text(c.Param("id"))
	})
	app.Compile()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/users/42")
		ctx.Request.Header.SetMethod("GET")
		for pb.Next() {
			ctx.Response.Reset()
			app.ServeFast(ctx)
		}
	})
}
