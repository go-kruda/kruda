//go:build windows

package kruda

import (
	"log/slog"

	"github.com/go-kruda/kruda/transport"
)

func newFastHTTPTransport(cfg Config, logger *slog.Logger) transport.Transport {
	logger.Warn("fasthttp not available on Windows, falling back to nethttp")
	return transport.NewNetHTTP(transport.NetHTTPConfig{
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
		MaxBodySize:  cfg.BodyLimit,
		TrustProxy:   cfg.TrustProxy,
	})
}
