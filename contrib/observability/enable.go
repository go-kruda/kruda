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
	return prov, nil
}
