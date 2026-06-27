package kruda

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"time"
)

// --- WithIdleTimeout / WithLogger / JSON codec / WithTrustProxy ---

func TestWithIdleTimeout(t *testing.T) {
	app := &App{config: defaultConfig()}
	WithIdleTimeout(45 * time.Second)(app)
	if app.config.IdleTimeout != 45*time.Second {
		t.Errorf("IdleTimeout = %v", app.config.IdleTimeout)
	}
}

func TestWithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &App{config: defaultConfig()}
	WithLogger(logger)(app)
	if app.config.Logger != logger {
		t.Error("Logger not set")
	}
}

func TestWithJSONEncoder(t *testing.T) {
	called := false
	enc := func(v any) ([]byte, error) { called = true; return nil, nil }
	app := &App{config: defaultConfig()}
	WithJSONEncoder(enc)(app)
	app.config.JSONEncoder(nil)
	if !called {
		t.Error("JSONEncoder not set")
	}
}

func TestWithJSONStreamEncoder(t *testing.T) {
	called := false
	enc := func(buf *bytes.Buffer, v any) error { called = true; return nil }
	app := &App{config: defaultConfig()}
	WithJSONStreamEncoder(enc)(app)
	app.config.JSONStreamEncoder(&bytes.Buffer{}, nil)
	if !called {
		t.Error("JSONStreamEncoder not set")
	}
}

func TestWithJSONDecoder(t *testing.T) {
	called := false
	dec := func(data []byte, v any) error { called = true; return nil }
	app := &App{config: defaultConfig()}
	WithJSONDecoder(dec)(app)
	app.config.JSONDecoder(nil, nil)
	if !called {
		t.Error("JSONDecoder not set")
	}
}

func TestWithTrustProxy(t *testing.T) {
	app := &App{config: defaultConfig()}
	WithTrustProxy(true)(app)
	if !app.config.TrustProxy {
		t.Error("TrustProxy not set")
	}
}

// --- Transport selection ---

func TestWithTransport_SetsTransport(t *testing.T) {
	custom := &mockTransport{}
	app := &App{config: defaultConfig()}
	WithTransport(custom)(app)
	if app.config.Transport != custom {
		t.Error("WithTransport did not set Transport")
	}
}

func TestWingOption_SetsTransportName(t *testing.T) {
	app := &App{config: defaultConfig()}
	Wing()(app)
	if app.config.TransportName != "wing" {
		t.Errorf("Wing() set TransportName = %q, want %q", app.config.TransportName, "wing")
	}
}

func TestSelectTransport_FastHTTPWithTLS(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "fasthttp"
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	if name != "nethttp" {
		t.Errorf("fasthttp+TLS should fallback to nethttp, got %q", name)
	}
}

func TestSelectTransport_WingWithTLS(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.TransportName = "wing"
	cfg.TLSCertFile = "cert.pem"
	cfg.TLSKeyFile = "key.pem"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("selectTransport returned nil")
	}
	if name != "nethttp" {
		t.Errorf("wing+TLS should fallback to nethttp, got %q", name)
	}
}

func TestSelectTransport_ExplicitTransportNameReturned(t *testing.T) {
	custom := &mockTransport{}
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.Transport = custom
	cfg.TransportName = "custom-name"
	tr, name := selectTransport(cfg, cfg.Logger)
	if tr != custom {
		t.Error("expected explicit transport")
	}
	if name != "custom-name" {
		t.Errorf("expected TransportName %q, got %q", "custom-name", name)
	}
}

// --- parseSize error branches ---

func TestParseSize_GBError(t *testing.T) {
	_, err := parseSize("abcGB")
	if err == nil {
		t.Error("expected error for invalid GB value")
	}
}

func TestParseSize_KBError(t *testing.T) {
	_, err := parseSize("xyzKB")
	if err == nil {
		t.Error("expected error for invalid KB value")
	}
}

func TestParseSize_MBError(t *testing.T) {
	_, err := parseSize("badMB")
	if err == nil {
		t.Error("expected error for invalid MB value")
	}
}

// --- Wing preset route options ---

func TestPresetRouteOptions(t *testing.T) {
	// These just create RouteOption funcs — calling them should not panic.
	opts := []RouteOption{
		Plaintext,
		JSON,
		DB,
		Render,
	}
	for i, opt := range opts {
		if opt == nil {
			t.Errorf("Wing preset option %d is nil", i)
		}
		// Apply to a routeConfig to exercise the code
		var rc routeConfig
		opt.applyRoute(&rc)
		if rc.preset == nil {
			t.Errorf("Wing preset option %d did not set preset", i)
		}
	}
}

// --- WithViews ---

func TestWithViews(t *testing.T) {
	engine := &mockViewEngine{}
	app := &App{config: defaultConfig()}
	WithViews(engine)(app)
	if app.config.Views != engine {
		t.Error("Views not set")
	}
}
