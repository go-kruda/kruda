package prometheus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
	promclient "github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// newTestRegistry creates an isolated Prometheus registry for each test,
// preventing "duplicate metric" panics across parallel test runs.
func newTestRegistry() *promclient.Registry {
	return promclient.NewRegistry()
}

// newTestApp creates a Kruda app with the Prometheus middleware and a set of
// test routes. It returns the app, the httptest server, and the registry used.
func newTestApp(cfg Config) (*kruda.App, *httptest.Server, *promclient.Registry) {
	if cfg.Registry == nil {
		cfg.Registry = newTestRegistry()
	}
	reg := cfg.Registry

	app := kruda.New(kruda.NetHTTP())
	app.Use(New(cfg))

	app.Get("/hello", func(c *kruda.Ctx) error {
		return c.Text("hello")
	})
	app.Get("/users/:id", func(c *kruda.Ctx) error {
		return c.JSON(map[string]string{"id": c.Param("id")})
	})
	app.Post("/data", func(c *kruda.Ctx) error {
		return c.Status(201).Text("created")
	})
	app.Get("/error", func(c *kruda.Ctx) error {
		return c.Status(500).Text("internal error")
	})
	app.Get("/metrics", Handler(reg))

	app.Compile()
	srv := httptest.NewServer(app)
	return app, srv, reg
}

// gatherMetric collects all metric families from a registry and returns the
// named one, or nil if not found.
func gatherMetric(reg *promclient.Registry, name string) *dto.MetricFamily {
	families, err := reg.Gather()
	if err != nil {
		return nil
	}
	for _, f := range families {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

// findCounter returns the counter value for the given label set, or -1 if not found.
func findCounter(family *dto.MetricFamily, labels map[string]string) float64 {
	if family == nil {
		return -1
	}
	for _, m := range family.GetMetric() {
		if matchLabels(m, labels) {
			return m.GetCounter().GetValue()
		}
	}
	return -1
}

// findHistogramCount returns the sample count for the given label set, or -1.
func findHistogramCount(family *dto.MetricFamily, labels map[string]string) uint64 {
	if family == nil {
		return 0
	}
	for _, m := range family.GetMetric() {
		if matchLabels(m, labels) {
			return m.GetHistogram().GetSampleCount()
		}
	}
	return 0
}

// findGaugeValue returns the gauge value, or -1 if the family is nil.
func findGaugeValue(family *dto.MetricFamily) float64 {
	if family == nil || len(family.GetMetric()) == 0 {
		return -1
	}
	return family.GetMetric()[0].GetGauge().GetValue()
}

// matchLabels checks that all expected labels match the metric's label pairs.
func matchLabels(m *dto.Metric, expected map[string]string) bool {
	pairs := m.GetLabel()
	if len(pairs) != len(expected) {
		return false
	}
	for _, lp := range pairs {
		v, ok := expected[lp.GetName()]
		if !ok || v != lp.GetValue() {
			return false
		}
	}
	return true
}

func TestNew_RequestsTotal(t *testing.T) {
	_, srv, reg := newTestApp(Config{})
	defer srv.Close()

	// Make 3 requests.
	for i := 0; i < 3; i++ {
		resp, err := http.Get(srv.URL + "/hello")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	family := gatherMetric(reg, "http_requests_total")
	if family == nil {
		t.Fatal("metric http_requests_total not found")
	}

	count := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/hello",
	})
	if count != 3 {
		t.Errorf("http_requests_total = %v, want 3", count)
	}
}

func TestNew_RequestDuration(t *testing.T) {
	_, srv, reg := newTestApp(Config{})
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	family := gatherMetric(reg, "http_request_duration_seconds")
	if family == nil {
		t.Fatal("metric http_request_duration_seconds not found")
	}

	count := findHistogramCount(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/hello",
	})
	if count != 1 {
		t.Errorf("histogram sample_count = %d, want 1", count)
	}
}

