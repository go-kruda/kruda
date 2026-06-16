package observability

import (
	"time"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "github.com/go-kruda/kruda/contrib/observability"

// sentinelRoute collapses unmatched (404/405) requests so the raw path can't
// explode metric cardinality.
const sentinelRoute = "__unmatched__"

// reqStateKey is the locals key under which the span middleware stores per-request
// timing/in-flight state for the metric hook.
const reqStateKey = "__obs_req_state"

type reqState struct {
	startNanos int64
	matched    bool // true when the span middleware ran (route matched)
}

// metrics holds the RED instruments. duration is wired in Task 5; inflight is
// used by the span middleware.
type metrics struct {
	duration metric.Float64Histogram
	inflight metric.Int64UpDownCounter
}

func nowNanos() int64 { return time.Now().UnixNano() }

func newMetrics(prov *Providers) (*metrics, error) {
	mt := prov.Meter.Meter(meterName)
	dur, err := mt.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithDescription("Duration of inbound HTTP server requests."))
	if err != nil {
		return nil, err
	}
	inflight, err := mt.Int64UpDownCounter("http.server.active_requests",
		metric.WithUnit("{request}"),
		metric.WithDescription("Number of in-flight inbound HTTP server requests."))
	if err != nil {
		return nil, err
	}
	return &metrics{duration: dur, inflight: inflight}, nil
}

// routeLabel returns the route pattern for matched requests and the sentinel for
// unmatched ones. The span middleware sets reqState.matched only when it ran (i.e.
// the route matched); on a 404/405 it never runs, so c.Route() would return the
// raw attacker-controlled path — collapse it to sentinelRoute to bound cardinality.
func routeLabel(c *kruda.Ctx) string {
	if st, ok := kruda.Need[*reqState](c, reqStateKey); ok && st.matched {
		return c.Route()
	}
	return sentinelRoute
}

func metricAttrs(c *kruda.Ctx) attribute.Set {
	return attribute.NewSet(
		attribute.String("http.request.method", c.Method()),
		attribute.Int("http.response.status_code", c.StatusCode()),
		attribute.String("http.route", routeLabel(c)),
		attribute.String("url.scheme", schemeOf(c)),
	)
}

// onResponse returns the RED-metric hook. It skips the configured meta paths.
func (m *metrics) onResponse(r resolved) kruda.HookFunc {
	return func(c *kruda.Ctx) error {
		if isMetaPath(r, c.Path()) || m.duration == nil {
			return nil
		}
		var elapsed float64
		if st, ok := kruda.Need[*reqState](c, reqStateKey); ok && st.startNanos > 0 {
			elapsed = float64(nowNanos()-st.startNanos) / 1e9
		}
		m.duration.Record(c.Context(), elapsed, metric.WithAttributeSet(metricAttrs(c)))
		return nil
	}
}

func isMetaPath(r resolved, path string) bool {
	switch path {
	case r.metricsPath, r.livenessPath, r.readinessPath, r.healthPath:
		return true
	}
	return false
}
