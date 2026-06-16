package observability

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-kruda/kruda"
)

// TestEnable_ReturnsProviders verifies Enable returns non-nil Providers with a Flush.
func TestEnable_ReturnsProviders(t *testing.T) {
	app := kruda.New()
	prov, err := Enable(app, Config{ServiceName: "test-svc", SetGlobal: ptrBool(false)})
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if prov == nil || prov.Flush == nil {
		t.Fatal("Enable must return non-nil Providers with a Flush")
	}
	t.Cleanup(func() { _ = prov.Flush(nil) })
}

// TestEnable_DoubleEnableErrors verifies the meta-side guard.
func TestEnable_DoubleEnableErrors(t *testing.T) {
	app := kruda.New()
	if _, err := Enable(app, Config{SetGlobal: ptrBool(false)}); err != nil {
		t.Fatalf("first Enable: %v", err)
	}
	if _, err := Enable(app, Config{SetGlobal: ptrBool(false)}); err == nil {
		t.Fatal("second Enable on same app must return an error")
	}
}

// TestEnable_ConcurrentSameApp verifies exactly one concurrent Enable wins.
func TestEnable_ConcurrentSameApp(t *testing.T) {
	app := kruda.New()
	var wg sync.WaitGroup
	var okCount int32
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := Enable(app, Config{SetGlobal: ptrBool(false)}); err == nil {
				atomic.AddInt32(&okCount, 1)
			}
		}()
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("exactly one concurrent Enable should succeed, got %d", okCount)
	}
}

func ptrBool(b bool) *bool { return &b }
