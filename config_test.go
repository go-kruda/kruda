package kruda

import (
	"os"
	"testing"
	"time"
)

func TestDefaultConfigShutdownTimeout(t *testing.T) {
	cfg := defaultConfig()
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected ShutdownTimeout 10s, got %v", cfg.ShutdownTimeout)
	}
}

func TestWithShutdownTimeout(t *testing.T) {
	app := &App{config: defaultConfig()}
	opt := WithShutdownTimeout(30 * time.Second)
	opt(app)
	if app.config.ShutdownTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", app.config.ShutdownTimeout)
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"4MB", 4 * 1024 * 1024},
		{"4mb", 4 * 1024 * 1024},
		{"4Mb", 4 * 1024 * 1024},
		{"10KB", 10 * 1024},
		{"10kb", 10 * 1024},
		{"1GB", 1 * 1024 * 1024 * 1024},
		{"1gb", 1 * 1024 * 1024 * 1024},
		{"4096", 4096},
		{" 2MB ", 2 * 1024 * 1024},
	}
	for _, tt := range tests {
		got, err := parseSize(tt.input)
		if err != nil {
			t.Errorf("parseSize(%q) error: %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestParseSizeError(t *testing.T) {
	_, err := parseSize("abc")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestApplyEnvConfig(t *testing.T) {
	os.Setenv("TEST_READ_TIMEOUT", "5s")
	os.Setenv("TEST_WRITE_TIMEOUT", "15s")
	os.Setenv("TEST_IDLE_TIMEOUT", "60s")
	os.Setenv("TEST_BODY_LIMIT", "8MB")
	os.Setenv("TEST_SHUTDOWN_TIMEOUT", "20s")
	defer func() {
		os.Unsetenv("TEST_READ_TIMEOUT")
		os.Unsetenv("TEST_WRITE_TIMEOUT")
		os.Unsetenv("TEST_IDLE_TIMEOUT")
		os.Unsetenv("TEST_BODY_LIMIT")
		os.Unsetenv("TEST_SHUTDOWN_TIMEOUT")
	}()

	cfg := defaultConfig()
	applyEnvConfig("TEST", &cfg)

	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v, want 15s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", cfg.IdleTimeout)
	}
	if cfg.BodyLimit != 8*1024*1024 {
		t.Errorf("BodyLimit = %d, want %d", cfg.BodyLimit, 8*1024*1024)
	}
	if cfg.ShutdownTimeout != 20*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 20s", cfg.ShutdownTimeout)
	}
}

func TestApplyEnvConfigMissingVars(t *testing.T) {
	cfg := defaultConfig()
	original := cfg
	applyEnvConfig("NONEXISTENT", &cfg)

	if cfg.ReadTimeout != original.ReadTimeout {
		t.Error("ReadTimeout changed with missing env var")
	}
	if cfg.BodyLimit != original.BodyLimit {
		t.Error("BodyLimit changed with missing env var")
	}
	if cfg.ShutdownTimeout != original.ShutdownTimeout {
		t.Error("ShutdownTimeout changed with missing env var")
	}
}

func TestApplyEnvConfigInvalidValues(t *testing.T) {
	os.Setenv("BAD_READ_TIMEOUT", "notaduration")
	os.Setenv("BAD_BODY_LIMIT", "notasize")
	defer func() {
		os.Unsetenv("BAD_READ_TIMEOUT")
		os.Unsetenv("BAD_BODY_LIMIT")
	}()

	cfg := defaultConfig()
	original := cfg
	applyEnvConfig("BAD", &cfg)

	if cfg.ReadTimeout != original.ReadTimeout {
		t.Error("ReadTimeout changed with invalid env var")
	}
	if cfg.BodyLimit != original.BodyLimit {
		t.Error("BodyLimit changed with invalid env var")
	}
}

func TestWithEnvPrefix(t *testing.T) {
	os.Setenv("MYAPP_READ_TIMEOUT", "3s")
	defer os.Unsetenv("MYAPP_READ_TIMEOUT")

	app := &App{config: defaultConfig()}
	opt := WithEnvPrefix("MYAPP")
	opt(app)

	if app.config.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want 3s", app.config.ReadTimeout)
	}
}
