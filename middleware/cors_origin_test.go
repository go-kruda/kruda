package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
)

func TestCORSTrailingSlashOriginMatches(t *testing.T) {
	app := kruda.New()
	app.Use(CORS(CORSConfig{AllowOrigins: []string{"https://app.example.com/"}}))
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want the browser's origin", got)
	}
}

func TestCORSRejectsOriginWithPath(t *testing.T) {
	for _, o := range []string{"https://a.com/app", "https://a.com?x=1", "https://a.com#f", "https://u@a.com", "https://a.com/?"} {
		t.Run(o, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("CORS accepted %q, which can never match a browser Origin", o)
				}
			}()
			CORS(CORSConfig{AllowOrigins: []string{o}})
		})
	}
}
