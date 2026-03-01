//go:build !windows

package bench

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	kruda "github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
)

// Fair fasthttp-level benchmarks: both Kruda and Fiber use fasthttp.RequestCtx directly.
// No httptest.NewRecorder overhead — pure framework comparison on fasthttp.

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

func BenchmarkFiberFH_StaticGET(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	handler := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.SetMethod("GET")
		for pb.Next() {
			ctx.Response.Reset()
			handler(ctx)
		}
	})
}

func BenchmarkFiberFH_ParamGET(b *testing.B) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	app.Get("/users/:id", func(c *fiber.Ctx) error {
		return c.SendString(c.Params("id"))
	})
	handler := app.Handler()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := &fasthttp.RequestCtx{}
		ctx.Request.SetRequestURI("/users/42")
		ctx.Request.Header.SetMethod("GET")
		for pb.Next() {
			ctx.Response.Reset()
			handler(ctx)
		}
	})
}
