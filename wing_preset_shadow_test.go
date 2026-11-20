//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"testing"
)

// TestWingWildcardPresetDoesNotShadowPlainRoutes registers a StaticText
// wildcard next to more specific routes that have no preset. Those routes win
// in the router, so Wing must run their handler and middleware instead of
// answering with the wildcard's prebuilt response.
func TestWingWildcardPresetDoesNotShadowPlainRoutes(t *testing.T) {
	app := New(Wing())
	app.Get("/files/*path", func(c *Ctx) error { return c.Text("public") }, StaticText(200, "text/plain", "public"))
	guard := func(c *Ctx) error { return c.Status(401).Text("guarded") }
	app.Get("/files/account", guard)
	app.Get("/files/:id/meta", guard)
	addr, stop := startWingApp(t, app)
	defer stop()
	for path, want := range map[string]string{
		"/files/account":   "guarded",
		"/files/7/meta":    "guarded",
		"/files/other.txt": "public",
	} {
		resp, err := http.Get("http://" + addr + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if string(body) != want {
			t.Errorf("GET %s = %d %q, want %q", path, resp.StatusCode, body, want)
		}
	}
}
