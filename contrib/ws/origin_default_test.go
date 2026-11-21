package ws

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/go-kruda/kruda/transport"
)

// handshakeStatus sends one upgrade request with the given Origin ("" for none)
// and returns the response status.
func handshakeStatus(t *testing.T, backend kruda.Option, cfg Config, origin func(host string) string, forwardedProto string, proxyOptions ...kruda.Option) int {
	t.Helper()
	app := kruda.New(append([]kruda.Option{backend}, proxyOptions...)...)
	upgrader := New(cfg)
	app.Get("/ws", func(c *kruda.Ctx) error {
		return upgrader.Upgrade(c, func(conn *Conn) { conn.Close(CloseNormalClosure, "") })
	}, kruda.Hijack)
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- app.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
		select {
		case <-done:
		case <-ctx.Done():
		}
	}()
	host := ln.Addr().String()
	req := "GET /ws HTTP/1.1\r\nHost: " + host + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + base64.StdEncoding.EncodeToString([]byte("test-key-1234567")) + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n"
	if o := origin(host); o != "" {
		req += "Origin: " + o + "\r\n"
	}
	if forwardedProto != "" {
		req += "X-Forwarded-Proto: " + forwardedProto + "\r\n"
	}
	// Wing rebinds the address itself, so an early dial can land on the
	// listener it closes. Retry until a response comes back.
	var lastErr error
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		code, err := tryHandshake(host, req)
		if err == nil {
			return code
		}
		lastErr = err
	}
	t.Fatalf("no handshake response: %v", lastErr)
	return 0
}

func tryHandshake(host, req string) (int, error) {
	conn, err := net.DialTimeout("tcp", host, time.Second)
	if err != nil {
		return 0, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(req + "\r\n")); err != nil {
		return 0, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

// TestUpgrade_DefaultOriginIsSameOrigin checks the default with no
// AllowedOrigins: a browser page on another site must not open an
// authenticated WebSocket, while same-origin pages and non-browser clients can.
func TestUpgrade_DefaultOriginIsSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cfg    Config
		origin func(host string) string
		want   int
	}{
		{"no origin", Config{}, func(string) string { return "" }, http.StatusSwitchingProtocols},
		{"same origin", Config{}, func(h string) string { return "http://" + h }, http.StatusSwitchingProtocols},
		{"same origin, other case", Config{}, func(h string) string { return "HTTP://" + strings.ToUpper(h) }, http.StatusSwitchingProtocols},
		{"different scheme", Config{}, func(h string) string { return "https://" + h }, http.StatusForbidden},
		{"different port", Config{}, func(h string) string { host, _, _ := net.SplitHostPort(h); return "http://" + host }, http.StatusForbidden},
		{"public origin behind TLS proxy", Config{AllowedOrigins: []string{"https://app.example.test"}}, func(string) string { return "https://app.example.test" }, http.StatusSwitchingProtocols},
		{"cross origin", Config{}, func(string) string { return "https://evil.example.com" }, http.StatusForbidden},
		{"null origin", Config{}, func(string) string { return "null" }, http.StatusForbidden},
		{"explicit wildcard", Config{AllowedOrigins: []string{"*"}}, func(string) string { return "https://evil.example.com" }, http.StatusSwitchingProtocols},
	} {
		for _, b := range []struct {
			name   string
			option kruda.Option
		}{{"nethttp", kruda.NetHTTP()}, {"wing", kruda.Wing()}} { // fasthttp has no WebSocket upgrade
			t.Run(b.name+"/"+tc.name, func(t *testing.T) {
				if got := handshakeStatus(t, b.option, tc.cfg, tc.origin, ""); got != tc.want {
					t.Fatalf("status = %d, want %d", got, tc.want)
				}
			})
		}
	}
}

func TestNetHTTPOriginUsesTLSAndDefaultPort(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	app.Get("/ws", func(c *kruda.Ctx) error {
		if err := New().checkOrigin(c); err != nil {
			return c.Status(http.StatusForbidden).Text("origin rejected")
		}
		return c.Text("origin accepted")
	})
	for _, tc := range []struct {
		origin string
		want   int
	}{
		{"https://example.test", 200},
		{"https://example.test:443", 200},
		{"http://example.test:443", 403},
		{"https://example.test:444", 403},
		{"https://example.test/#fragment", 403},
	} {
		req := httptest.NewRequest("GET", "https://example.test:443/ws", nil)
		req.Header.Set("Origin", tc.origin)
		req.Header.Set("X-Forwarded-Proto", "http")
		rec := httptest.NewRecorder()
		app.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.origin, rec.Code, tc.want)
		}
	}
}

func TestUpgradeRejectsBodyReadError(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	HandleFunc(app, "/ws", func(*Conn) { t.Fatal("upgrade reached the handler") })
	req := httptest.NewRequest("GET", "/ws", iotest.ErrReader(errors.New("body read error")))
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Sec-WebSocket-Version", "13")
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body)
	}
}

type proxyTestTransport struct {
	transport.Transport
	trustedIP string
}

func (p proxyTestTransport) ProxyConfig() (bool, []string) {
	return true, []string{p.trustedIP}
}

func (p proxyTestTransport) SetRoutePreset(method, path string, preset any) {
	if t, ok := p.Transport.(transport.PresetConfigurator); ok {
		t.SetRoutePreset(method, path, preset)
	}
}

func TestUpgradeHTTPSBehindTrustedProxy(t *testing.T) {
	if _, ok := any((*kruda.Ctx)(nil)).(interface{ IsTrustedProxy() bool }); !ok {
		t.Skip("trusted scheme needs a core with IsTrustedProxy")
	}
	for _, backend := range []struct {
		name         string
		option       kruda.Option
		newTransport func() transport.Transport
	}{
		{"nethttp", kruda.NetHTTP(), func() transport.Transport {
			return transport.NewNetHTTP(transport.NetHTTPConfig{})
		}},
		{"wing", kruda.Wing(), func() transport.Transport {
			return kruda.NewWingTransport(kruda.WingConfig{Workers: 1})
		}},
	} {
		t.Run(backend.name, func(t *testing.T) {
			if backend.name == "wing" && runtime.GOOS == "windows" {
				t.Skip("Wing is unavailable on Windows")
			}
			for _, tc := range []struct {
				name      string
				trustedIP string
				want      int
			}{
				{"trusted", "127.0.0.1", http.StatusSwitchingProtocols},
				{"untrusted", "192.0.2.1", http.StatusForbidden},
			} {
				t.Run(tc.name, func(t *testing.T) {
					proxy := proxyTestTransport{Transport: backend.newTransport(), trustedIP: tc.trustedIP}
					got := handshakeStatus(t, backend.option, Config{}, func(host string) string {
						return "https://" + host
					}, "https", kruda.WithTransport(proxy))
					if got != tc.want {
						t.Fatalf("status = %d, want %d", got, tc.want)
					}
				})
			}
		})
	}
}
