package observability

import (
	"time"

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
