package kruda

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHeaderHostOnNetHTTP reads the Host header on net/http, which moves it
// out of the header map into Request.Host. It must match what Wing and
// fasthttp return.
func TestHeaderHostOnNetHTTP(t *testing.T) {
	app := New(NetHTTP())
	var got [2]string
	app.Get("/", func(c *Ctx) error {
		got = [2]string{c.Header("Host"), c.Header("host")}
		return c.Text("ok")
	})
	app.Compile()
	req := httptest.NewRequest(http.MethodGet, "http://example.com:8080/", nil)
	app.ServeHTTP(httptest.NewRecorder(), req)
	if got != [2]string{"example.com:8080", "example.com:8080"} {
		t.Fatalf("Header(Host), Header(host) = %q", got)
	}
}
