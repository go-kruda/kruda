//go:build linux || darwin

package kruda

import (
	"log/slog"
	"os"
	"strconv"
	"syscall"

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
	bodyLimit := cfg.BodyLimit
	if bodyLimit > maxContentLength {
		bodyLimit = maxContentLength // Wing's parser cannot accept bodies larger than this
	}
	maxConns := cfg.MaxConns
	if maxConns < 0 { // unset → derive from fd ulimit
		var rl syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl); err == nil {
			maxConns = deriveMaxConns(rl.Cur, workers)
		} else {
			maxConns = 0 // can't read ulimit → unlimited (no surprise cap)
		}
	}
	if maxConns > 0 && maxConns < acceptCapLowFloor {
		logger.Warn("kruda/wing: derived connection cap is low; raise the fd ulimit or set WithMaxConns", "cap", maxConns)
	}
	if maxConns > 0 {
		logger.Info("kruda/wing: connection cap", "max", maxConns)
	}
	wcfg := WingConfig{
		Workers:          workers,
		HandlerPoolSize:  poolSize,
		ReadBufSize:      readBufSize,
		ReadTimeout:      cfg.ReadTimeout,
		WriteTimeout:     cfg.WriteTimeout,
		IdleTimeout:      cfg.IdleTimeout,
		BodyLimit:        bodyLimit,
		HeaderLimit:      cfg.HeaderLimit,
		TrustProxy:       cfg.TrustProxy,
		MaxConns:         maxConns,
		MaxConnsPerIP:    cfg.MaxConnsPerIP,
		AcceptRatePerSec: cfg.AcceptRatePerSec,
		AcceptRateBurst:  cfg.AcceptRateBurst,
		Logger:           logger,
	}
	return NewWingTransport(wcfg)
}
