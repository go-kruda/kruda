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
	os.Setenv("KRUDA_READ_BUF_SIZE", "4096")
	defer os.Unsetenv("KRUDA_READ_BUF_SIZE")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr, ok := newWingTransport(cfg, cfg.Logger).(*Transport)
	if !ok {
		t.Fatal("newWingTransport did not return Wing transport")
	}
	if tr.config.ReadBufSize != 4096 {
		t.Fatalf("ReadBufSize = %d, want 4096", tr.config.ReadBufSize)
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

func TestNewWingTransport_CPUAffinityEnv(t *testing.T) {
	os.Setenv("KRUDA_CPU_AFFINITY", "1")
	defer os.Unsetenv("KRUDA_CPU_AFFINITY")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr, ok := newWingTransport(cfg, cfg.Logger).(*Transport)
	if !ok {
		t.Fatal("newWingTransport did not return Wing transport")
	}
	if !tr.config.CPUAffinity {
		t.Fatal("CPUAffinity = false, want true")
	}
}

func TestNewWingTransport_AsyncEnv(t *testing.T) {
	os.Setenv("KRUDA_ASYNC", "1")
	defer os.Unsetenv("KRUDA_ASYNC")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_ASYNC=1")
	}
}

func TestNewWingTransport_PoolRoutes(t *testing.T) {
	os.Setenv("KRUDA_POOL_ROUTES", "GET /db,GET /queries")
	defer os.Unsetenv("KRUDA_POOL_ROUTES")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_POOL_ROUTES")
	}
}

func TestNewWingTransport_SpawnRoutes(t *testing.T) {
	os.Setenv("KRUDA_SPAWN_ROUTES", "POST /upload,POST /import")
	defer os.Unsetenv("KRUDA_SPAWN_ROUTES")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_SPAWN_ROUTES")
	}
}

func TestNewWingTransport_SpawnRoutes_WithoutPoolRoutes(t *testing.T) {
	// Test spawn routes when no pool routes are set (creates feathers map)
	os.Unsetenv("KRUDA_POOL_ROUTES")
	os.Setenv("KRUDA_SPAWN_ROUTES", "POST /api")
	defer os.Unsetenv("KRUDA_SPAWN_ROUTES")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil")
	}
}

func TestNewWingTransport_StaticEnv(t *testing.T) {
	os.Setenv("KRUDA_STATIC", "1")
	defer os.Unsetenv("KRUDA_STATIC")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_STATIC=1")
	}
}

func TestNewWingTransport_StaticEnv_WithPoolRoutes(t *testing.T) {
	os.Setenv("KRUDA_STATIC", "1")
	os.Setenv("KRUDA_POOL_ROUTES", "GET /db")
	defer os.Unsetenv("KRUDA_STATIC")
	defer os.Unsetenv("KRUDA_POOL_ROUTES")

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with KRUDA_STATIC + KRUDA_POOL_ROUTES")
	}
}

func TestNewWingTransport_AllEnv(t *testing.T) {
	os.Setenv("KRUDA_WORKERS", "2")
	os.Setenv("KRUDA_POOL_SIZE", "512")
	os.Setenv("KRUDA_ASYNC", "1")
	os.Setenv("KRUDA_POOL_ROUTES", "GET /db")
	os.Setenv("KRUDA_SPAWN_ROUTES", "POST /upload")
	defer func() {
		os.Unsetenv("KRUDA_WORKERS")
		os.Unsetenv("KRUDA_POOL_SIZE")
		os.Unsetenv("KRUDA_ASYNC")
		os.Unsetenv("KRUDA_POOL_ROUTES")
		os.Unsetenv("KRUDA_SPAWN_ROUTES")
	}()

	cfg := defaultConfig()
	cfg.Logger = discardLogger()
	tr := newWingTransport(cfg, cfg.Logger)
	if tr == nil {
		t.Fatal("newWingTransport returned nil with all env vars")
	}
}
