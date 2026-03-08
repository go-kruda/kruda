//go:build !windows

package bench

import (
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

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
