//go:build !windows

package kruda

import (
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
)

// TestServeFastRawHeaderNamesCannotAddFields sets raw header names with a line
// break through the App's own fasthttp path, the macOS default. fasthttp writes
// names as given, so such a name could add a field of its own.
func TestServeFastRawHeaderNamesCannotAddFields(t *testing.T) {
	app := New()
	app.Get("/", func(c *Ctx) error {
		h := c.ResponseWriter().Header()
		h.Set("X-Bad\r\nX-Injected", "1")
		h.Add("X-Bad\nX-Injected", "2")
		h.Set("X-Good", "3")
		return c.Text("ok")
	})
	var fast fasthttp.RequestCtx
	fast.Request.SetRequestURI("/")
	fast.Request.Header.SetMethod("GET")
	app.ServeFast(&fast)
	out := fast.Response.String()
	if strings.Contains(out, "X-Injected") || !strings.Contains(out, "X-Good: 3\r\n") {
		t.Fatalf("response %q", out)
	}
}
