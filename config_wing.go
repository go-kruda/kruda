//go:build linux || darwin

package kruda

import (
	"log/slog"
	"os"
	"strconv"

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
	if cfg.HeaderLimit > 0 && readBufSize > 0 && readBufSize < cfg.HeaderLimit {
		panic("kruda: KRUDA_READ_BUF_SIZE is smaller than HeaderLimit; raise the read buffer or lower HeaderLimit")
	}
	wcfg := WingConfig{
		Workers:         workers,
		HandlerPoolSize: poolSize,
		ReadBufSize:     readBufSize,
		ReadTimeout:     cfg.ReadTimeout,
		WriteTimeout:    cfg.WriteTimeout,
		IdleTimeout:     cfg.IdleTimeout,
		BodyLimit:       cfg.BodyLimit,
		HeaderLimit:     cfg.HeaderLimit,
		TrustProxy:      cfg.TrustProxy,
	}
	return NewWingTransport(wcfg)
}
