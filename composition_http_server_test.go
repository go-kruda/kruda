//go:build linux || darwin

package kruda

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	krudajson "github.com/go-kruda/kruda/json"
)

func TestCompositionBenchmarkServer(t *testing.T) {
	if os.Getenv("KRUDA_COMPOSITION_SERVER") != "1" {
		t.Skip("composition HTTP server is opt-in")
	}
	fields := os.Getenv("KRUDA_COMPOSITION_FIELDS")
	if fields != "5" && fields != "10" && fields != "30" {
		t.Fatalf("invalid KRUDA_COMPOSITION_FIELDS %q", fields)
	}
	workersText := os.Getenv("KRUDA_COMPOSITION_WORKERS")
	if workersText == "" {
		workersText = "4"
	}
	if workersText != "4" && workersText != "8" {
		t.Fatalf("invalid KRUDA_COMPOSITION_WORKERS %q", workersText)
	}
	workers, _ := strconv.Atoi(workersText)
	addr := os.Getenv("KRUDA_COMPOSITION_ADDR")
	if addr == "" {
		t.Fatal("KRUDA_COMPOSITION_ADDR is required")
	}
	var fixture compositionFixture
	for _, candidate := range compositionFixtures() {
		if candidate.name == "fields"+fields {
			fixture = candidate
			break
		}
	}
	if fixture.register == nil {
		t.Fatal("composition fixture not found")
	}
	wing := NewWingTransport(WingConfig{Workers: workers, ReadBufSize: 8192})
	app := New(Wing(), WithTransport(wing))
	compositionPipeline(app)
	if got := fixture.eligible(app.config.Validator); got != compositionTypedValidationExpected {
		t.Fatalf("typed validation eligibility=%t, want %t", got, compositionTypedValidationExpected)
	}
	fixture.register(app)
	app.Compile()
	if app.transport != wing || wing.config.Workers != workers || wing.config.ReadBufSize != 8192 {
		t.Fatal("composition Compile changed the selected Wing transport or configuration")
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	errCh := make(chan error, 1)
	serveDone := false
	defer func() {
		if serveDone {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := app.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		select {
		case err := <-errCh:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("composition Serve did not finish after shutdown")
		}
	}()
	// Serve leaves signal handling to this fixture; Listen installs a second owner.
	go func() { errCh <- app.Serve(listener) }()
	startup := time.NewTimer(5 * time.Second)
	defer startup.Stop()
	select {
	case <-wing.ready:
	case err := <-errCh:
		serveDone = true
		t.Fatalf("composition server exited before readiness: %v", err)
	case <-signals:
		return
	case <-startup.C:
		t.Fatal("composition Wing startup timed out")
	}
	fmt.Printf("composition fields=%s typed=%t go=%s engine=%s addr=%s workers=%d read_buffer=%d gomaxprocs=%d\n",
		fields, compositionTypedValidationExpected, runtime.Version(), krudajson.ActiveEngine(),
		addr, wing.config.Workers, wing.config.ReadBufSize, runtime.GOMAXPROCS(0))
	lifetime := time.NewTimer(45 * time.Second)
	defer lifetime.Stop()
	select {
	case err := <-errCh:
		serveDone = true
		t.Fatalf("composition server exited before shutdown signal: %v", err)
	case <-signals:
	case <-lifetime.C:
		t.Error("composition server exceeded its 45-second lifetime")
	}
}
