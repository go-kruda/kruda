package kruda

import (
	"net/http/httptest"
	"testing"
)

func TestUnregisteredMethodDoesNotReachStandardRoute(t *testing.T) {
	app := New(NetHTTP())
	app.Post("/users", func(c *Ctx) error { return c.Text("post") })
	app.Compile()
	for _, m := range []string{"POLL", "POXX", "GEX", "HEXX"} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(m, "/users", nil))
		if w.Code != 405 || w.Body.String() == "post" {
			t.Errorf("%s /users = %d %q, want 405 and not the POST handler", m, w.Code, w.Body.String())
		}
	}
}

func TestCustomMethodKeepsItsOwnTree(t *testing.T) {
	app := New(NetHTTP())
	app.Post("/users", func(c *Ctx) error { return c.Text("post") })
	app.addRoute("POLL", "/users", func(c *Ctx) error { return c.Text("poll") })
	app.Compile()
	for m, want := range map[string]string{"POST": "post", "POLL": "poll"} {
		w := httptest.NewRecorder()
		app.ServeHTTP(w, httptest.NewRequest(m, "/users", nil))
		if w.Body.String() != want {
			t.Errorf("%s /users = %q, want %q", m, w.Body.String(), want)
		}
	}
}
