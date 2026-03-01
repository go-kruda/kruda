//go:build !windows

package kruda

import (
	"log/slog"

	"github.com/go-kruda/kruda/transport"
)

func newFastHTTPTransport(cfg Config, logger *slog.Logger) transport.Transport {
	fasthttpCfg := transport.FastHTTPConfig{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		MaxBodySize:  cfg.BodyLimit,
		TrustProxy:   cfg.TrustProxy,
	}
	logger.Debug("transport selected", "name", "fasthttp")
	return transport.NewFastHTTP(fasthttpCfg)
}
