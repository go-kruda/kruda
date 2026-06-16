package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	otelglobal "go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type sdkBundle struct {
	tp         *sdktrace.TracerProvider
	mp         *metric.MeterProvider
	res        *resource.Resource
	propagator propagation.TextMapPropagator
	promReg    *prometheus.Registry // dedicated registry the /metrics handler serves; nil if metrics off
}

func (s *sdkBundle) shutdown(ctx context.Context) error {
	var first error
	if err := s.tp.Shutdown(ctx); err != nil {
		first = err
	}
	if err := s.mp.Shutdown(ctx); err != nil && first == nil {
		first = err
	}
	return first
}

func buildSDK(ctx context.Context, r resolved) (*sdkBundle, error) {
	res, err := buildResource(ctx, r.serviceName)
	if err != nil {
		return nil, err
	}

	spanExp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}
	tpOpts := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(spanExp),
		sdktrace.WithResource(res),
	}
	if r.sampleRatio > 0 {
		tpOpts = append(tpOpts, sdktrace.WithSampler(
			sdktrace.ParentBased(sdktrace.TraceIDRatioBased(r.sampleRatio))))
	}
	tp := sdktrace.NewTracerProvider(tpOpts...)

	// Meter readers: a Prometheus exporter reader into a dedicated registry (which
	// the /metrics handler serves) makes the turnkey scrape work out of the box
	// without colliding on the global DefaultRegisterer when several apps Enable in
	// the same process; an env-driven autoexport reader is added only for an
	// explicit OTLP/console push.
	//
	// tp is already constructed, so any error past this point must shut it down to
	// avoid leaking its batch-span-processor goroutine (the module documents
	// retry-after-failure).
	mpOpts := []metric.Option{metric.WithResource(res)}
	var promReg *prometheus.Registry
	if r.metricsOn {
		promReg = prometheus.NewRegistry()
		promExp, perr := otelprom.New(otelprom.WithRegisterer(promReg))
		if perr != nil {
			_ = tp.Shutdown(context.Background())
			return nil, perr
		}
		mpOpts = append(mpOpts, metric.WithReader(promExp))
	}
	if exp := strings.ToLower(os.Getenv("OTEL_METRICS_EXPORTER")); exp != "" && exp != "prometheus" && exp != "none" {
		reader, rerr := autoexport.NewMetricReader(ctx)
		if rerr != nil {
			_ = tp.Shutdown(context.Background())
			return nil, rerr
		}
		mpOpts = append(mpOpts, metric.WithReader(reader))
	}
	mp := metric.NewMeterProvider(mpOpts...)

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	)

	if r.setGlobal {
		otelglobal.SetTracerProvider(tp)
		otelglobal.SetMeterProvider(mp)
		otelglobal.SetTextMapPropagator(prop)
	}

	warnIfNoEndpoint()

	return &sdkBundle{tp: tp, mp: mp, res: res, propagator: prop, promReg: promReg}, nil
}

// buildResource builds the OTel resource. The service.name chain is explicit
// serviceName => OTEL_SERVICE_NAME (resource.WithFromEnv) => unknown_service.
func buildResource(ctx context.Context, serviceName string) (*resource.Resource, error) {
	opts := []resource.Option{
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	}
	if serviceName != "" {
		opts = append(opts, resource.WithAttributes(semconv.ServiceName(serviceName)))
	}
	return resource.New(ctx, opts...)
}

// warnIfNoEndpoint logs one slog.Warn when default OTLP is selected with no endpoint.
func warnIfNoEndpoint() {
	exp := os.Getenv("OTEL_TRACES_EXPORTER")
	if exp != "" && exp != "otlp" {
		return
	}
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" {
		return
	}
	warnOnce()
}

var noEndpointOnce sync.Once

func warnOnce() {
	noEndpointOnce.Do(func() {
		slog.Warn("observability: default OTLP exporter selected but no OTEL_EXPORTER_OTLP_ENDPOINT set; traces/metrics will be dropped. Set the endpoint or OTEL_TRACES_EXPORTER=console for local dev.")
	})
}
