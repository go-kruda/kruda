package kruda

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func TestSelectTransport_DefaultReturnsTransport(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	// Auto-selection: fasthttp on Linux/macOS, net/http on Windows.
	switch tr.(type) {
	case *transport.FastHTTPTransport:
		// expected on Linux/macOS
	case *transport.NetHTTPTransport:
		// expected on Windows
	default:
		t.Errorf("expected *FastHTTPTransport or *NetHTTPTransport, got %T", tr)
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

func TestSelectTransport_WithTransportNameNetHTTP(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "nethttp"
	tr := selectTransport(cfg, cfg.Logger)
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected *transport.NetHTTPTransport, got %T", tr)
	}
}

func TestSelectTransport_NetpollExplicit(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "netpoll"
	tr := selectTransport(cfg, cfg.Logger)
	// On Linux/macOS, NewNetpoll succeeds → *NetpollTransport.
	// On Windows, NewNetpoll returns error → falls back to *NetHTTPTransport.
	switch tr.(type) {
	case *transport.NetpollTransport:
		// expected on Linux/macOS
	case *transport.NetHTTPTransport:
		// expected on Windows (netpoll not supported)
	default:
		t.Errorf("expected *NetpollTransport or *NetHTTPTransport, got %T", tr)
	}
}

func TestSelectTransport_NetpollTLSFallback(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "netpoll"
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr := selectTransport(cfg, cfg.Logger)
	// TLS configured → must fall back to net/http regardless of OS.
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected TLS fallback to *transport.NetHTTPTransport, got %T", tr)
	}
}

func TestSelectTransport_AutoSelectsNetpollOrNetHTTP(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	// TransportName defaults to "auto"
	tr := selectTransport(cfg, cfg.Logger)
	switch tr.(type) {
	case *transport.FastHTTPTransport:
		// expected on Linux/macOS
	case *transport.NetHTTPTransport:
		// expected on Windows
	default:
		t.Errorf("expected *FastHTTPTransport or *NetHTTPTransport, got %T", tr)
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
	// TransportName is empty/"auto" — env should take effect
	tr := selectTransport(cfg, cfg.Logger)
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected *transport.NetHTTPTransport from env override, got %T", tr)
	}
}

func TestSelectTransport_ExplicitTransportOverridesEnv(t *testing.T) {
	os.Setenv("KRUDA_TRANSPORT", "netpoll")
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

func TestSelectTransport_TransportNameOverridesEnv(t *testing.T) {
	os.Setenv("KRUDA_TRANSPORT", "netpoll")
	defer os.Unsetenv("KRUDA_TRANSPORT")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "nethttp"
	tr := selectTransport(cfg, cfg.Logger)
	// TransportName is "nethttp" (not "auto"), so env should NOT override
	if _, ok := tr.(*transport.NetHTTPTransport); !ok {
		t.Errorf("expected WithTransportName to override env, got %T", tr)
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

func TestWithTransportName_Option(t *testing.T) {
	app := New(WithTransportName("nethttp"))
	if app.config.TransportName != "nethttp" {
		t.Errorf("expected TransportName 'nethttp', got %q", app.config.TransportName)
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
