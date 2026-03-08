//go:build linux || darwin

package kruda

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/go-kruda/kruda/transport"
	"github.com/go-kruda/kruda/transport/wing"
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
	wcfg := wing.Config{
		Workers:         workers,
		HandlerPoolSize: poolSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
	}
	if os.Getenv("KRUDA_ASYNC") == "1" {
		wcfg.DefaultFeather = wing.Feather{Dispatch: wing.Pool}
	}
	// Per-route dispatch from env: KRUDA_POOL_ROUTES="GET /db,GET /queries,GET /fortunes,GET /updates"
	if routes := os.Getenv("KRUDA_POOL_ROUTES"); routes != "" {
		wcfg.Feathers = make(map[string]wing.Feather)
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = wing.Feather{Dispatch: wing.Pool}
		}
	}
	if routes := os.Getenv("KRUDA_SPAWN_ROUTES"); routes != "" {
		if wcfg.Feathers == nil {
			wcfg.Feathers = make(map[string]wing.Feather)
		}
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = wing.Feather{Dispatch: wing.Spawn}
		}
	}
	// Static responses: bypass handler entirely for maximum throughput.
	if os.Getenv("KRUDA_STATIC") == "1" {
		if wcfg.Feathers == nil {
			wcfg.Feathers = make(map[string]wing.Feather)
		}
		wcfg.Feathers["GET /"] = wing.Bolt.With(wing.Static(
			transport.GetStaticResponseString(200, "text/plain; charset=utf-8", "Hello, World!")))
		wcfg.Feathers["GET /json"] = wing.Bolt.With(wing.Static(
			transport.GetStaticResponseString(200, "application/json; charset=utf-8", `{"message":"Hello, World!"}`)))
	}
	return wing.New(wcfg)
}
