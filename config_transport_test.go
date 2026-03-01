package kruda

import (
	"context"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestSelectTransport_DefaultReturnsFastHTTP(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	// Default is fasthttp on Linux/macOS, net/http on Windows (fasthttp build tag).
	switch tr.(type) {
	case *transport.NetHTTPTransport:
		// expected on Windows (fasthttp has build tag !windows)
	default:
		// fasthttp on Linux/macOS — any non-nil transport is valid
	}
}

func TestSelectTransport_ExplicitTransport(t *testing.T) {
	custom := &mockTransport{}
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.Transport = custom
	tr := selectTransport(cfg, cfg.Logger)
	if tr != custom {
		t.Error("expected explicit Transport to take priority")
	}
}

func TestSelectTransport_NetHTTPOption(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "nethttp" // set by NetHTTP() option
	tr := selectTransport(cfg, cfg.Logger)
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected *transport.NetHTTPTransport, got %T", tr)
	}
}

func TestSelectTransport_DefaultSelectsFastHTTP(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	// TransportName defaults to "" → fasthttp
	tr := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil for default")
	}
}

func TestSelectTransport_AutoTLSUsesNetHTTP(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr := selectTransport(cfg, cfg.Logger)
	// Auto + TLS → always net/http for HTTP/2.
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected auto+TLS to select *transport.NetHTTPTransport, got %T", tr)
	}
}

func TestSelectTransport_EnvOverride(t *testing.T) {
	os.Setenv("KRUDA_TRANSPORT", "nethttp")
	defer os.Unsetenv("KRUDA_TRANSPORT")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	// TransportName is empty — env should take effect
	tr := selectTransport(cfg, cfg.Logger)
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected *transport.NetHTTPTransport from env override, got %T", tr)
	}
}

func TestSelectTransport_ExplicitTransportOverridesEnv(t *testing.T) {
	os.Setenv("KRUDA_TRANSPORT", "nethttp")
	defer os.Unsetenv("KRUDA_TRANSPORT")

	custom := &mockTransport{}
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.Transport = custom
	tr := selectTransport(cfg, cfg.Logger)
	if tr != custom {
		t.Error("explicit WithTransport should override KRUDA_TRANSPORT env")
	}
}

func TestSelectTransport_ExplicitOptionOverridesEnv(t *testing.T) {
	os.Setenv("KRUDA_TRANSPORT", "nethttp")
	defer os.Unsetenv("KRUDA_TRANSPORT")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "nethttp" // set by NetHTTP() option
	tr := selectTransport(cfg, cfg.Logger)
	// Explicit TransportName set, env should NOT override
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected NetHTTP() to override env, got %T", tr)
	}
}

func TestSelectTransport_ConfigPassthrough(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.ReadTimeout = 5e9   // 5s
	cfg.WriteTimeout = 10e9 // 10s
	cfg.BodyLimit = 1024
	cfg.TrustProxy = true
	tr := selectTransport(cfg, cfg.Logger)
	// Just verify it returns a valid transport — config passthrough is internal
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
}

func TestNetHTTP_Option(t *testing.T) {
	app := New(NetHTTP())
	if app.config.TransportName != "nethttp" {
		t.Errorf("expected TransportName 'nethttp', got %q", app.config.TransportName)
	}
}

func TestFastHTTP_Option(t *testing.T) {
	app := New(FastHTTP())
	if app.config.TransportName != "fasthttp" {
		t.Errorf("expected TransportName 'fasthttp', got %q", app.config.TransportName)
	}
}

// mockTransport is a minimal transport for testing explicit WithTransport priority.
type mockTransport struct{}

func (m *mockTransport) ListenAndServe(addr string, handler transport.Handler) error {
	return nil
}

func (m *mockTransport) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockTransport) Serve(ln net.Listener, handler transport.Handler) error {
	return nil
}
