package observability

import (
	"testing"

	"github.com/go-kruda/kruda"
)

// TestHealth_LivezAlwaysOK verifies the probes return 200 with no DI container.
func TestHealth_LivezAlwaysOK(t *testing.T) {
	app := kruda.New(kruda.NetHTTP())
	r := Config{}.resolve()
	mountHealthRoutes(app, r)
	app.Compile()

	if code := doGETStatus(t, app, "/livez"); code != 200 {
		t.Fatalf("/livez = %d, want 200", code)
	}
	if code := doGETStatus(t, app, "/readyz"); code != 200 {
		t.Fatalf("/readyz with no checkers = %d, want 200", code)
	}
	if code := doGETStatus(t, app, "/health"); code != 200 {
		t.Fatalf("/health alias = %d, want 200", code)
	}
}

// TestHealth_MountedByEnable verifies Enable mounts the probes end-to-end.
func TestHealth_MountedByEnable(t *testing.T) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	app := kruda.New(kruda.NetHTTP())
	if _, err := Enable(app, Config{ServiceName: "t", SetGlobal: ptrBool(false)}); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	app.Compile()
	if code := doGETStatus(t, app, "/livez"); code != 200 {
		t.Fatalf("/livez via Enable = %d, want 200", code)
	}
	if code := doGETStatus(t, app, "/readyz"); code != 200 {
		t.Fatalf("/readyz via Enable = %d, want 200", code)
	}
}
