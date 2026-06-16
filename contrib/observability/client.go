package observability

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// RoundTripper wraps base so outbound HTTP requests continue the active trace,
// using the globally-installed TracerProvider and propagator (set by Enable).
// A nil base falls back to http.DefaultTransport.
func RoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return otelhttp.NewTransport(base)
}

// HTTPClient returns an *http.Client whose Transport propagates trace context.
func HTTPClient() *http.Client {
	return &http.Client{Transport: RoundTripper(http.DefaultTransport)}
}
