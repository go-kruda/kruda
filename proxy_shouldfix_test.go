//go:build linux || darwin

package kruda

import (
	"context"
	"github.com/go-kruda/kruda/transport"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithTransportInheritsProxyPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		tr   transport.Transport
	}{
		{"nethttp", transport.NewNetHTTP(transport.NetHTTPConfig{TrustProxy: true, TrustedProxies: []string{"127.0.0.1"}})},
		{"fasthttp", transport.NewFastHTTP(transport.FastHTTPConfig{TrustProxy: true, TrustedProxies: []string{"127.0.0.1"}})},
		{"wing", NewWingTransport(WingConfig{Workers: 1, TrustProxy: true, TrustedProxies: []string{"127.0.0.1"}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := New(WithTransport(tc.tr))
			app.Get("/ready", func(c *Ctx) error { return c.Text("ready") })
			app.Get("/ip", func(c *Ctx) error {
				remote := c.Request().RemoteAddr()
				if host, _, err := net.SplitHostPort(remote); err == nil {
					remote = host
				}
				return c.Text(c.IP() + "|" + remote)
			})
			base := "http://" + startStaticStreamApp(t, app)
			client := &http.Client{}
			defer client.CloseIdleConnections()
			req, _ := http.NewRequest("GET", base+"/ip", nil)
			req.Header.Set("X-Forwarded-For", "192.0.2.1")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			want := "192.0.2.1|192.0.2.1"
			if tc.name == "fasthttp" {
				want = "192.0.2.1|127.0.0.1"
			}
			if err != nil || string(body) != want {
				t.Fatalf("proxy identities=%q err=%v", body, err)
			}
		})
	}
}

func TestExplicitAppProxyPolicyOverridesTransport(t *testing.T) {
	for _, transportFirst := range []bool{false, true} {
		tr := WithTransport(transport.NewNetHTTP(transport.NetHTTPConfig{TrustProxy: true, TrustedProxies: []string{"127.0.0.1"}}))
		opts := []Option{WithTrustProxy(false), WithTrustedProxies("198.51.100.1")}
		if transportFirst {
			opts = append([]Option{tr}, opts...)
		} else {
			opts = append(opts, tr)
		}
		app := New(opts...)
		if app.config.TrustProxy || len(app.config.TrustedProxies) != 1 || app.config.TrustedProxies[0] != "198.51.100.1" {
			t.Fatalf("explicit policy changed: %+v", app.config)
		}
	}
}

type directPeerRequest struct{ addr string }

func (r directPeerRequest) Method() string                               { return "GET" }
func (r directPeerRequest) Path() string                                 { return "/ip" }
func (r directPeerRequest) Header(string) string                         { return "192.0.2.99" }
func (r directPeerRequest) Body() ([]byte, error)                        { return nil, nil }
func (r directPeerRequest) QueryParam(string) string                     { return "" }
func (r directPeerRequest) RemoteAddr() string                           { return r.addr }
func (r directPeerRequest) Cookie(string) string                         { return "" }
func (r directPeerRequest) RawRequest() any                              { return nil }
func (r directPeerRequest) Context() context.Context                     { return context.Background() }
func (r directPeerRequest) MultipartForm(int64) (*multipart.Form, error) { return nil, nil }
func TestCustomRequestKeepsDirectPeerWithoutProxyHooks(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{{"198.51.100.17:1234", "198.51.100.17"}, {"[::ffff:198.51.100.17]:1234", "198.51.100.17"}} {
		app := New(WithTrustProxy(true), WithTrustedProxies("198.51.100.17"))
		app.Get("/ip", func(c *Ctx) error { return c.Text(c.IP()) })
		rec := httptest.NewRecorder()
		app.ServeKruda(&fastNetHTTPResponseWriter{w: rec, statusCode: 200}, directPeerRequest{tc.addr})
		if rec.Body.String() != tc.want {
			t.Fatalf("ip=%q want=%q", rec.Body.String(), tc.want)
		}
	}
}

func TestTransportProxyDefaultsRespectEnvironmentAndEmptyAllowlist(t *testing.T) {
	tr := transport.NewNetHTTP(transport.NetHTTPConfig{TrustProxy: true, TrustedProxies: []string{"127.0.0.1"}})
	t.Setenv("TEST_PROXY_TRUST_PROXY", "false")
	app := New(WithEnvPrefix("TEST_PROXY"), WithTransport(tr))
	if app.config.TrustProxy {
		t.Fatal("transport replaced explicit environment setting")
	}
	app = New(WithTrustedProxies(), WithTransport(tr))
	if !app.config.TrustProxy || len(app.config.TrustedProxies) != 0 {
		t.Fatal("explicit empty allowlist was replaced")
	}
	_, ips := tr.ProxyConfig()
	ips[0] = "192.0.2.99"
	_, again := tr.ProxyConfig()
	if again[0] != "127.0.0.1" {
		t.Fatal("ProxyConfig leaked its allowlist")
	}
}
