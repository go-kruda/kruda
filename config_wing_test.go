//go:build linux || darwin

package kruda

import (
	"os"
	"testing"
	"time"
)

func TestNewWingTransport_Default(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil")
	}
}

func TestNewWingTransport_WithTimeouts(t *testing.T) {
	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 10 * time.Second
	cfg.IdleTimeout = 60 * time.Second
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with custom timeouts")
	}
}

func TestNewWingTransport_WorkersEnv(t *testing.T) {
	os.Setenv("KRUDA_WORKERS", "4")
	defer os.Unsetenv("KRUDA_WORKERS")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_WORKERS")
	}
}

func TestNewWingTransport_WorkersEnv_Invalid(t *testing.T) {
	os.Setenv("KRUDA_WORKERS", "invalid")
	defer os.Unsetenv("KRUDA_WORKERS")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with invalid KRUDA_WORKERS")
	}
}

func TestNewWingTransport_WorkersEnv_Zero(t *testing.T) {
	os.Setenv("KRUDA_WORKERS", "0")
	defer os.Unsetenv("KRUDA_WORKERS")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with zero workers")
	}
}

func TestNewWingTransport_PoolSizeEnv(t *testing.T) {
	os.Setenv("KRUDA_POOL_SIZE", "1024")
	defer os.Unsetenv("KRUDA_POOL_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_POOL_SIZE")
	}
}

func TestNewWingTransport_PoolSizeEnv_Invalid(t *testing.T) {
	os.Setenv("KRUDA_POOL_SIZE", "abc")
	defer os.Unsetenv("KRUDA_POOL_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with invalid pool size")
	}
}

func TestNewWingTransport_ReadBufSizeEnv(t *testing.T) {
	// 16384 >= default HeaderLimit (8192), so the size constraint is satisfied.
	os.Setenv("KRUDA_READ_BUF_SIZE", "16384")
	defer os.Unsetenv("KRUDA_READ_BUF_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr, ok := newWingTransport(cfg, cfg.Logger).(*Transport)
	if !ok {
		t.Fatal("newWingTransport did not return Wing transport")
	}
	if tr.config.ReadBufSize != 16384 {
		t.Fatalf("ReadBufSize = %d, want 16384", tr.config.ReadBufSize)
	}
}

func TestNewWingTransport_ReadBufSizeEnv_Invalid(t *testing.T) {
	os.Setenv("KRUDA_READ_BUF_SIZE", "abc")
	defer os.Unsetenv("KRUDA_READ_BUF_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr, ok := newWingTransport(cfg, cfg.Logger).(*Transport)
	if !ok {
		t.Fatal("newWingTransport did not return Wing transport")
	}
	if tr.config.ReadBufSize != 8192 {
		t.Fatalf("ReadBufSize = %d, want default 8192", tr.config.ReadBufSize)
	}
}

func TestNewWingTransport_ReadBufSizeEnv_Zero(t *testing.T) {
	os.Setenv("KRUDA_READ_BUF_SIZE", "0")
	defer os.Unsetenv("KRUDA_READ_BUF_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr, ok := newWingTransport(cfg, cfg.Logger).(*Transport)
	if !ok {
		t.Fatal("newWingTransport did not return Wing transport")
	}
	if tr.config.ReadBufSize != 8192 {
		t.Fatalf("ReadBufSize = %d, want default 8192", tr.config.ReadBufSize)
	}
}

func TestNewWingTransport_AllEnv(t *testing.T) {
	os.Setenv("KRUDA_WORKERS", "2")
	os.Setenv("KRUDA_POOL_SIZE", "512")
	defer func() {
		os.Unsetenv("KRUDA_WORKERS")
		os.Unsetenv("KRUDA_POOL_SIZE")
	}()

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with all env vars")
	}
}
