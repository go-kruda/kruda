package observability

import (
	"context"
	"errors"
	"sync"

	"github.com/go-kruda/kruda"
)

// enabledApps tracks apps that already called Enable (meta-side double-Enable guard).
var (
	enabledMu   sync.Mutex
	enabledApps = map[*kruda.App]struct{}{}
)

// ErrAlreadyEnabled is returned when Enable is called twice on the same app.
var ErrAlreadyEnabled = errors.New("observability: Enable already called on this app")

// Enable wires observability into app. It MUST be called before any route is
// registered (the span middleware bakes into route chains at registration time).
func Enable(app *kruda.App, cfg Config) (*Providers, error) {
	enabledMu.Lock()
	if _, dup := enabledApps[app]; dup {
		enabledMu.Unlock()
		return nil, ErrAlreadyEnabled
	}
	enabledApps[app] = struct{}{}
	enabledMu.Unlock()

	r := cfg.resolve()

	ctx := context.Background()
	sdk, err := buildSDK(ctx, r)
	if err != nil {
		enabledMu.Lock()
		delete(enabledApps, app) // allow retry after a build failure
		enabledMu.Unlock()
		return nil, err
	}

	prov := &Providers{
		Tracer:     sdk.tp,
		Meter:      sdk.mp,
		Propagator: sdk.propagator,
		Resource:   sdk.res,
	}

	// Bounded, once-guarded flush: ForceFlush then Shutdown each provider. Shutdown
	// leaves the providers unusable, which is correct for a flush-on-shutdown — the
	// same closure backs both prov.Flush and the OnShutdown hook so Listen() users
	// drain telemetry without an explicit call.
	flush := flusher(r.flushTimeout, sdk.tp.ForceFlush, sdk.tp.Shutdown, sdk.mp.ForceFlush, sdk.mp.Shutdown)
	prov.Flush = flush
	app.OnShutdown(func() { _ = flush(context.Background()) })

	// 1. Meta routes first — registered before the span middleware so they carry
	//    no span (self-tracing skip). The metric hook (installed below) skips them too.
	mountHealthRoutes(app, r)

	// 1b. /metrics — a meta route mounted before the span middleware (no
	//     self-tracing). Default is an internal loopback listener; opt into a
	//     custom bind or a public mount on the app's own port.
	if r.metricsOn && sdk.promReg != nil {
		switch {
		case cfg.MetricsBind != "":
			startMetricsListener(app, cfg.MetricsBind, r.metricsPath, sdk.promReg, cfg.MetricsAuth)
		case cfg.MetricsPublic:
			app.Get(r.metricsPath, metricsKrudaHandler(sdk.promReg, cfg.MetricsAuth))
		default:
			startMetricsListener(app, "127.0.0.1:0", r.metricsPath, sdk.promReg, cfg.MetricsAuth)
		}
	}

	// 2. RED metric hook (always-fires OnResponse).
	var m *metrics
	if r.metricsOn {
		m, err = newMetrics(prov)
		if err != nil {
			return nil, err
		}
		app.OnResponse(m.onResponse(r))
	}

	// 3. Span middleware — only instruments routes registered AFTER this point,
	//    which is why Enable must run before user routes.
	if r.tracesOn || m != nil {
		app.Use(spanMiddleware(prov, m, r))
	}

	// 4. Log enricher (trace_id/span_id into c.Log()).
	app.SetLogEnricher(logEnricher)
	return prov, nil
}
