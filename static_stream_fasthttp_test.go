//go:build !windows

package kruda

import (
	"testing"

	"github.com/valyala/fasthttp"
)

func TestStaticFastHTTPDefersFileReadsAndClosesOnReset(t *testing.T) {
	files := &boundedStaticFS{size: 64 << 20}
	app := New(FastHTTP())
	app.StaticFS("/assets", files)
	app.Compile()
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/assets/asset.bin")
	app.ServeFastHTTP(&ctx)
	defer ctx.Response.Reset()
	if !ctx.Response.IsBodyStream() {
		t.Fatal("static file was buffered instead of streamed")
	}
	if n := files.maxRead.Load(); n != 0 {
		t.Fatalf("handler read file before transport delivery: max read=%d", n)
	}
	if n := files.closes.Load(); n != 0 {
		t.Fatalf("handler closed deferred file %d times", n)
	}
	ctx.Response.Reset()
	if n := files.closes.Load(); n != 1 {
		t.Fatalf("reset closed file %d times, want 1", n)
	}
}

func TestStaticFastHTTPPreservesAlreadySentResponse(t *testing.T) {
	files := &boundedStaticFS{size: 1024}
	app := New(FastHTTP())
	app.BeforeHandle(func(c *Ctx) error { return c.Text("already") })
	app.StaticFS("/assets", files)
	app.Compile()
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/assets/asset.bin")
	app.ServeFastHTTP(&ctx)
	defer ctx.Response.Reset()
	if ctx.Response.IsBodyStream() || string(ctx.Response.Body()) != "already" {
		t.Fatal("static replaced an existing fasthttp response")
	}
}
