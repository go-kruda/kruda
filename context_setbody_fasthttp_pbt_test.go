//go:build !windows

package kruda

import (
	"bytes"
	"testing"
	"testing/quick"

	"github.com/valyala/fasthttp"
)

// Property: SetBody via ServeFastHTTP Path
//
// For any byte slice, SetBody(data) via ServeFastHTTP writes exact bytes to
// fasthttp response body without nil writer panic (c.writer is nil in fasthttp path).

func TestPropertySetBodyViaServeFastHTTP(t *testing.T) {
	cfg := &quick.Config{MaxCount: 200}

	t.Run("ExactBytesWritten", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			app.Get("/test", func(c *Ctx) error {
				c.SetBody(data)
				return nil
			})
			app.Compile()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/test")
			ctx.Request.Header.SetMethod("GET")

			app.ServeFast(ctx)

			return bytes.Equal(ctx.Response.Body(), data)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("NoPanicOnNilWriter", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			app.Get("/test", func(c *Ctx) error {
				// Verify c.writer is nil in fasthttp path
				if c.writer != nil {
					return nil // not the fasthttp path
				}
				c.SetBody(data)
				return nil
			})
			app.Compile()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/test")
			ctx.Request.Header.SetMethod("GET")

			// Should not panic despite c.writer being nil
			app.ServeFast(ctx)

			return bytes.Equal(ctx.Response.Body(), data)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("ContentLengthSet", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			app.Get("/test", func(c *Ctx) error {
				c.SetBody(data)
				return nil
			})
			app.Compile()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/test")
			ctx.Request.Header.SetMethod("GET")

			app.ServeFast(ctx)

			return ctx.Response.Header.ContentLength() == len(data)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})

	t.Run("ContentTypePreserved", func(t *testing.T) {
		f := func(data []byte) bool {
			app := New()
			app.Get("/test", func(c *Ctx) error {
				c.SetContentType("application/json")
				c.SetBody(data)
				return nil
			})
			app.Compile()

			ctx := &fasthttp.RequestCtx{}
			ctx.Request.SetRequestURI("/test")
			ctx.Request.Header.SetMethod("GET")

			app.ServeFast(ctx)

			ct := string(ctx.Response.Header.ContentType())
			return ct == "application/json" && bytes.Equal(ctx.Response.Body(), data)
		}
		if err := quick.Check(f, cfg); err != nil {
			t.Error(err)
		}
	})
}
