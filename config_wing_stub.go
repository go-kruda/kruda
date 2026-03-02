//go:build !linux && !darwin

package kruda

import (
	"log/slog"

	"github.com/go-kruda/kruda/transport"
)

func newWingTransport(cfg Config, logger *slog.Logger) transport.Transport {
	logger.Warn("wing not available on this platform, falling back to fasthttp")
	return newFastHTTPTransport(cfg, logger)
}
