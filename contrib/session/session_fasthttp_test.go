//go:build !windows

package session

import (
	"github.com/go-kruda/kruda"
	"github.com/valyala/fasthttp"
	"testing"
)

func TestFastHTTPSessionCookieRoundTrip(t *testing.T) {
	store := NewMemoryStore()
	defer store.Close()
	app := kruda.New(kruda.FastHTTP())
	app.Use(New(Config{Store: store}))
	app.Get("/set", func(c *kruda.Ctx) error { GetSession(c).Set("user", "alice"); return c.Text("set") })
	app.Get("/get", func(c *kruda.Ctx) error { return c.Text(GetSession(c).GetString("user")) })
	app.Compile()
	var first fasthttp.RequestCtx
	first.Request.SetRequestURI("/set")
	first.Request.Header.SetMethod("GET")
	app.ServeFast(&first)
	var cookie fasthttp.Cookie
	cookie.SetKey("_session")
	if !first.Response.Header.Cookie(&cookie) || len(cookie.Value()) == 0 {
		t.Fatal("missing session cookie")
	}
	var second fasthttp.RequestCtx
	second.Request.SetRequestURI("/get")
	second.Request.Header.SetMethod("GET")
	second.Request.Header.SetCookie("_session", string(cookie.Value()))
	app.ServeFast(&second)
	if got := string(second.Response.Body()); got != "alice" {
		t.Fatalf("session value=%q", got)
	}
}
