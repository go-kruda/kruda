//go:build linux || darwin

package kruda

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/go-kruda/kruda/transport"
)

func newWingTransport(cfg Config, logger *slog.Logger) transport.Transport {
	workers := 0
	if v := os.Getenv("KRUDA_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}
	poolSize := 0
	if v := os.Getenv("KRUDA_POOL_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			poolSize = n
		}
	}
	readBufSize := 0
	if v := os.Getenv("KRUDA_READ_BUF_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			readBufSize = n
		}
	}
	wcfg := WingConfig{
		Workers:         workers,
		HandlerPoolSize: poolSize,
		ReadBufSize:     readBufSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
	}
	if os.Getenv("KRUDA_ASYNC") == "1" {
		wcfg.DefaultPreset = Preset{Dispatch: Pool}
	}
	// Per-route dispatch from env: KRUDA_POOL_ROUTES="GET /db,GET /queries,GET /fortunes,GET /updates"
	if routes := os.Getenv("KRUDA_POOL_ROUTES"); routes != "" {
		wcfg.Presets = make(map[string]Preset)
		for _, r := range strings.Split(routes, ",") {
			wcfg.Presets[strings.TrimSpace(r)] = Preset{Dispatch: Pool}
		}
	}
	if routes := os.Getenv("KRUDA_SPAWN_ROUTES"); routes != "" {
		if wcfg.Presets == nil {
			wcfg.Presets = make(map[string]Preset)
		}
		for _, r := range strings.Split(routes, ",") {
			wcfg.Presets[strings.TrimSpace(r)] = Preset{Dispatch: Spawn}
		}
	}
	// Static responses: bypass handler entirely for maximum throughput.
	if os.Getenv("KRUDA_STATIC") == "1" {
		if wcfg.Presets == nil {
			wcfg.Presets = make(map[string]Preset)
		}
		wcfg.Presets["GET /"] = Bolt.With(Static(
			transport.GetStaticResponseString(200, "text/plain; charset=utf-8", "Hello, World!")))
		wcfg.Presets["GET /json"] = Bolt.With(Static(
			transport.GetStaticResponseString(200, "application/json; charset=utf-8", `{"message":"Hello, World!"}`)))
	}
	return NewWingTransport(wcfg)
}
