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
	if v := os.Getenv("WING_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workers = n
		}
	}
	poolSize := 0
	if v := os.Getenv("WING_POOL_SIZE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			poolSize = n
		}
	}
	wcfg := wing.Config{Workers: workers, HandlerPoolSize: poolSize}
	if os.Getenv("WING_ASYNC") == "1" {
		wcfg.DefaultFeather = wing.Feather{Dispatch: wing.Pool}
	}
	if os.Getenv("WING_PREFORK") == "1" {
		wcfg.Prefork = true
	}
	// Per-route dispatch from env: WING_POOL_ROUTES="GET /db,GET /queries,GET /fortunes,GET /updates"
	if routes := os.Getenv("WING_POOL_ROUTES"); routes != "" {
		wcfg.Feathers = make(map[string]wing.Feather)
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = wing.Feather{Dispatch: wing.Pool}
		}
	}
	if routes := os.Getenv("WING_SPAWN_ROUTES"); routes != "" {
		if wcfg.Feathers == nil {
			wcfg.Feathers = make(map[string]wing.Feather)
		}
		for _, r := range strings.Split(routes, ",") {
			wcfg.Feathers[strings.TrimSpace(r)] = wing.Feather{Dispatch: wing.Spawn}
		}
	}
	// Static responses: bypass handler entirely for maximum throughput.
	if os.Getenv("WING_STATIC") == "1" {
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
