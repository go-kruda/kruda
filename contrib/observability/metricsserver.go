package observability

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/go-kruda/kruda"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newBearerAuth returns a checker that constant-time-compares the presented
// Authorization header against want. Both sides are SHA-256 digested (always
// 32 bytes) so there is no length-dependent early return (no length oracle).
func newBearerAuth(want string) func(authHeader string) bool {
	wantSum := sha256.Sum256([]byte(want))
	return func(h string) bool {
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) {
			// Constant-time compare against a zero digest so the no-prefix path
			// does not branch faster than the real compare (timing uniformity).
			var zero [32]byte
			subtle.ConstantTimeCompare(wantSum[:], zero[:])
			return false
		}
		gotSum := sha256.Sum256([]byte(h[len(prefix):]))
		return subtle.ConstantTimeCompare(wantSum[:], gotSum[:]) == 1
	}
}

// metricsHTTPHandler serves THIS app's dedicated Prometheus registry (not the
// global default registry — two apps in one process would otherwise collide),
// optionally bearer-guarded.
func metricsHTTPHandler(reg *prometheus.Registry, token string) http.Handler {
	base := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	if token == "" {
		return base
	}
	check := newBearerAuth(token)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !check(req.Header.Get("Authorization")) {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		base.ServeHTTP(w, req)
	})
}

// metricsKrudaHandler adapts the scrape handler to a kruda.HandlerFunc for the
// MetricsPublic case (mount on the app's own port/router). It drives the
// http.Handler with an httptest recorder and copies the result through the Ctx,
// so it works on any transport (core exposes no http.Handler-mount helper).
func metricsKrudaHandler(reg *prometheus.Registry, token string) kruda.HandlerFunc {
	h := metricsHTTPHandler(reg, token)
	return func(c *kruda.Ctx) error {
		req, err := http.NewRequest(http.MethodGet, c.Path(), nil)
		if err != nil {
			return c.Status(500).Text("metrics error")
		}
		if auth := c.Header("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		for k, vs := range rec.Header() {
			if len(vs) > 0 {
				c.SetHeader(k, vs[0])
			}
		}
		return c.Status(rec.Code).SendBytes(rec.Body.Bytes())
	}
}

// startMetricsListener serves /metrics on a separate listener that is closed on
// app shutdown (so it cannot delay termination).
func startMetricsListener(app *kruda.App, addr, path string, reg *prometheus.Registry, token string) {
	mux := http.NewServeMux()
	mux.Handle(path, metricsHTTPHandler(reg, token))
	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Warn("observability: metrics listener failed", "addr", addr, "err", err)
		return
	}
	go func() { _ = srv.Serve(ln) }()
	app.OnShutdown(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})
}
