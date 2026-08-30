package kruda

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kruda/kruda/transport"
)

var errTLSProbeDone = errors.New("TLS probe done")

type tlsProbeTransport struct {
	serveCalled    bool
	serveTLSCalled bool
}

func (*tlsProbeTransport) ListenAndServe(string, transport.Handler) error {
	return errors.New("ListenAndServe should not be called by App.Listen")
}

func (tr *tlsProbeTransport) Serve(ln net.Listener, _ transport.Handler) error {
	tr.serveCalled = true
	_ = ln.Close()
	return errTLSProbeDone
}

func (tr *tlsProbeTransport) ServeTLS(ln net.Listener, _ transport.Handler) error {
	tr.serveTLSCalled = true
	_ = ln.Close()
	return errTLSProbeDone
}

func (*tlsProbeTransport) Shutdown(context.Context) error { return nil }

func TestListenUsesTLSServerForConfiguredTLS(t *testing.T) {
	tr := &tlsProbeTransport{}
	app := New(WithTransport(tr), WithTLS("cert.pem", "key.pem"))

	err := app.Listen("127.0.0.1:0")
	if !errors.Is(err, errTLSProbeDone) {
		t.Fatalf("Listen error = %v, want %v", err, errTLSProbeDone)
	}
	if !tr.serveTLSCalled {
		t.Fatal("transport ServeTLS was not called")
	}
	if tr.serveCalled {
		t.Fatal("transport Serve was called with configured TLS")
	}
}

func TestListenRejectsConfiguredTLSWithoutTLSServer(t *testing.T) {
	tr := &startupProbeTransport{}
	app := New(WithTransport(tr), WithTLS("cert.pem", "key.pem"))

	err := app.Listen("127.0.0.1:0")
	if err == nil || !strings.Contains(err.Error(), "does not support configured TLS") {
		t.Fatalf("Listen error = %v, want TLS capability error", err)
	}
	if tr.serveCalled {
		t.Fatal("transport Serve was called with configured TLS")
	}
}

func TestServeAllowsTLSConfiguredTransportWithoutTLSServer(t *testing.T) {
	tr := &startupProbeTransport{}
	app := New(WithTransport(tr), WithTLS("cert.pem", "key.pem"))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	err = app.Serve(ln)
	if !errors.Is(err, errStartupTransportDone) {
		t.Fatalf("Serve error = %v, want %v", err, errStartupTransportDone)
	}
	if !tr.serveCalled {
		t.Fatal("transport Serve was not called for caller-owned listener")
	}
}

func TestListenReleasesListenerWhenTLSSetupFails(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close()

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := New(NetHTTP(), WithTLS(certFile, keyFile))
	if err := app.Listen(addr); err == nil {
		t.Fatal("Listen succeeded with an invalid TLS key pair")
	}

	rebound, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listener remained bound after TLS setup failed: %v", err)
	}
	rebound.Close()
}

func TestListenRejectsIncompleteTLSConfiguration(t *testing.T) {
	tests := []struct {
		name string
		cert string
		key  string
	}{
		{name: "certificate only", cert: "cert.pem"},
		{name: "key only", key: "key.pem"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &tlsProbeTransport{}
			app := New(WithTransport(tr), WithTLS(tt.cert, tt.key))

			err := app.Listen("127.0.0.1:0")
			if err == nil || !strings.Contains(err.Error(), "TLS certificate and key") {
				t.Fatalf("Listen error = %v, want incomplete TLS configuration error", err)
			}
			if tr.serveCalled || tr.serveTLSCalled {
				t.Fatal("transport started with incomplete TLS configuration")
			}
		})
	}
}
