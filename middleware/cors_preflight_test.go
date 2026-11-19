package middleware

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// docsCORS is the setup shown in docs/guide/security.md.
func docsCORS() kruda.HandlerFunc {
	return CORS(CORSConfig{
		AllowOrigins:     []string{"https://app.example.com", "https://admin.example.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
		ExposeHeaders:    []string{"X-Request-ID"},
		MaxAge:           3600,
	})
}

func preflight(url string) *http.Request {
	r, _ := http.NewRequest(http.MethodOptions, url, nil)
	r.Header.Set("Origin", "https://app.example.com")
	r.Header.Set("Access-Control-Request-Method", "POST")
	r.Header.Set("Access-Control-Request-Headers", "Content-Type")
	return r
}

func checkPreflight(t *testing.T, code int, h http.Header) {
	t.Helper()
	if code != http.StatusNoContent || h.Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("preflight = %d, Access-Control-Allow-Origin %q; want 204 and the origin", code, h.Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSPreflightWithoutOptionsRoute(t *testing.T) {
	t.Run("ServeHTTP", func(t *testing.T) {
		app := kruda.New()
		app.Use(docsCORS())
		app.Post("/users", func(c *kruda.Ctx) error { return c.Text("created") })
		app.Compile()
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, preflight("http://x/users"))
		checkPreflight(t, rec.Code, rec.Header())
	})
	for name, opt := range map[string]kruda.Option{"Wing": kruda.Wing(), "fasthttp": kruda.FastHTTP(), "NetHTTP": kruda.NetHTTP()} {
		t.Run(name, func(t *testing.T) {
			app := kruda.New(opt)
			app.Use(docsCORS())
			app.Post("/users/:id", func(c *kruda.Ctx) error { return c.Text("updated") })
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			addr := ln.Addr().String()
			_ = ln.Close()
			go func() { _ = app.Listen(addr) }()
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = app.Shutdown(ctx)
			}()
			var resp *http.Response
			for i := 0; i < 100; i++ {
				if resp, err = http.DefaultClient.Do(preflight("http://" + addr + "/users/7")); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			checkPreflight(t, resp.StatusCode, resp.Header)
		})
	}
}

func TestOptionsWithoutCORSStillAnswers405(t *testing.T) {
	app := kruda.New()
	app.Use(func(c *kruda.Ctx) error { return c.Next() })
	app.Post("/users", func(c *kruda.Ctx) error { return c.Text("created") })
	app.Compile()

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/users", nil))
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") == "" {
		t.Fatalf("OPTIONS /users = %d Allow %q; want 405 with Allow", rec.Code, rec.Header().Get("Allow"))
	}

	rec = httptest.NewRecorder()
	app.ServeHTTP(rec, preflight("http://x/missing"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("OPTIONS /missing = %d, want 404", rec.Code)
	}
}
