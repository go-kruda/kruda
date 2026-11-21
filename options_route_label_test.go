package kruda

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOptionsPreflightRouteIsPattern sends preflights to a param route that
// has no OPTIONS handler. Global middleware runs for them, and c.Route() must
// give the route pattern, not the raw path, or every preflight path becomes a
// new metrics label.
func TestOptionsPreflightRouteIsPattern(t *testing.T) {
	app := New(NetHTTP())
	var seen []string
	app.Use(func(c *Ctx) error { seen = append(seen, c.Route()); return c.Next() })
	app.Get("/users/:id", func(c *Ctx) error { return c.Text("ok") })
	app.Get("/static", func(c *Ctx) error { return c.Text("ok") })
	app.Compile()
	for _, p := range []string{"/users/a1", "/users/b2", "/static"} {
		app.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodOptions, p, nil))
	}
	want := []string{"/users/:id", "/users/:id", "/static"}
	if len(seen) != len(want) {
		t.Fatalf("middleware ran %d times, want %d", len(seen), len(want))
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("Route() on preflight %d = %q, want %q", i, seen[i], want[i])
		}
	}
}
