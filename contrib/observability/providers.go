package observability

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// Providers holds the OTel providers Enable built. Flush is sync.Once-guarded,
// bounded by FlushTimeout, and returns the stored first-flush error to every caller.
type Providers struct {
	Tracer     trace.TracerProvider
	Meter      metric.MeterProvider
	Propagator propagation.TextMapPropagator
	Resource   *resource.Resource
	Flush      func(context.Context) error
}

// flusher builds the bounded, once-guarded Flush closure. shutdownFns run the
// underlying ForceFlush/Shutdown on the tracer + meter providers.
func flusher(timeout time.Duration, shutdownFns ...func(context.Context) error) func(context.Context) error {
	var once sync.Once
	var stored error
	return func(caller context.Context) error {
		once.Do(func() {
			if caller == nil {
				caller = context.Background()
			}
			ctx, cancel := context.WithTimeout(caller, timeout)
			defer cancel()
			for _, fn := range shutdownFns {
				if err := fn(ctx); err != nil && stored == nil {
					stored = err
				}
			}
		})
		return stored
	}
}
