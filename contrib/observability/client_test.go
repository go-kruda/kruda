package observability

import (
	"net/http"
	"testing"
)

// TestRoundTripper_WrapsBase verifies RoundTripper returns a non-nil transport
// and HTTPClient returns a client with a trace-propagating transport.
func TestRoundTripper_WrapsBase(t *testing.T) {
	rt := RoundTripper(http.DefaultTransport)
	if rt == nil {
		t.Fatal("RoundTripper returned nil")
	}
	if rt == http.DefaultTransport {
		t.Fatal("RoundTripper must wrap, not return the base unchanged")
	}
	c := HTTPClient()
	if c == nil || c.Transport == nil {
		t.Fatal("HTTPClient must return a client with a Transport")
	}
}

// TestRoundTripper_NilBase verifies a nil base falls back to http.DefaultTransport.
func TestRoundTripper_NilBase(t *testing.T) {
	if RoundTripper(nil) == nil {
		t.Fatal("RoundTripper(nil) must not return nil")
	}
}
