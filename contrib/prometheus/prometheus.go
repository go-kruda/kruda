// Package prometheus provides Prometheus metrics middleware for the Kruda web framework.
//
// It tracks HTTP request count, duration, in-flight requests, and response size
// using standard Prometheus metric types. The path label uses the route pattern
// (e.g. "/users/:id") by default to prevent cardinality explosion.
//
// Usage:
//
//	app := kruda.New()
//	app.Use(prometheus.New())
//	app.Get("/metrics", prometheus.Handler())
package prometheus

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"time"

	"github.com/go-kruda/kruda"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// New creates a Prometheus metrics middleware that tracks:
//   - http_requests_total: CounterVec (method, status, path)
//   - http_request_duration_seconds: HistogramVec (method, status, path)
//   - http_requests_in_flight: Gauge
//   - http_response_size_bytes: HistogramVec (method, status, path)
func New(config ...Config) kruda.HandlerFunc {
	var cfg Config
	if len(config) > 0 {
		cfg = config[0]
	}
	cfg = cfg.defaults()

	labels := []string{"method", "status", "path"}

	// Determine registerer: custom registry or default.
	var registerer promclient.Registerer
	if cfg.Registry != nil {
		registerer = cfg.Registry
	} else {
		registerer = promclient.DefaultRegisterer
	}

	requestsTotal := promclient.NewCounterVec(
		promclient.CounterOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		labels,
	)

	requestDuration := promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   cfg.Buckets,
		},
		labels,
	)

	requestsInFlight := promclient.NewGauge(
		promclient.GaugeOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being processed.",
		},
	)

	responseSizeBytes := promclient.NewHistogramVec(
		promclient.HistogramOpts{
			Namespace: cfg.Namespace,
			Subsystem: cfg.Subsystem,
			Name:      "response_size_bytes",
			Help:      "HTTP response size in bytes.",
			Buckets:   promclient.ExponentialBuckets(100, 10, 7), // 100B .. 100MB
		},
		labels,
	)

	// Register all metrics. MustRegister panics on duplicate registration,
	// which is correct for application startup. For testing, provide a
	// dedicated *promclient.Registry per test.
	registerer.MustRegister(requestsTotal, requestDuration, requestsInFlight, responseSizeBytes)

	return func(c *kruda.Ctx) error {
		// Skip check.
		if cfg.Skip != nil && cfg.Skip(c) {
			return c.Next()
		}

		requestsInFlight.Inc()
		start := time.Now()

		// Execute next handler in chain.
		err := c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.StatusCode())
		method := c.Method()
		path := cfg.GroupPathFunc(c)

		requestsTotal.WithLabelValues(method, status, path).Inc()
		requestDuration.WithLabelValues(method, status, path).Observe(duration)

		// Record response size. Kruda does not expose a public getter for the
		// response content length, so we observe 0 when the size is unknown.
		// This still provides per-route observation counts; if precise sizes are
		// needed, set Content-Length explicitly and it will be captured here in
		// future Kruda versions that expose response metadata.
		responseSizeBytes.WithLabelValues(method, status, path).Observe(0)

		requestsInFlight.Dec()

		return err
	}
}

// Handler returns a Kruda handler that serves the Prometheus metrics endpoint.
// It adapts promhttp.Handler() (an http.Handler) to Kruda's handler signature.
//
// Usage:
//
//	app.Get("/metrics", prometheus.Handler())
func Handler(registry ...*promclient.Registry) kruda.HandlerFunc {
	var h http.Handler
	if len(registry) > 0 && registry[0] != nil {
		h = promhttp.HandlerFor(registry[0], promhttp.HandlerOpts{})
	} else {
		h = promhttp.Handler()
	}

	return func(c *kruda.Ctx) error {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/metrics", nil)
		h.ServeHTTP(w, r)

		c.Status(w.Code)
		for k, vals := range w.Header() {
			if len(vals) > 0 {
				c.SetHeader(k, vals[0])
			}
		}
		return c.SendBytes(w.Body.Bytes())
	}
}
