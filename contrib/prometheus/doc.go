// Package prometheus provides Prometheus metrics middleware for Kruda.
//
// # Usage
//
//	import "github.com/go-kruda/kruda/contrib/prometheus"
//
//	app := kruda.New()
//	app.Use(prometheus.New())
//	app.Get("/metrics", prometheus.Handler())
//
// # What it does
//
// The middleware records four standard HTTP metrics for every request:
//
//   - http_requests_total          — CounterVec  (method, status, path)
//   - http_request_duration_seconds — HistogramVec (method, status, path)
//   - http_requests_in_flight       — Gauge
//   - http_response_size_bytes      — HistogramVec (method, status, path)
//
// The path label uses the route pattern (e.g. "/users/:id") rather than
// the resolved path, which prevents the label cardinality explosion that
// happens when each user ID becomes its own time series. Override this
// with the GroupPathFunc config option.
//
// [Handler] returns a kruda.HandlerFunc that exposes the configured registry
// in Prometheus exposition format — register it on whichever route you
// scrape (typically /metrics).
//
// # Configuration
//
//   - Namespace:     metric namespace prefix (default empty)
//   - Subsystem:     metric subsystem prefix (default "http")
//   - Buckets:       histogram bucket boundaries (default prometheus.DefBuckets)
//   - Registry:      custom *prometheus.Registry (default: DefaultRegisterer)
//   - Skip:          per-request bypass function (e.g. skip /metrics itself)
//   - GroupPathFunc: route-label extractor (default c.Route())
//
// # See also
//
//   - https://prometheus.io/docs/concepts/metric_types/
package prometheus
