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
	var epollIdleSpins *int
	if v := os.Getenv("KRUDA_WING_EPOLL_IDLE_SPINS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			epollIdleSpins = &n
		}
	}
	wcfg := WingConfig{
		Workers:         workers,
		HandlerPoolSize: poolSize,
		ReadBufSize:     readBufSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
		EpollIdleSpins:  epollIdleSpins,
	}
	if os.Getenv("KRUDA_ASYNC") == "1" {
		wcfg.DefaultFeather = Feather{Dispatch: Pool}
	}
	// Per-route dispatch from env: KRUDA_POOL_ROUTES="GET /db,GET /queries,GET /fortunes,GET /updates"
	if routes := os.Getenv("KRUDA_POOL_ROUTES"); routes != "" {
		wcfg.Feathers = make(map[string]Feather)
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = Feather{Dispatch: Pool}
		}
	}
	if routes := os.Getenv("KRUDA_SPAWN_ROUTES"); routes != "" {
		if wcfg.Feathers == nil {
			wcfg.Feathers = make(map[string]Feather)
		}
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = Feather{Dispatch: Spawn}
		}
	}
	// Static responses: bypass handler entirely for maximum throughput.
	if os.Getenv("KRUDA_STATIC") == "1" {
		if wcfg.Feathers == nil {
			wcfg.Feathers = make(map[string]Feather)
		}
		wcfg.Feathers["GET /"] = Bolt.With(Static(
			transport.GetStaticResponseString(200, "text/plain; charset=utf-8", "Hello, World!")))
		wcfg.Feathers["GET /json"] = Bolt.With(Static(
			transport.GetStaticResponseString(200, "application/json; charset=utf-8", `{"message":"Hello, World!"}`)))
	}
	return NewWingTransport(wcfg)
}
