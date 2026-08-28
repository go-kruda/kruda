package transport

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNetHTTPTransportServeTLSUsesConfiguredTLS(t *testing.T) {
	certFile, keyFile := writeTestTLSKeyPair(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	tr := NewNetHTTP(NetHTTPConfig{TLSCertFile: certFile, TLSKeyFile: keyFile})
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.ServeTLS(ln, HandlerFunc(func(w ResponseWriter, _ Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secure"))
		}))
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{InsecureSkipVerify: true}) //nolint:gosec -- test certificate
	if err != nil {
		shutdownNetHTTPTestServer(t, tr, serveErr)
		t.Fatalf("TLS handshake through ServeTLS failed: %v", err)
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "secure" {
		t.Fatalf("response = (%d, %q), want (200, %q)", resp.StatusCode, body, "secure")
	}

	shutdownNetHTTPTestServer(t, tr, serveErr)
}

func TestNetHTTPTransportServePreservesWrappedListener(t *testing.T) {
	certFile, keyFile := writeTestTLSKeyPair(t)
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn := tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{cert}})

	tr := NewNetHTTP(NetHTTPConfig{TLSCertFile: certFile, TLSKeyFile: keyFile})
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- tr.Serve(tlsLn, HandlerFunc(func(w ResponseWriter, _ Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("secure"))
		}))
	}()

	conn, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{InsecureSkipVerify: true}) //nolint:gosec -- test certificate
	if err != nil {
		shutdownNetHTTPTestServer(t, tr, serveErr)
		t.Fatalf("TLS handshake through wrapped listener failed: %v", err)
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		shutdownNetHTTPTestServer(t, tr, serveErr)
		t.Fatalf("read response through wrapped listener: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "secure" {
		t.Fatalf("response = (%d, %q), want (200, %q)", resp.StatusCode, body, "secure")
	}

	shutdownNetHTTPTestServer(t, tr, serveErr)
}

func writeTestTLSKeyPair(t *testing.T) (string, string) {
	t.Helper()
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cert := tlsServer.TLS.Certificates[0]
	tlsServer.Close()

	keyDER, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile
}

func shutdownNetHTTPTestServer(t *testing.T, tr *NetHTTPTransport, serveErr <-chan error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := tr.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not stop")
	}
}
