package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-kruda/kruda"
)

// TestFlush_RunsOnceStoresError verifies the flush runs the underlying shutdown
// exactly once and returns the same stored error to every caller.
func TestFlush_RunsOnceStoresError(t *testing.T) {
	calls := 0
	sentinel := errors.New("boom")
	f := flusher(time.Second, func(ctx context.Context) error {
		calls++
		return sentinel
	})
	if err := f(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("first call err = %v, want sentinel", err)
	}
	if err := f(context.Background()); !errors.Is(err, sentinel) {
		t.Fatalf("second call err = %v, want same stored sentinel", err)
	}
	if calls != 1 {
		t.Fatalf("underlying flush ran %d times, want 1", calls)
	}
}

// TestFlush_RespectsTimeout verifies a hung flush is bounded by FlushTimeout.
func TestFlush_RespectsTimeout(t *testing.T) {
	f := flusher(50*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	start := time.Now()
	err := f(context.Background())
	if time.Since(start) > 500*time.Millisecond {
		t.Fatal("flush did not respect FlushTimeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want DeadlineExceeded", err)
	}
}

// TestFlush_WiredByEnable verifies Enable returns a real (non-nil-behavior) Flush
// and that calling it is safe and idempotent.
func TestFlush_WiredByEnable(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	app := kruda.New(kruda.NetHTTP())
	prov, err := Enable(app, Config{ServiceName: "t", SetGlobal: ptrBool(false)})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if prov.Flush == nil {
		t.Fatal("Flush must be set")
	}
	if err := prov.Flush(context.Background()); err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	if err := prov.Flush(context.Background()); err != nil {
		t.Fatalf("second Flush (idempotent): %v", err)
	}
}
