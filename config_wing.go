//go:build linux || darwin

package kruda

import (
	"log/slog"

	"github.com/go-kruda/kruda/transport"
	// "github.com/go-kruda/kruda/transport/wing"
)

func newWingTransport(cfg Config, logger *slog.Logger) transport.Transport {
	logger.Warn("wing temporarily disabled for benchmarking")
	return newFastHTTPTransport(cfg, logger)
}
