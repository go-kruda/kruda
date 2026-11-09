package ratelimit

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

func TestClientIPAcrossBackends(t *testing.T) {
	for _, tc := range []struct {
		name   string
		option kruda.Option
	}{
		{"nethttp", kruda.NetHTTP()},
		{"fasthttp", kruda.FastHTTP()},
		{"wing", kruda.Wing()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			app := kruda.New(tc.option)
			store := NewMemoryStore(time.Minute, 2*time.Minute)
			defer store.Stop()
			app.Use(ForRoute("/test", 2, time.Minute, Config{Store: store}))
			app.Get("/test", func(c *kruda.Ctx) error { return c.Text(c.IP()) })
			app.Get("/ready", func(c *kruda.Ctx) error { return c.Text("ready") })
			done := make(chan error, 1)
			go func() { done <- app.Serve(ln) }()
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := app.Shutdown(ctx); err != nil {
					t.Error(err)
				}
				select {
				case <-done:
				case <-ctx.Done():
					t.Error(ctx.Err())
				}
			}()
			client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 3 * time.Second}
			deadline := time.Now().Add(3 * time.Second)
			for {
				resp, err := client.Get(fmt.Sprintf("http://%s/ready", ln.Addr()))
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatalf("server did not start: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			for i := 0; i < 3; i++ {
				resp, err := client.Get(fmt.Sprintf("http://%s/test", ln.Addr()))
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if i < 2 && string(body) != "127.0.0.1" {
					t.Fatalf("request %d: IP = %q", i+1, body)
				}
				want := http.StatusOK
				if i == 2 {
					want = http.StatusTooManyRequests
				}
				if resp.StatusCode != want {
					t.Fatalf("request %d: status %d, want %d", i+1, resp.StatusCode, want)
				}
			}
		})
	}
}

func TestUntrustedForwardedForDoesNotChangeKey(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	app := kruda.New(kruda.NetHTTP(), kruda.WithTrustProxy(true))
	store := NewMemoryStore(time.Minute, 2*time.Minute)
	defer store.Stop()
	app.Use(New(Config{Max: 2, Window: time.Minute, Store: store}))
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	done := make(chan error, 1)
	go func() { done <- app.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Error(ctx.Err())
		}
	}()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 3 * time.Second}
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/", ln.Addr()), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := http.StatusOK
		if i == 2 {
			want = http.StatusTooManyRequests
		}
		if resp.StatusCode != want {
			t.Fatalf("request %d: status %d, want %d", i+1, resp.StatusCode, want)
		}
	}
}

func TestTrustedForwardingAcrossBackends(t *testing.T) {
	for _, tc := range []struct {
		name   string
		option kruda.Option
	}{{"nethttp", kruda.NetHTTP()}, {"fasthttp", kruda.FastHTTP()}, {"wing", kruda.Wing()}} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			app := kruda.New(tc.option, kruda.WithTrustProxy(true))
			trusted := newProxyPolicy([]string{"127.0.0.1", "10.0.0.2"})
			app.Get("/ip", func(c *kruda.Ctx) error { return c.Text(extractIP(c, trusted)) })
			app.Get("/ready", func(c *kruda.Ctx) error { return c.Text("ready") })
			done := make(chan error, 1)
			go func() { done <- app.Serve(ln) }()
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := app.Shutdown(ctx); err != nil {
					t.Error(err)
				}
				select {
				case <-done:
				case <-ctx.Done():
					t.Error(ctx.Err())
				}
			}()
			client := &http.Client{Timeout: 3 * time.Second}
			defer client.CloseIdleConnections()
			deadline := time.Now().Add(3 * time.Second)
			for {
				resp, err := client.Get(fmt.Sprintf("http://%s/ready", ln.Addr()))
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == 200 {
						break
					}
				}
				if time.Now().After(deadline) {
					t.Fatalf("server did not start: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
			for _, v := range []struct {
				name     string
				xff, xri []string
				want     string
			}{
				{"rightmost untrusted", []string{"198.51.100.9, 203.0.113.7, 10.0.0.2"}, nil, "203.0.113.7"},
				{"mapped ipv4", []string{"::ffff:192.0.2.7"}, nil, "192.0.2.7"},
				{"canonical ipv6", []string{"2001:0db8:0:0:0:0:0:1"}, nil, "2001:db8::1"},
				{"all hops trusted", []string{"10.0.0.2, 127.0.0.1"}, nil, "10.0.0.2"},
				{"invalid trusted-side hop", []string{"192.0.2.5, invalid"}, nil, "127.0.0.1"},
				{"zone rejected", []string{"fe80::1%en0"}, nil, "127.0.0.1"},
				{"real ip", nil, []string{"192.0.2.8"}, "192.0.2.8"},
				{"invalid xff does not use real ip", []string{"not-an-ip"}, []string{"192.0.2.8"}, "127.0.0.1"},
				{"empty xff does not use real ip", []string{""}, []string{"192.0.2.8"}, "127.0.0.1"},
				{"duplicate xff", []string{"198.51.100.9", "203.0.113.7"}, nil, "127.0.0.1"},
				{"empty then duplicate xff", []string{"", "203.0.113.7"}, nil, "127.0.0.1"},
				{"duplicate real ip", nil, []string{"198.51.100.9", "203.0.113.7"}, "127.0.0.1"},
				{"no headers after duplicate", nil, nil, "127.0.0.1"},
			} {
				t.Run(v.name, func(t *testing.T) {
					req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/ip", ln.Addr()), nil)
					if err != nil {
						t.Fatal(err)
					}
					for _, h := range v.xff {
						req.Header.Add("X-Forwarded-For", h)
					}
					for _, h := range v.xri {
						req.Header.Add("X-Real-IP", h)
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
					if string(body) != v.want {
						t.Fatalf("IP=%q want=%q", body, v.want)
					}
				})
			}
		})
	}
}

func TestTrustedProxyMappedAddressIsNormalized(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	app := kruda.New(kruda.NetHTTP())
	store := NewMemoryStore(time.Minute, 2*time.Minute)
	defer store.Stop()
	app.Use(New(Config{Max: 2, Window: time.Minute, Store: store, TrustedProxies: []string{"::ffff:127.0.0.1"}}))
	app.Get("/", func(c *kruda.Ctx) error { return c.Text("ok") })
	done := make(chan error, 1)
	go func() { done <- app.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
		case <-ctx.Done():
			t.Error(ctx.Err())
		}
	}()
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 3 * time.Second}
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/", ln.Addr()), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", i+1))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d: status %d", i+1, resp.StatusCode)
		}
	}
}
