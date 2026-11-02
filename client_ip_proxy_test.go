package kruda

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"
)

func TestClientIPProxyPolicyAcrossBackends(t *testing.T) {
	backends := []struct {
		name   string
		option Option
	}{{"nethttp", NetHTTP()}, {"fasthttp", FastHTTP()}, {"wing", Wing()}}
	for _, backend := range backends {
		if runtime.GOOS == "windows" && backend.name != "nethttp" {
			continue
		}
		for _, tc := range []struct {
			name    string
			enabled bool
			ips     []string
			want    string
		}{
			{"trust flag alone", true, nil, "127.0.0.1"},
			{"trusted", true, []string{"::ffff:127.0.0.1", "10.0.0.2"}, "192.0.2.1"},
			{"untrusted peer", true, []string{"10.0.0.2"}, "127.0.0.1"},
			{"disabled", false, []string{"127.0.0.1"}, "127.0.0.1"},
		} {
			t.Run(backend.name+"/"+tc.name, func(t *testing.T) {
				app := New(backend.option, WithTrustProxy(tc.enabled), WithTrustedProxies(tc.ips...))
				app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
				app.Get("/ip", func(c *Ctx) error { return c.Text(c.IP()) })
				ln, err := net.Listen("tcp4", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() { done <- app.Serve(ln) }()
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = app.Shutdown(ctx)
					select {
					case <-done:
					case <-ctx.Done():
						t.Error(ctx.Err())
					}
				}()
				client := &http.Client{Timeout: time.Second}
				defer client.CloseIdleConnections()
				endpoint := "http://" + ln.Addr().String()
				deadline := time.Now().Add(3 * time.Second)
				for {
					r, e := client.Get(endpoint + "/ready")
					if e == nil {
						r.Body.Close()
						if r.StatusCode == 200 {
							break
						}
					}
					if time.Now().After(deadline) {
						t.Fatalf("startup: %v", e)
					}
					time.Sleep(10 * time.Millisecond)
				}
				for _, v := range []struct {
					name    string
					headers []string
					want    string
				}{
					{"chain", []string{"attacker, 192.0.2.1, 10.0.0.2"}, tc.want},
					{"duplicate", []string{"198.51.100.1", "192.0.2.1"}, "127.0.0.1"},
					{"empty duplicate", []string{"", "192.0.2.1"}, "127.0.0.1"},
					{"invalid", []string{"bad"}, "127.0.0.1"},
					{"no header after duplicate", nil, "127.0.0.1"},
				} {
					req, _ := http.NewRequest("GET", endpoint+"/ip", nil)
					for _, h := range v.headers {
						req.Header.Add("X-Forwarded-For", h)
					}
					resp, err := client.Do(req)
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						t.Fatal(err)
					}
					if resp.StatusCode != 200 || string(body) != v.want {
						t.Fatalf("%s: status=%d IP=%q want=%q", v.name, resp.StatusCode, body, v.want)
					}
				}
			})
		}
	}
}

func TestDirectHTTPProxyPolicy(t *testing.T) {
	app := New(NetHTTP(), WithTrustProxy(true), WithTrustedProxies("127.0.0.1"))
	app.Get("/", func(c *Ctx) error { return c.Text(c.IP()) })
	app.Compile()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "spoof, 192.0.2.1")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Body.String() != "192.0.2.1" {
		t.Fatal(fmt.Sprintf("IP=%q", rec.Body.String()))
	}
}

func TestTrustedProxiesCopiesOptionInput(t *testing.T) {
	ips := []string{"127.0.0.1"}
	opt := WithTrustedProxies(ips...)
	ips[0] = "198.51.100.1"
	app := New(NetHTTP(), WithTrustProxy(true))
	opt(app)
	app.Get("/", func(c *Ctx) error { return c.Text(c.IP()) })
	app.Compile()
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "192.0.2.1")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Body.String() != "192.0.2.1" {
		t.Fatalf("copied policy IP=%q", rec.Body.String())
	}
}
