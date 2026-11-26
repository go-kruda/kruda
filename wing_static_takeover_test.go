//go:build linux || darwin

package kruda

import (
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

// TestWingStaticPresetAfterTakeover asks for a StaticText route on a connection
// a Takeover route already owns. It gets what a fresh connection would.
func TestWingStaticPresetAfterTakeover(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			app := New(Wing())
			app.Get("/db", func(c *Ctx) error { return c.Text("db") }, DB)
			var ran atomic.Int32
			handler := func(c *Ctx) error {
				ran.Add(1)
				return c.Text("handler")
			}
			static := StaticText(200, "text/plain", "static")
			app.Get("/s", handler, static)
			app.Head("/s", handler, static)
			addr, stop := startWingApp(t, app)
			defer stop()
			conn, br := framingDial(t, addr)
			// The trailing /db proves the static reply left the stream in step.
			if _, err := io.WriteString(conn, framingGet("/db")+method+" /s HTTP/1.1\r\nHost: x\r\n\r\n"+framingGet("/db")); err != nil {
				t.Fatal(err)
			}
			static200 := "static"
			if method == "HEAD" {
				static200 = ""
			}
			for i, want := range []string{"db", static200, "db"} {
				m := "GET"
				if i == 1 {
					m = method
				}
				res, err := http.ReadResponse(br, &http.Request{Method: m})
				if err != nil {
					t.Fatalf("response %d: %v", i, err)
				}
				body, err := io.ReadAll(res.Body)
				if err != nil || string(body) != want {
					t.Fatalf("response %d body %q err %v, want %q", i, body, err, want)
				}
				if i == 1 && res.ContentLength != int64(len("static")) {
					t.Fatalf("static Content-Length %d", res.ContentLength)
				}
			}
			if ran.Load() != 0 {
				t.Fatal("static route ran its handler")
			}
		})
	}
}
