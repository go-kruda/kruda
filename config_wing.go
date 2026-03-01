//go:build linux || darwin || windows

package kruda

import (
	"log/slog"

	"github.com/go-kruda/kruda/transport"
	"github.com/go-kruda/kruda/transport/wing"
)

func newWingTransport(cfg Config, logger *slog.Logger) transport.Transport {
	wingCfg := wing.Config{
		Workers:     0, // auto = NumCPU
		RingSize:    4096,
		ReadBufSize: 8192,
	}
	logger.Debug("transport selected", "name", "wing")
	return wing.New(wingCfg)
}
