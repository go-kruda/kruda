package prometheus

import (
	"github.com/go-kruda/kruda"
	promclient "github.com/prometheus/client_golang/prometheus"
)

// Config holds the configuration for the Prometheus metrics middleware.
type Config struct {
	// Namespace is the Prometheus metric namespace prefix (default: "").
	Namespace string

	// Subsystem is the Prometheus metric subsystem prefix (default: "http").
	Subsystem string

	// Buckets defines the histogram bucket boundaries for request duration.
	// Default: prometheus.DefBuckets.
	Buckets []float64

	// Registry is the Prometheus registry to use for metric registration.
	// Default: a new registry that also includes the default process/go collectors
	// is NOT used; metrics are registered with prometheus.DefaultRegisterer.
	// Provide a custom *promclient.Registry to isolate metrics (useful for testing).
	Registry *promclient.Registry

	// Skip is an optional function that returns true for requests that should
	// bypass metrics collection (e.g. health checks, metrics endpoint itself).
	Skip func(*kruda.Ctx) bool

	// GroupPathFunc extracts the path label from a request context.
	// Default: c.Route(), which returns the route pattern (e.g. "/users/:id")
	// instead of the resolved path (e.g. "/users/42"), preventing label
	// cardinality explosion.
	GroupPathFunc func(*kruda.Ctx) string
}

// defaults returns a Config with default values applied for any zero-value fields.
func (cfg Config) defaults() Config {
	if cfg.Subsystem == "" {
		cfg.Subsystem = "http"
	}
	if cfg.Buckets == nil {
		cfg.Buckets = promclient.DefBuckets
	}
	if cfg.GroupPathFunc == nil {
		cfg.GroupPathFunc = func(c *kruda.Ctx) string {
			return c.Route()
		}
	}
	return cfg
}
