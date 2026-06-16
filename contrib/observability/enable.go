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
		Flush:      func(context.Context) error { return nil }, // bounded flush wired in Task 8
	}

	// 1. Meta routes first — registered before the span middleware so they carry
	//    no span (self-tracing skip). The metric hook (installed below) skips them too.
	mountHealthRoutes(app, r)

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