func TestNew_InFlight(t *testing.T) {
	reg := newTestRegistry()
	app := kruda.New(kruda.NetHTTP())

	// Channel to hold handler until we check in-flight.
	hold := make(chan struct{})

	app.Use(New(Config{Registry: reg}))
	app.Get("/slow", func(c *kruda.Ctx) error {
		<-hold
		return c.Text("done")
	})
	app.Compile()
	srv := httptest.NewServer(app)
	defer srv.Close()

	// Start a request that will block.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := http.Get(srv.URL + "/slow")
		if err == nil {
			resp.Body.Close()
		}
	}()

	// Give the request time to reach the handler.
	time.Sleep(50 * time.Millisecond)

	family := gatherMetric(reg, "http_requests_in_flight")
	val := findGaugeValue(family)
	if val != 1 {
		t.Errorf("in_flight = %v, want 1", val)
	}

	// Release the handler.
	close(hold)
	wg.Wait()

	// After completion, in_flight should be back to 0.
	family = gatherMetric(reg, "http_requests_in_flight")
	val = findGaugeValue(family)
	if val != 0 {
		t.Errorf("in_flight after completion = %v, want 0", val)
	}
}

func TestNew_StatusLabels(t *testing.T) {
	_, srv, reg := newTestApp(Config{})
	defer srv.Close()

	// 200 request.
	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// 500 request.
	resp, err = http.Get(srv.URL + "/error")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// 201 request.
	resp, err = http.Post(srv.URL+"/data", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	family := gatherMetric(reg, "http_requests_total")
	if family == nil {
		t.Fatal("metric http_requests_total not found")
	}

	tests := []struct {
		labels map[string]string
		want   float64
	}{
		{map[string]string{"method": "GET", "status": "200", "path": "/hello"}, 1},
		{map[string]string{"method": "GET", "status": "500", "path": "/error"}, 1},
		{map[string]string{"method": "POST", "status": "201", "path": "/data"}, 1},
	}
	for _, tt := range tests {
		got := findCounter(family, tt.labels)
		if got != tt.want {
			t.Errorf("counter for %v = %v, want %v", tt.labels, got, tt.want)
		}
	}
}

func TestNew_PathGrouping(t *testing.T) {
	_, srv, reg := newTestApp(Config{})
	defer srv.Close()

	// Hit parameterized route with different IDs.
	for _, id := range []string{"1", "2", "42", "999"} {
		resp, err := http.Get(srv.URL + "/users/" + id)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
	}

	family := gatherMetric(reg, "http_requests_total")
	if family == nil {
		t.Fatal("metric http_requests_total not found")
	}

	// All 4 requests should be grouped under the route pattern "/users/:id".
	count := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/users/:id",
	})
	if count != 4 {
		t.Errorf("grouped counter = %v, want 4", count)
	}

	// There should be NO entry for resolved paths like "/users/1".
	for _, id := range []string{"1", "2", "42", "999"} {
		c := findCounter(family, map[string]string{
			"method": "GET",
			"status": "200",
			"path":   "/users/" + id,
		})
		if c != -1 {
			t.Errorf("found resolved path /users/%s with count %v; expected grouping", id, c)
		}
	}
}

