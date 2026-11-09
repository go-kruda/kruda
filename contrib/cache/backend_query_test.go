//go:build linux || darwin

package cache

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
)

func TestCacheQueryAcrossBackends(t *testing.T) {
	backends := []struct {
		name   string
		option kruda.Option
	}{
		{"nethttp", kruda.NetHTTP()},
		{"nethttp-generic", kruda.WithTransport(transport.NewNetHTTP(transport.NetHTTPConfig{}))},
		{"fasthttp", kruda.FastHTTP()},
		{"wing-inline", kruda.WithTransport(kruda.NewWingTransport(kruda.WingConfig{Workers: 1}))},
		{"wing-pool", kruda.WithTransport(kruda.NewWingTransport(kruda.WingConfig{Workers: 1, HandlerPoolSize: 2, DefaultPreset: kruda.Arrow}))},
		{"wing-spawn", kruda.WithTransport(kruda.NewWingTransport(kruda.WingConfig{Workers: 1, DefaultPreset: kruda.Arrow.With(kruda.Dispatch(kruda.Spawn))}))},
	}
	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			store := NewMemoryStore(100)
			defer store.Close()
			app := kruda.New(backend.option)
			app.Use(New(Config{Store: store}))
			var calls atomic.Int32
			app.Get("/data", func(c *kruda.Ctx) error {
				calls.Add(1)
				return CacheBytes(c, "text/plain", []byte(c.Request().(interface{ RawQuery() string }).RawQuery()))
			})
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- app.Serve(ln) }()
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				app.Shutdown(ctx)
				select {
				case <-done:
				case <-ctx.Done():
					t.Error(ctx.Err())
				}
			}()
			client := &http.Client{Timeout: 3 * time.Second}
			defer client.CloseIdleConnections()
			base := "http://" + ln.Addr().String() + "/data?"
			deadline := time.Now().Add(3 * time.Second)
			for {
				resp, e := client.Get(base + "ready=1")
				if e == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					break
				}
				if time.Now().After(deadline) {
					t.Fatal(e)
				}
				time.Sleep(time.Millisecond * 10)
			}
			queries := []string{"id=alpha", "id=beta", "a=one%26b%3Dtwo", "a=one&b=two", "a=one%2C+two", "a=one&a=two"}
			for _, q := range queries {
				before := calls.Load()
				for n := 0; n < 2; n++ {
					resp, e := client.Get(base + q)
					if e != nil {
						t.Fatal(e)
					}
					body, e := io.ReadAll(resp.Body)
					resp.Body.Close()
					if e != nil {
						t.Fatal(e)
					}
					want := "MISS"
					if n == 1 {
						want = "HIT"
					}
					if string(body) != q || resp.Header.Get("X-Cache") != want {
						t.Fatalf("%s: body=%q cache=%q want %s", q, body, resp.Header.Get("X-Cache"), want)
					}
				}
				if calls.Load() != before+2 {
					t.Fatal("cache hit skipped route handler")
				}
			}
		})
	}
}
