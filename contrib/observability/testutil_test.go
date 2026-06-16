package observability

import (
	"net/http/httptest"
	"testing"

	"github.com/go-kruda/kruda"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// doGET drives a GET through the app's net/http pipeline and returns the recorder.
func doGET(t *testing.T, app *kruda.App, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)
	return rec
}

// doGETStatus returns just the status code.
func doGETStatus(t *testing.T, app *kruda.App, path string) int {
	return doGET(t, app, path).Code
}

// noopTracerProvider returns a no-op TracerProvider for metric-only tests.
func noopTracerProvider() trace.TracerProvider { return noop.NewTracerProvider() }

// collectRouteLabels returns the set of http.route attribute values seen across
// the data points of the named float64 histogram in rm.
func collectRouteLabels(t *testing.T, rm *metricdata.ResourceMetrics, name string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, sm := range rm.ScopeMetrics {
		for _, mt := range sm.Metrics {
			if mt.Name != name {
				continue
			}
			hist, ok := mt.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is not a float64 histogram, got %T", name, mt.Data)
			}
			for _, dp := range hist.DataPoints {
				if v, ok := dp.Attributes.Value(attribute.Key("http.route")); ok {
					out[v.AsString()] = true
				}
			}
		}
	}
	return out
}