func TestNew_Skip(t *testing.T) {
	reg := newTestRegistry()
	_, srv, _ := newTestApp(Config{
		Registry: reg,
		Skip: func(c *kruda.Ctx) bool {
			return c.Path() == "/hello"
		},
	})
	defer srv.Close()

	// /hello should be skipped.
	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("/hello status = %d, want 200", resp.StatusCode)
	}

	// /users/:id should be tracked.
	resp, err = http.Get(srv.URL + "/users/1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	family := gatherMetric(reg, "http_requests_total")
	if family == nil {
		t.Fatal("metric http_requests_total not found")
	}

	// Skipped path should have no counter entry.
	skippedCount := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/hello",
	})
	if skippedCount != -1 {
		t.Errorf("/hello counter = %v, want -1 (not found)", skippedCount)
	}

	// Tracked path should have a counter.
	trackedCount := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/users/:id",
	})
	if trackedCount != 1 {
		t.Errorf("/users/:id counter = %v, want 1", trackedCount)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	reg := newTestRegistry()
	customBuckets := []float64{0.01, 0.05, 0.1, 0.5, 1.0}

	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{
		Namespace: "myapp",
		Subsystem: "api",
		Buckets:   customBuckets,
		Registry:  reg,
	}))
	app.Get("/ping", func(c *kruda.Ctx) error {
		return c.Text("pong")
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Verify metrics use custom namespace/subsystem.
	family := gatherMetric(reg, "myapp_api_requests_total")
	if family == nil {
		t.Fatal("metric myapp_api_requests_total not found")
	}

	count := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/ping",
	})
	if count != 1 {
		t.Errorf("myapp_api_requests_total = %v, want 1", count)
	}

	// Duration histogram should also use custom prefix.
	durationFamily := gatherMetric(reg, "myapp_api_request_duration_seconds")
	if durationFamily == nil {
		t.Fatal("metric myapp_api_request_duration_seconds not found")
	}

	// Verify custom buckets were applied by checking bucket count.
	for _, m := range durationFamily.GetMetric() {
		buckets := m.GetHistogram().GetBucket()
		// Custom buckets + the implicit +Inf bucket = len(customBuckets)
		if len(buckets) != len(customBuckets) {
			t.Errorf("bucket count = %d, want %d", len(buckets), len(customBuckets))
		}
		break
	}
}

func TestHandler_ServesMetrics(t *testing.T) {
	_, srv, _ := newTestApp(Config{})
	defer srv.Close()

	// Make a request first so there are metrics to serve.
	resp, err := http.Get(srv.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Now fetch /metrics.
	resp, err = http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("/metrics status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	bodyStr := string(body)

	// The /metrics response should contain Prometheus text format.
	if !strings.Contains(bodyStr, "http_requests_total") {
		t.Error("/metrics does not contain http_requests_total")
	}
	if !strings.Contains(bodyStr, "http_request_duration_seconds") {
		t.Error("/metrics does not contain http_request_duration_seconds")
	}
	if !strings.Contains(bodyStr, "http_requests_in_flight") {
		t.Error("/metrics does not contain http_requests_in_flight")
	}
	if !strings.Contains(bodyStr, "http_response_size_bytes") {
		t.Error("/metrics does not contain http_response_size_bytes")
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	reg := newTestRegistry()

	app := kruda.New(kruda.NetHTTP())
	app.Use(New(Config{Registry: reg}))
	app.Get("/test", func(c *kruda.Ctx) error {
		return c.Text("ok")
	})
	app.Compile()

	srv := httptest.NewServer(app)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/test")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	// Default subsystem is "http", no namespace.
	family := gatherMetric(reg, "http_requests_total")
	if family == nil {
		t.Fatal("default config: metric http_requests_total not found")
	}

	count := findCounter(family, map[string]string{
		"method": "GET",
		"status": "200",
		"path":   "/test",
	})
	if count != 1 {
		t.Errorf("default config counter = %v, want 1", count)
	}

	// Verify duration histogram exists.
	dFamily := gatherMetric(reg, "http_request_duration_seconds")
	if dFamily == nil {
		t.Fatal("default config: metric http_request_duration_seconds not found")
	}

	// Verify response size histogram exists.
	sFamily := gatherMetric(reg, "http_response_size_bytes")
	if sFamily == nil {
		t.Fatal("default config: metric http_response_size_bytes not found")
	}

	// Verify in-flight gauge exists.
	gFamily := gatherMetric(reg, "http_requests_in_flight")
	if gFamily == nil {
		t.Fatal("default config: metric http_requests_in_flight not found")
	}
}
