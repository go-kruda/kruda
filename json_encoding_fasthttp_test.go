//go:build !windows

package kruda

import (
	"bytes"
	"github.com/valyala/fasthttp"
	"strings"
	"testing"
)

func TestFastHTTPAddHeaderPreservesJSONEncoderSelection(t *testing.T) {
	app := New(WithJSONEncoder(func(any) ([]byte, error) { return []byte(`"encoder"`), nil }), WithJSONStreamEncoder(func(b *bytes.Buffer, _ any) error { b.WriteString(`"stream"`); return nil }))
	app.Get("/", func(c *Ctx) error { c.AddHeader("Vary", "Origin"); return c.JSON(1) })
	var ctx fasthttp.RequestCtx
	ctx.Request.SetRequestURI("/")
	ctx.Request.Header.SetMethod("GET")
	app.ServeFast(&ctx)
	if string(ctx.Response.Body()) != `"stream"` {
		t.Fatalf("AddHeader changed existing JSON encoding path: %s", ctx.Response.Body())
	}
}
func TestFastHTTPResponseHeaderSetAndAppendOrder(t *testing.T) {
	for _, replace := range []bool{false, true} {
		app := New()
		app.Get("/", func(c *Ctx) error {
			if replace {
				c.AddHeader("Vary", "Origin")
				c.SetHeader("Vary", "Accept-Encoding")
			} else {
				c.SetHeader("Vary", "Origin")
				c.AddHeader("Vary", "Accept-Encoding")
			}
			return c.Text("ok")
		})
		var ctx fasthttp.RequestCtx
		ctx.Request.SetRequestURI("/")
		ctx.Request.Header.SetMethod("GET")
		app.ServeFast(&ctx)
		var headers []string
		for _, value := range ctx.Response.Header.PeekAll("Vary") {
			headers = append(headers, string(value))
		}
		values := strings.Join(headers, ",")
		if !strings.Contains(values, "Accept-Encoding") || strings.Contains(values, "Origin") == replace {
			t.Fatalf("replace=%v Vary=%s", replace, values)
		}
	}
}
