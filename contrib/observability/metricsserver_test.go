package observability

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricsAuth_FixedLengthCompare verifies correct token passes; wrong scheme,
// wrong length, and wrong value all fail — without a length-dependent early return.
func TestMetricsAuth_FixedLengthCompare(t *testing.T) {
	const want = "s3cr3t-token"
	check := newBearerAuth(want)
	cases := []struct {
		hdr  string
		pass bool
	}{
		{"Bearer " + want, true},
		{"Bearer wrong", false},
		{"Bearer " + want + "x", false}, // wrong length
		{"Basic " + want, false},        // wrong scheme
		{"", false},
		{want, false}, // missing scheme
	}
	for _, tc := range cases {
		if got := check(tc.hdr); got != tc.pass {
			t.Fatalf("auth(%q) = %v, want %v", tc.hdr, got, tc.pass)
		}
	}
}

// TestMetrics_PublicScrapeServesData proves the dedicated-registry wiring serves
// real metric text end-to-end (not empty/500) when mounted on the app's port.
func TestMetrics_PublicScrapeServesData(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	app := kruda.New(kruda.NetHTTP())
	_, err := Enable(app, Config{ServiceName: "t", SetGlobal: ptrBool(false), MetricsPublic: true})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	app.Get("/ping", func(c *kruda.Ctx) error { return c.Status(200).Text("ok") })
	app.Compile()
	doGET(t, app, "/ping") // generate a metric data point
	rec := doGET(t, app, "/metrics")
	if rec.Code != 200 {
		t.Fatalf("/metrics = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "http_server_request_duration") &&
		!strings.Contains(rec.Body.String(), "target_info") {
		t.Fatalf("/metrics body has no recognizable metrics: %s", rec.Body.String())
	}
}

// TestMetricsHandler_BearerGate drives metricsHTTPHandler directly: no token => 401,
// correct bearer => 200.
func TestMetricsHandler_BearerGate(t *testing.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGauge(prometheus.GaugeOpts{Name: "up", Help: "up"}))
	const token = "tok-123"
	h := metricsHTTPHandler(reg, token)

	noAuth := httptest.NewRecorder()
	h.ServeHTTP(noAuth, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("no-bearer = %d, want 401", noAuth.Code)
	}
	if got := noAuth.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q, want Bearer", got)
	}

	ok := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("valid-bearer = %d, want 200; body=%s", ok.Code, ok.Body.String())
	}
	if !strings.Contains(ok.Body.String(), "up") {
		t.Fatalf("authorized scrape missing metric: %s", ok.Body.String())
	}
}

// TestStartMetricsListener_ServesThenCloses binds a separate listener, scrapes it
// (200 with metric text), then fires the registered OnShutdown and asserts the
// port no longer accepts — proving the listener is closed on app shutdown.
func TestStartMetricsListener_ServesThenCloses(t *testing.T) {
	// Grab a free port deterministically, then hand the address to the listener.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGauge(prometheus.GaugeOpts{Name: "up", Help: "up"}))

	app := kruda.New(kruda.NetHTTP())
	startMetricsListener(app, addr, "/metrics", reg, "")

	url := "http://" + addr + "/metrics"
	resp := getWithRetry(t, url)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scrape before shutdown = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	_ = app.Shutdown(context.Background()) // fires the OnShutdown hook that closes the listener
	if !portRefusedWithin(addr, 2*time.Second) {
		t.Fatalf("port %s still accepting after shutdown", addr)
	}
}

// getWithRetry GETs url, retrying briefly while the goroutine-started server binds.
func getWithRetry(t *testing.T, url string) *http.Response {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			return resp
		}
		if time.Now().After(deadline) {
			t.Fatalf("GET %s never succeeded: %v", url, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// portRefusedWithin reports whether addr stops accepting TCP connections within d.
func portRefusedWithin(addr string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return true
		}
		_ = conn.Close()
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
