package middleware

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

func lateHandler(c *kruda.Ctx) error {
	time.Sleep(60 * time.Millisecond)
	return c.Text("late")
}

// recordErr returns middleware that stores what the rest of the chain
// returned, which is what Logger and metrics middleware see.
func recordErr(got chan<- error) kruda.HandlerFunc {
	return func(c *kruda.Ctx) error {
		err := c.Next()
		got <- err
		return err
	}
}

func TestTimeoutKeepsAResponseAlreadySent(t *testing.T) {
	t.Run("ServeHTTP", func(t *testing.T) {
		got := make(chan error, 1)
		app := kruda.New()
		app.Use(recordErr(got), Timeout(10*time.Millisecond))
		app.Get("/late", lateHandler)
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/late", nil))
		if rec.Code != 200 || rec.Body.String() != "late" {
			t.Fatalf("got %d %q, want 200 \"late\"", rec.Code, rec.Body.String())
		}
		if err := <-got; err != nil {
			t.Fatalf("chain returned %v after a 200 was sent; want nil", err)
		}
	})
	for name, opt := range map[string]kruda.Option{"Wing": kruda.Wing(), "fasthttp": kruda.FastHTTP(), "NetHTTP": kruda.NetHTTP()} {
		t.Run(name, func(t *testing.T) {
			got := make(chan error, 1)
			app := kruda.New(opt)
			app.Use(recordErr(got), Timeout(10*time.Millisecond))
			app.Get("/late", lateHandler)
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
				if resp, err = http.Get("http://" + addr + "/late"); err == nil {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode != 200 || string(body) != "late" {
				t.Fatalf("got %d %q, want 200 \"late\"", resp.StatusCode, body)
			}
			if err := <-got; err != nil {
				t.Fatalf("chain returned %v after a 200 was sent; want nil", err)
			}
		})
	}
}
