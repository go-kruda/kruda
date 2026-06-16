package observability

import (
	"context"
	"testing"

	"github.com/go-kruda/kruda"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// TestMetrics_DurationRecordedWithRoute verifies a duration data point with the
// route label is recorded for a served request, and 404 uses the sentinel route.
func TestMetrics_DurationRecordedWithRoute(t *testing.T) {
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	prov := &Providers{Meter: mp}

	m, err := newMetrics(prov)
	if err != nil {
		t.Fatalf("newMetrics: %v", err)
	}
	r := Config{}.resolve()
	app := kruda.New(kruda.NetHTTP())
	app.Use(spanMiddleware(&Providers{Tracer: noopTracerProvider()}, m, r))
	app.OnResponse(m.onResponse(r))
	app.Get("/ping", func(c *kruda.Ctx) error { return c.Status(200).Text("ok") })
	app.Compile()

	doGET(t, app, "/ping")
	doGET(t, app, "/does-not-exist") // 404: span middleware never runs -> sentinel route

	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	routes := collectRouteLabels(t, &rm, "http.server.request.duration")
	if !routes["/ping"] {
		t.Fatalf("missing /ping duration series; got %v", routes)
	}
	if !routes[sentinelRoute] {
		t.Fatalf("404 must use sentinel route %q; got %v", sentinelRoute, routes)
	}
}

// TestMetrics_MetaRouteSkipped verifies meta paths are detected for the skip.
func TestMetrics_MetaRouteSkipped(t *testing.T) {
	r := Config{}.resolve()
	if !isMetaPath(r, r.metricsPath) {
		t.Fatalf("%q should be a meta path", r.metricsPath)
	}
	if !isMetaPath(r, r.livenessPath) || !isMetaPath(r, r.readinessPath) || !isMetaPath(r, r.healthPath) {
		t.Fatal("livez/readyz/health must be meta paths")
	}
	if isMetaPath(r, "/ping") {
		t.Fatal("/ping must not be a meta path")
	}
}
