package quic

import (
	"testing"

	"github.com/go-kruda/kruda/transport"
)

// Compile-time check: QUICTransport implements transport.Transport.
var _ transport.Transport = (*QUICTransport)(nil)

func TestNewWithInvalidCertPaths(t *testing.T) {
	_, err := New(Config{
		TLSCertFile: "/nonexistent/cert.pem",
		TLSKeyFile:  "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for invalid cert paths, got nil")
	}
}

func TestShutdownNilServer(t *testing.T) {
	// QUICTransport with no server started — Shutdown should return nil.
	qt := &QUICTransport{}
	if err := qt.Shutdown(nil); err != nil {
		t.Fatalf("expected nil error from Shutdown on nil server, got: %v", err)
	}
}
